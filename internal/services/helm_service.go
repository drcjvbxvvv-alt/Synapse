package services

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/shaia/Synapse/internal/models"

	"gorm.io/gorm"
	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/chart/loader"
	"helm.sh/helm/v4/pkg/release"
	v1release "helm.sh/helm/v4/pkg/release/v1"
	"helm.sh/helm/v4/pkg/repo/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/yaml"
)

// helmHTTPClient is a package-level singleton that reuses TCP/TLS connections
// across all Helm repository index fetches (P2-13: avoid per-request TLS handshake).
var helmHTTPClient = &http.Client{
	Timeout: 30 * time.Second,
	Transport: &http.Transport{
		MaxIdleConnsPerHost: 5,
	},
}

// restClientGetter 實作 genericclioptions.RESTClientGetter 介面，供 Helm action.Configuration 使用
type restClientGetter struct {
	restConfig *rest.Config
	namespace  string
}

// ToRESTConfig 回傳 rest.Config
func (r *restClientGetter) ToRESTConfig() (*rest.Config, error) {
	return r.restConfig, nil
}

// ToDiscoveryClient 建立 Discovery 客戶端（帶記憶體快取）
func (r *restClientGetter) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	dc, err := discovery.NewDiscoveryClientForConfig(r.restConfig)
	if err != nil {
		return nil, err
	}
	return memory.NewMemCacheClient(dc), nil
}

// ToRESTMapper 建立 REST Mapper
func (r *restClientGetter) ToRESTMapper() (meta.RESTMapper, error) {
	dc, err := r.ToDiscoveryClient()
	if err != nil {
		return nil, err
	}
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(dc)
	return mapper, nil
}

// ToRawKubeConfigLoader 從 rest.Config 建構 clientcmd.ClientConfig
func (r *restClientGetter) ToRawKubeConfigLoader() clientcmd.ClientConfig {
	kubeConfig := clientcmdapi.NewConfig()
	clusterName := "cluster"
	authName := "auth"
	contextName := "context"

	kubeConfig.Clusters[clusterName] = &clientcmdapi.Cluster{
		Server:                   r.restConfig.Host,
		CertificateAuthorityData: r.restConfig.CAData,
		InsecureSkipTLSVerify:    r.restConfig.Insecure,
	}
	kubeConfig.AuthInfos[authName] = &clientcmdapi.AuthInfo{
		Token:                 r.restConfig.BearerToken,
		ClientCertificateData: r.restConfig.CertData,
		ClientKeyData:         r.restConfig.KeyData,
	}
	kubeConfig.Contexts[contextName] = &clientcmdapi.Context{
		Cluster:   clusterName,
		AuthInfo:  authName,
		Namespace: r.namespace,
	}
	kubeConfig.CurrentContext = contextName

	return clientcmd.NewDefaultClientConfig(*kubeConfig, &clientcmd.ConfigOverrides{})
}

// InstallRequest Helm Release 安裝請求
type InstallRequest struct {
	Namespace   string `json:"namespace"`
	ReleaseName string `json:"release_name"`
	RepoName    string `json:"repo_name"`
	ChartName   string `json:"chart_name"`
	Version     string `json:"version"`
	Values      string `json:"values"`
}

// UpgradeRequest Helm Release 升級請求
type UpgradeRequest struct {
	Values  string `json:"values"`
	Version string `json:"version"`
}

// ChartInfo Chart 資訊
type ChartInfo struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	RepoName    string `json:"repo_name"`
}

// HelmService Helm 操作服務
type HelmService struct {
	db *gorm.DB
}

// NewHelmService 建立 HelmService 例項
func NewHelmService(db *gorm.DB) *HelmService {
	return &HelmService{db: db}
}

// newActionConfig 建立 Helm action.Configuration（核心函式）
func (s *HelmService) newActionConfig(cluster *models.Cluster, namespace string) (*action.Configuration, error) {
	k8sClient, err := NewK8sClientForCluster(cluster)
	if err != nil {
		return nil, fmt.Errorf("建立 K8s 客戶端失敗: %w", err)
	}
	restConfig := k8sClient.GetRestConfig()

	getter := &restClientGetter{
		restConfig: restConfig,
		namespace:  namespace,
	}

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(getter, namespace, "secret"); err != nil {
		return nil, fmt.Errorf("初始化 Helm action 配置失敗: %w", err)
	}

	return actionConfig, nil
}

