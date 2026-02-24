package services

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/clay-wangzhi/KubePolaris/pkg/logger"
)

//go:embed dashboards/*.json
var dashboardFS embed.FS

// GrafanaService Grafana API 服务
type GrafanaService struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// DataSourceRequest Grafana 数据源请求
type DataSourceRequest struct {
	Name      string                 `json:"name"`
	UID       string                 `json:"uid,omitempty"`
	Type      string                 `json:"type"`
	URL       string                 `json:"url"`
	Access    string                 `json:"access"`
	IsDefault bool                   `json:"isDefault"`
	JSONData  map[string]interface{} `json:"jsonData,omitempty"`
}

// GenerateDataSourceUID 根据集群名生成数据源 UID
func GenerateDataSourceUID(clusterName string) string {
	// 转为小写，替换特殊字符为连字符
	uid := strings.ToLower(clusterName)
	uid = strings.ReplaceAll(uid, " ", "-")
	uid = strings.ReplaceAll(uid, "_", "-")
	return fmt.Sprintf("prometheus-%s", uid)
}

// DataSourceResponse Grafana 数据源响应
type DataSourceResponse struct {
	ID        int    `json:"id"`
	UID       string `json:"uid"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	URL       string `json:"url"`
	IsDefault bool   `json:"isDefault"`
}

// NewGrafanaService 创建 Grafana 服务
func NewGrafanaService(baseURL, apiKey string) *GrafanaService {
	return &GrafanaService{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// IsEnabled 检查 Grafana 服务是否启用
func (s *GrafanaService) IsEnabled() bool {
	return s.baseURL != "" && s.apiKey != ""
}

// UpdateConfig 热更新 Grafana 连接配置
func (s *GrafanaService) UpdateConfig(baseURL, apiKey string) {
	s.baseURL = strings.TrimSuffix(baseURL, "/")
	s.apiKey = apiKey
}

// GetBaseURL 获取当前 Grafana 地址
func (s *GrafanaService) GetBaseURL() string {
	return s.baseURL
}

// SyncDataSource 同步数据源（创建或更新）
func (s *GrafanaService) SyncDataSource(clusterName, prometheusURL string) error {
	if !s.IsEnabled() {
		logger.Info("Grafana 服务未启用，跳过数据源同步")
		return nil
	}

	if prometheusURL == "" {
		logger.Info("Prometheus URL 为空，跳过数据源同步", "cluster", clusterName)
		return nil
	}

	dataSourceName := fmt.Sprintf("Prometheus-%s", clusterName)

	// 先检查数据源是否存在
	exists, err := s.dataSourceExists(dataSourceName)
	if err != nil {
		logger.Error("检查数据源是否存在失败", "error", err)
		// 继续尝试创建
	}

	if exists {
		// 更新现有数据源
		return s.updateDataSource(dataSourceName, clusterName, prometheusURL)
	}

	// 创建新数据源
	return s.createDataSource(dataSourceName, clusterName, prometheusURL)
}

// DeleteDataSource 删除数据源
func (s *GrafanaService) DeleteDataSource(clusterName string) error {
	if !s.IsEnabled() {
		logger.Info("Grafana 服务未启用，跳过数据源删除")
		return nil
	}

	dataSourceName := fmt.Sprintf("Prometheus-%s", clusterName)

	url := fmt.Sprintf("%s/api/datasources/name/%s", s.baseURL, dataSourceName)
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("创建删除请求失败: %w", err)
	}

	s.setHeaders(req)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("删除数据源请求失败: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode == http.StatusNotFound {
		logger.Info("数据源不存在，无需删除", "name", dataSourceName)
		return nil
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("删除数据源失败: status=%d, body=%s", resp.StatusCode, string(body))
	}

	logger.Info("Grafana 数据源删除成功", "name", dataSourceName)
	return nil
}

// dataSourceExists 检查数据源是否存在
func (s *GrafanaService) dataSourceExists(name string) (bool, error) {
	url := fmt.Sprintf("%s/api/datasources/name/%s", s.baseURL, name)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false, err
	}

	s.setHeaders(req)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	return resp.StatusCode == http.StatusOK, nil
}

// createDataSource 创建数据源
func (s *GrafanaService) createDataSource(name, clusterName, prometheusURL string) error {
	dsReq := DataSourceRequest{
		Name:      name,
		UID:       GenerateDataSourceUID(clusterName),
		Type:      "prometheus",
		URL:       prometheusURL,
		Access:    "proxy",
		IsDefault: false,
		JSONData: map[string]interface{}{
			"httpMethod":   "POST",
			"timeInterval": "15s",
		},
	}

	body, err := json.Marshal(dsReq)
	if err != nil {
		return fmt.Errorf("序列化数据源请求失败: %w", err)
	}

	url := fmt.Sprintf("%s/api/datasources", s.baseURL)
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

	s.setHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("创建数据源请求失败: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("创建数据源失败: status=%d, body=%s", resp.StatusCode, string(respBody))
	}

	logger.Info("Grafana 数据源创建成功", "name", name, "url", prometheusURL)
	return nil
}

// updateDataSource 更新数据源
func (s *GrafanaService) updateDataSource(name, clusterName, prometheusURL string) error {
	// 先获取数据源 ID
	url := fmt.Sprintf("%s/api/datasources/name/%s", s.baseURL, name)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("创建获取请求失败: %w", err)
	}

	s.setHeaders(req)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("获取数据源失败: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("数据源不存在: %s", name)
	}

	var ds DataSourceResponse
	if err := json.NewDecoder(resp.Body).Decode(&ds); err != nil {
		return fmt.Errorf("解析数据源响应失败: %w", err)
	}

	// 更新数据源
	dsReq := DataSourceRequest{
		Name:      name,
		UID:       GenerateDataSourceUID(clusterName),
		Type:      "prometheus",
		URL:       prometheusURL,
		Access:    "proxy",
		IsDefault: ds.IsDefault,
		JSONData: map[string]interface{}{
			"httpMethod":   "POST",
			"timeInterval": "15s",
		},
	}

	body, err := json.Marshal(dsReq)
	if err != nil {
		return fmt.Errorf("序列化数据源请求失败: %w", err)
	}

	updateURL := fmt.Sprintf("%s/api/datasources/%d", s.baseURL, ds.ID)
	updateReq, err := http.NewRequest("PUT", updateURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("创建更新请求失败: %w", err)
	}

	s.setHeaders(updateReq)
	updateReq.Header.Set("Content-Type", "application/json")

	updateResp, err := s.httpClient.Do(updateReq)
	if err != nil {
		return fmt.Errorf("更新数据源请求失败: %w", err)
	}
	defer func() {
		_ = updateResp.Body.Close()
	}()

	if updateResp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(updateResp.Body)
		return fmt.Errorf("更新数据源失败: status=%d, body=%s", updateResp.StatusCode, string(respBody))
	}

	logger.Info("Grafana 数据源更新成功", "name", name, "url", prometheusURL)
	return nil
}

// setHeaders 设置请求头
func (s *GrafanaService) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.apiKey))
	req.Header.Set("Accept", "application/json")
}

// TestConnection 测试 Grafana 连接
func (s *GrafanaService) TestConnection() error {
	if !s.IsEnabled() {
		return fmt.Errorf("grafana 服务未配置")
	}

	url := fmt.Sprintf("%s/api/health", s.baseURL)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

	s.setHeaders(req)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("连接 Grafana 失败: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("grafana 健康检查失败: status=%d", resp.StatusCode)
	}

	return nil
}

// DashboardSyncStatus Dashboard 同步状态
type DashboardSyncStatus struct {
	FolderExists bool                    `json:"folder_exists"`
	Dashboards   []DashboardStatusItem   `json:"dashboards"`
	AllSynced    bool                    `json:"all_synced"`
}

// DashboardStatusItem 单个 Dashboard 的状态
type DashboardStatusItem struct {
	UID    string `json:"uid"`
	Title  string `json:"title"`
	Exists bool   `json:"exists"`
}

// EnsureDashboards 确保 KubePolaris 文件夹和 Dashboard 已导入到 Grafana
func (s *GrafanaService) EnsureDashboards() (*DashboardSyncStatus, error) {
	if !s.IsEnabled() {
		return nil, fmt.Errorf("grafana 服务未配置")
	}

	status := &DashboardSyncStatus{
		Dashboards: []DashboardStatusItem{},
	}

	// 1. 创建 KubePolaris 文件夹（幂等）
	folderExists, err := s.ensureFolder("kubepolaris-folder", "KubePolaris")
	if err != nil {
		return nil, fmt.Errorf("创建 KubePolaris 文件夹失败: %w", err)
	}
	status.FolderExists = folderExists

	// 2. 读取嵌入的 Dashboard JSON 文件并逐个导入
	entries, err := dashboardFS.ReadDir("dashboards")
	if err != nil {
		return nil, fmt.Errorf("读取嵌入的 Dashboard 文件失败: %w", err)
	}

	status.AllSynced = true
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		data, err := dashboardFS.ReadFile("dashboards/" + entry.Name())
		if err != nil {
			logger.Error("读取 Dashboard 文件失败", "file", entry.Name(), "error", err)
			status.AllSynced = false
			continue
		}

		// 解析 Dashboard JSON 获取 UID 和 Title
		var dashboardJSON map[string]interface{}
		if err := json.Unmarshal(data, &dashboardJSON); err != nil {
			logger.Error("解析 Dashboard JSON 失败", "file", entry.Name(), "error", err)
			status.AllSynced = false
			continue
		}

		uid, _ := dashboardJSON["uid"].(string)
		title, _ := dashboardJSON["title"].(string)

		// 导入 Dashboard
		if err := s.importDashboard(dashboardJSON, "kubepolaris-folder"); err != nil {
			logger.Error("导入 Dashboard 失败", "uid", uid, "title", title, "error", err)
			status.Dashboards = append(status.Dashboards, DashboardStatusItem{
				UID: uid, Title: title, Exists: false,
			})
			status.AllSynced = false
			continue
		}

		logger.Info("Dashboard 导入成功", "uid", uid, "title", title)
		status.Dashboards = append(status.Dashboards, DashboardStatusItem{
			UID: uid, Title: title, Exists: true,
		})
	}

	return status, nil
}

// GetDashboardSyncStatus 获取 Dashboard 同步状态（只检查不导入）
func (s *GrafanaService) GetDashboardSyncStatus() (*DashboardSyncStatus, error) {
	if !s.IsEnabled() {
		return nil, fmt.Errorf("grafana 服务未配置")
	}

	status := &DashboardSyncStatus{
		AllSynced:  true,
		Dashboards: []DashboardStatusItem{},
	}

	// 检查文件夹
	status.FolderExists = s.folderExists("kubepolaris-folder")

	// 检查每个 Dashboard
	entries, err := dashboardFS.ReadDir("dashboards")
	if err != nil {
		return nil, fmt.Errorf("读取嵌入的 Dashboard 文件失败: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		data, err := dashboardFS.ReadFile("dashboards/" + entry.Name())
		if err != nil {
			continue
		}

		var dashboardJSON map[string]interface{}
		if err := json.Unmarshal(data, &dashboardJSON); err != nil {
			continue
		}

		uid, _ := dashboardJSON["uid"].(string)
		title, _ := dashboardJSON["title"].(string)

		exists := s.dashboardExists(uid)
		status.Dashboards = append(status.Dashboards, DashboardStatusItem{
			UID: uid, Title: title, Exists: exists,
		})
		if !exists {
			status.AllSynced = false
		}
	}

	if !status.FolderExists {
		status.AllSynced = false
	}

	return status, nil
}

// ensureFolder 确保 Grafana 文件夹存在（幂等）
func (s *GrafanaService) ensureFolder(uid, title string) (bool, error) {
	if s.folderExists(uid) {
		return true, nil
	}

	reqBody, _ := json.Marshal(map[string]string{
		"uid":   uid,
		"title": title,
	})

	apiURL := fmt.Sprintf("%s/api/folders", s.baseURL)
	req, err := http.NewRequest("POST", apiURL, bytes.NewReader(reqBody))
	if err != nil {
		return false, err
	}
	s.setHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("创建文件夹请求失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// 200/412(已存在) 都算成功
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusPreconditionFailed {
		return true, nil
	}

	body, _ := io.ReadAll(resp.Body)
	return false, fmt.Errorf("创建文件夹失败: status=%d, body=%s", resp.StatusCode, string(body))
}

// folderExists 检查文件夹是否存在
func (s *GrafanaService) folderExists(uid string) bool {
	apiURL := fmt.Sprintf("%s/api/folders/%s", s.baseURL, uid)
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return false
	}
	s.setHeaders(req)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode == http.StatusOK
}

// dashboardExists 检查 Dashboard 是否存在
func (s *GrafanaService) dashboardExists(uid string) bool {
	apiURL := fmt.Sprintf("%s/api/dashboards/uid/%s", s.baseURL, uid)
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return false
	}
	s.setHeaders(req)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode == http.StatusOK
}

// importDashboard 导入 Dashboard 到指定文件夹
func (s *GrafanaService) importDashboard(dashboardJSON map[string]interface{}, folderUID string) error {
	// 移除 id 字段以确保新建或覆盖
	delete(dashboardJSON, "id")

	reqBody, err := json.Marshal(map[string]interface{}{
		"dashboard": dashboardJSON,
		"folderUid": folderUID,
		"overwrite": true,
	})
	if err != nil {
		return fmt.Errorf("序列化请求失败: %w", err)
	}

	apiURL := fmt.Sprintf("%s/api/dashboards/db", s.baseURL)
	req, err := http.NewRequest("POST", apiURL, bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}
	s.setHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("导入请求失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("导入失败: status=%d, body=%s", resp.StatusCode, string(body))
	}

	return nil
}

// DataSourceSyncStatus 数据源同步状态
type DataSourceSyncStatus struct {
	DataSources []DataSourceStatusItem `json:"datasources"`
	AllSynced   bool                   `json:"all_synced"`
}

// DataSourceStatusItem 单个数据源的状态
type DataSourceStatusItem struct {
	ClusterName   string `json:"cluster_name"`
	DataSourceUID string `json:"datasource_uid"`
	PrometheusURL string `json:"prometheus_url"`
	Exists        bool   `json:"exists"`
}

// GetDataSourceSyncStatus 获取所有集群的数据源同步状态
func (s *GrafanaService) GetDataSourceSyncStatus(clusters []DataSourceClusterInfo) (*DataSourceSyncStatus, error) {
	if !s.IsEnabled() {
		return nil, fmt.Errorf("grafana 服务未配置")
	}

	status := &DataSourceSyncStatus{
		AllSynced:   true,
		DataSources: []DataSourceStatusItem{},
	}

	for _, c := range clusters {
		uid := GenerateDataSourceUID(c.ClusterName)
		dsName := fmt.Sprintf("Prometheus-%s", c.ClusterName)
		exists, _ := s.dataSourceExists(dsName)

		status.DataSources = append(status.DataSources, DataSourceStatusItem{
			ClusterName:   c.ClusterName,
			DataSourceUID: uid,
			PrometheusURL: c.PrometheusURL,
			Exists:        exists,
		})
		if !exists {
			status.AllSynced = false
		}
	}

	if len(clusters) == 0 {
		status.AllSynced = false
	}

	return status, nil
}

// SyncAllDataSources 批量同步所有集群的数据源
func (s *GrafanaService) SyncAllDataSources(clusters []DataSourceClusterInfo) (*DataSourceSyncStatus, error) {
	if !s.IsEnabled() {
		return nil, fmt.Errorf("grafana 服务未配置")
	}

	status := &DataSourceSyncStatus{
		AllSynced:   true,
		DataSources: []DataSourceStatusItem{},
	}

	for _, c := range clusters {
		uid := GenerateDataSourceUID(c.ClusterName)
		err := s.SyncDataSource(c.ClusterName, c.PrometheusURL)

		item := DataSourceStatusItem{
			ClusterName:   c.ClusterName,
			DataSourceUID: uid,
			PrometheusURL: c.PrometheusURL,
			Exists:        err == nil,
		}
		status.DataSources = append(status.DataSources, item)
		if err != nil {
			logger.Error("同步数据源失败", "cluster", c.ClusterName, "error", err)
			status.AllSynced = false
		}
	}

	if len(clusters) == 0 {
		status.AllSynced = false
	}

	return status, nil
}

// DataSourceClusterInfo 用于数据源同步的集群信息
type DataSourceClusterInfo struct {
	ClusterName   string
	PrometheusURL string
}

// ListDataSources 列出所有数据源
func (s *GrafanaService) ListDataSources() ([]DataSourceResponse, error) {
	if !s.IsEnabled() {
		return nil, fmt.Errorf("grafana 服务未配置")
	}

	url := fmt.Sprintf("%s/api/datasources", s.baseURL)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	s.setHeaders(req)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("获取数据源列表失败: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("获取数据源列表失败: status=%d", resp.StatusCode)
	}

	var dataSources []DataSourceResponse
	if err := json.NewDecoder(resp.Body).Decode(&dataSources); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	return dataSources, nil
}