// ListReleases 列出 Helm Releases
func (s *HelmService) ListReleases(ctx context.Context, cluster *models.Cluster, namespace string) ([]*v1release.Release, error) {
	cfg, err := s.newActionConfig(cluster, namespace)
	if err != nil {
		return nil, err
	}

	listAction := action.NewList(cfg)
	listAction.AllNamespaces = namespace == ""
	listAction.All = true

	rels, err := listAction.Run()
	if err != nil {
		return nil, err
	}
	return toReleases(rels)
}

// GetRelease 取得單一 Helm Release 詳情
func (s *HelmService) GetRelease(ctx context.Context, cluster *models.Cluster, namespace, name string) (*v1release.Release, error) {
	cfg, err := s.newActionConfig(cluster, namespace)
	if err != nil {
		return nil, err
	}

	statusAction := action.NewStatus(cfg)
	rel, err := statusAction.Run(name)
	if err != nil {
		return nil, err
	}
	return toRelease(rel)
}

// GetHistory 取得 Helm Release 歷史版本
func (s *HelmService) GetHistory(ctx context.Context, cluster *models.Cluster, namespace, name string) ([]*v1release.Release, error) {
	cfg, err := s.newActionConfig(cluster, namespace)
	if err != nil {
		return nil, err
	}

	historyAction := action.NewHistory(cfg)
	historyAction.Max = 256
	rels, err := historyAction.Run(name)
	if err != nil {
		return nil, err
	}
	return toReleases(rels)
}

// GetValues 取得 Helm Release 的 values
func (s *HelmService) GetValues(ctx context.Context, cluster *models.Cluster, namespace, name string, allValues bool) (map[string]interface{}, error) {
	cfg, err := s.newActionConfig(cluster, namespace)
	if err != nil {
		return nil, err
	}

	getValuesAction := action.NewGetValues(cfg)
	getValuesAction.AllValues = allValues
	return getValuesAction.Run(name)
}

// InstallRelease 安裝 Helm Release
func (s *HelmService) InstallRelease(ctx context.Context, cluster *models.Cluster, req InstallRequest) (*v1release.Release, error) {
	cfg, err := s.newActionConfig(cluster, req.Namespace)
	if err != nil {
		return nil, err
	}

	// 下載 chart
	chartPath, err := s.downloadChart(ctx, req.RepoName, req.ChartName, req.Version)
	if err != nil {
		return nil, fmt.Errorf("下載 Chart 失敗: %w", err)
	}
	defer func() {
		os.Remove(chartPath)
		os.RemoveAll(filepath.Dir(chartPath))
	}()

	chrt, err := loader.Load(chartPath)
	if err != nil {
		return nil, fmt.Errorf("載入 Chart 失敗: %w", err)
	}

	// 解析 values
	vals, err := parseValues(req.Values)
	if err != nil {
		return nil, fmt.Errorf("解析 Values 失敗: %w", err)
	}

	installAction := action.NewInstall(cfg)
	installAction.ReleaseName = req.ReleaseName
	installAction.Namespace = req.Namespace
	installAction.CreateNamespace = true

	rel, err := installAction.Run(chrt, vals)
	if err != nil {
		return nil, err
	}
	return toRelease(rel)
}

// UpgradeRelease 升級 Helm Release
func (s *HelmService) UpgradeRelease(ctx context.Context, cluster *models.Cluster, namespace, name string, req UpgradeRequest) (*v1release.Release, error) {
	cfg, err := s.newActionConfig(cluster, namespace)
	if err != nil {
		return nil, err
	}

	// 取得目前 release 以找出 chart 資訊
	statusAction := action.NewStatus(cfg)
	currentRel, err := statusAction.Run(name)
	if err != nil {
		return nil, fmt.Errorf("取得目前 Release 失敗: %w", err)
	}
	currentRelease, err := toRelease(currentRel)
	if err != nil {
		return nil, fmt.Errorf("轉換 Release 型別失敗: %w", err)
	}

	// 從 chart metadata 中取得 chart 名稱
	chartName := ""
	if currentRelease.Chart != nil && currentRelease.Chart.Metadata != nil {
		chartName = currentRelease.Chart.Metadata.Name
	}

	// 嘗試找 repo（從現有 repos 中搜尋 chart 名稱）
	repoName := ""
	var repos []models.HelmRepository
	s.db.WithContext(ctx).Find(&repos)
	for _, r := range repos {
		if s.chartExistsInRepo(&r, chartName) {
			repoName = r.Name
			break
		}
	}

	if repoName == "" {
		return nil, fmt.Errorf("找不到對應的 Chart Repository，無法升級")
	}

	version := req.Version
	if version == "" && currentRelease.Chart != nil && currentRelease.Chart.Metadata != nil {
		version = currentRelease.Chart.Metadata.Version
	}

	chartPath, err := s.downloadChart(ctx, repoName, chartName, version)
	if err != nil {
		return nil, fmt.Errorf("下載 Chart 失敗: %w", err)
	}
	defer func() {
		os.Remove(chartPath)
		os.RemoveAll(filepath.Dir(chartPath))
	}()

	chrt, err := loader.Load(chartPath)
	if err != nil {
		return nil, fmt.Errorf("載入 Chart 失敗: %w", err)
	}

	// 解析 values
	vals, err := parseValues(req.Values)
	if err != nil {
		return nil, fmt.Errorf("解析 Values 失敗: %w", err)
	}

	upgradeAction := action.NewUpgrade(cfg)
	upgradeAction.Namespace = namespace

	rel, err := upgradeAction.Run(name, chrt, vals)
	if err != nil {
		return nil, err
	}
	return toRelease(rel)
}

// RollbackRelease 回滾 Helm Release
func (s *HelmService) RollbackRelease(ctx context.Context, cluster *models.Cluster, namespace, name string, version int) error {
	cfg, err := s.newActionConfig(cluster, namespace)
	if err != nil {
		return err
	}

	rollbackAction := action.NewRollback(cfg)
	rollbackAction.Version = version
	return rollbackAction.Run(name)
}

// UninstallRelease 解除安裝 Helm Release
func (s *HelmService) UninstallRelease(ctx context.Context, cluster *models.Cluster, namespace, name string) error {
	cfg, err := s.newActionConfig(cluster, namespace)
	if err != nil {
		return err
	}

	uninstallAction := action.NewUninstall(cfg)
	_, err = uninstallAction.Run(name)
	return err
}

// ListRepos 列出所有 Helm Repository
func (s *HelmService) ListRepos(ctx context.Context) ([]models.HelmRepository, error) {
	var repos []models.HelmRepository
	if err := s.db.WithContext(ctx).Find(&repos).Error; err != nil {
		return nil, err
	}
	return repos, nil
}

// AddRepo 新增 Helm Repository
func (s *HelmService) AddRepo(ctx context.Context, name, url, username, password string) (*models.HelmRepository, error) {
	helmRepo := &models.HelmRepository{
		Name:     name,
		URL:      url,
		Username: username,
		Password: password,
	}
	if err := s.db.WithContext(ctx).Create(helmRepo).Error; err != nil {
		return nil, fmt.Errorf("新增 Repository 失敗: %w", err)
	}
	return helmRepo, nil
}

// RemoveRepo 刪除 Helm Repository
func (s *HelmService) RemoveRepo(ctx context.Context, name string) error {
	result := s.db.WithContext(ctx).Where("name = ?", name).Delete(&models.HelmRepository{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("找不到 Repository: %s", name)
	}
	return nil
}

// SearchCharts 搜尋 Chart（P2-10：並行 HTTP 請求，最多 5 個 Repo 同時拉取 index.yaml）
func (s *HelmService) SearchCharts(ctx context.Context, keyword string) ([]ChartInfo, error) {
	var repos []models.HelmRepository
	if err := s.db.WithContext(ctx).Find(&repos).Error; err != nil {
		return nil, err
	}
	if len(repos) == 0 {
		return nil, nil
	}

	type repoResult struct {
		charts []ChartInfo
	}

	results := make([]repoResult, len(repos))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 5) // 最多 5 個並行 HTTP 請求

	for i, r := range repos {
		wg.Add(1)
		i, r := i, r // 捕獲迴圈變數
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			indexFile, err := s.fetchRepoIndex(&r)
			if err != nil {
				// 單一 repo 失敗不影響其他 repo
				return
			}

			var collected []ChartInfo
			lowerKeyword := strings.ToLower(keyword)
			for chartName, versions := range indexFile.Entries {
				if keyword == "" || strings.Contains(strings.ToLower(chartName), lowerKeyword) {
					if len(versions) > 0 {
						collected = append(collected, ChartInfo{
							Name:        chartName,
							Version:     versions[0].Version,
							Description: versions[0].Description,
							RepoName:    r.Name,
						})
					}
				}
			}
			results[i].charts = collected
		}()
	}
	wg.Wait()

	var charts []ChartInfo
	for _, r := range results {
		charts = append(charts, r.charts...)
	}
	return charts, nil
}

// fetchRepoIndex 取得 Repository 的 index.yaml
func (s *HelmService) fetchRepoIndex(r *models.HelmRepository) (*repo.IndexFile, error) {
	indexURL := strings.TrimRight(r.URL, "/") + "/index.yaml"

	req, err := http.NewRequest("GET", indexURL, nil)
	if err != nil {
		return nil, err
	}

	if r.Username != "" && r.Password != "" {
		req.SetBasicAuth(r.Username, r.Password)
	}

	resp, err := helmHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("無法取得 repo index: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("取得 repo index 失敗，狀態碼: %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// 寫入暫存檔後用 LoadIndexFile 載入
	tmpFile, err := os.CreateTemp("", "helm-index-*.yaml")
	if err != nil {
		return nil, err
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return nil, err
	}
	tmpFile.Close()

	return repo.LoadIndexFile(tmpPath)
}

// chartExistsInRepo 檢查 chart 是否存在於 repo 中
func (s *HelmService) chartExistsInRepo(r *models.HelmRepository, chartName string) bool {
	if chartName == "" {
		return false
	}
	indexFile, err := s.fetchRepoIndex(r)
	if err != nil {
		return false
	}
	_, ok := indexFile.Entries[chartName]
	return ok
}

// downloadChart 從 repo 下載 chart 到暫存檔
func (s *HelmService) downloadChart(ctx context.Context, repoName, chartName, version string) (string, error) {
	var helmRepo models.HelmRepository
	if err := s.db.WithContext(ctx).Where("name = ?", repoName).First(&helmRepo).Error; err != nil {
		return "", fmt.Errorf("找不到 Repository '%s': %w", repoName, err)
	}

	// 取得 repo index 以找到 chart 下載 URL
	indexFile, err := s.fetchRepoIndex(&helmRepo)
	if err != nil {
		return "", fmt.Errorf("取得 repo index 失敗: %w", err)
	}

	versions, ok := indexFile.Entries[chartName]
	if !ok || len(versions) == 0 {
		return "", fmt.Errorf("在 repo '%s' 中找不到 chart '%s'", repoName, chartName)
	}

	// 找出指定版本（或最新版本）
	var chartVersion *repo.ChartVersion
	if version != "" {
		for _, v := range versions {
			if v.Version == version {
				chartVersion = v
				break
			}
		}
		if chartVersion == nil {
			return "", fmt.Errorf("找不到 chart '%s' 版本 '%s'", chartName, version)
		}
	} else {
		chartVersion = versions[0] // 最新版本（index 已排序）
	}

	if len(chartVersion.URLs) == 0 {
		return "", fmt.Errorf("chart '%s' 沒有下載 URL", chartName)
	}

	// 建構完整的下載 URL
	downloadURL := chartVersion.URLs[0]
	if !strings.HasPrefix(downloadURL, "http://") && !strings.HasPrefix(downloadURL, "https://") {
		downloadURL = strings.TrimRight(helmRepo.URL, "/") + "/" + downloadURL
	}

	// 下載 chart 壓縮包（使用 singleton helmHTTPClient，P2-13）
	req, err := http.NewRequest("GET", downloadURL, nil)
	if err != nil {
		return "", err
	}
	if helmRepo.Username != "" && helmRepo.Password != "" {
		req.SetBasicAuth(helmRepo.Username, helmRepo.Password)
	}

	resp, err := helmHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("下載 chart 失敗: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("下載 chart 失敗，狀態碼: %d", resp.StatusCode)
	}

	// 寫入暫存目錄
	tmpDir, err := os.MkdirTemp("", "helm-chart-*")
	if err != nil {
		return "", err
	}

	tmpFile := filepath.Join(tmpDir, chartName+"-"+chartVersion.Version+".tgz")
	f, err := os.Create(tmpFile)
	if err != nil {
		os.RemoveAll(tmpDir)
		return "", err
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		os.RemoveAll(tmpDir)
		return "", err
	}

	return tmpFile, nil
}

// toRelease 將 Helm v4 Releaser 介面轉換為具體的 *v1release.Release。
func toRelease(rel release.Releaser) (*v1release.Release, error) {
	r, ok := rel.(*v1release.Release)
	if !ok {
		return nil, fmt.Errorf("unexpected release type: %T", rel)
	}
	return r, nil
}

// toReleases 將 Releaser slice 轉換為 []*v1release.Release。
func toReleases(rels []release.Releaser) ([]*v1release.Release, error) {
	result := make([]*v1release.Release, 0, len(rels))
	for _, rel := range rels {
		r, err := toRelease(rel)
		if err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, nil
}

// parseValues 解析 YAML 格式的 values 字串
func parseValues(valuesStr string) (map[string]interface{}, error) {
	if valuesStr == "" {
		return map[string]interface{}{}, nil
	}

	vals := map[string]interface{}{}
	if err := yaml.Unmarshal([]byte(valuesStr), &vals); err != nil {
		return nil, err
	}
	return vals, nil
}
