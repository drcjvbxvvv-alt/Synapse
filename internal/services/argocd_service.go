package services

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"gorm.io/gorm"

	"kubepolaris/internal/models"
	"kubepolaris/pkg/logger"
)

// ArgoCDService ArgoCD 服务
type ArgoCDService struct {
	db         *gorm.DB
	httpClient *http.Client
}

// NewArgoCDService 创建 ArgoCD 服务
func NewArgoCDService(db *gorm.DB) *ArgoCDService {
	return &ArgoCDService{
		db: db,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetConfig 获取 ArgoCD 配置
func (s *ArgoCDService) GetConfig(ctx context.Context, clusterID uint) (*models.ArgoCDConfig, error) {
	var config models.ArgoCDConfig
	if err := s.db.Where("cluster_id = ?", clusterID).First(&config).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// 返回默认空配置
			return &models.ArgoCDConfig{
				ClusterID:     clusterID,
				Enabled:       false,
				GitBranch:     "main",
				ArgoCDProject: "default",
			}, nil
		}
		return nil, err
	}
	return &config, nil
}

// SaveConfig 保存 ArgoCD 配置
func (s *ArgoCDService) SaveConfig(ctx context.Context, config *models.ArgoCDConfig) error {
	var existing models.ArgoCDConfig
	if err := s.db.Where("cluster_id = ?", config.ClusterID).First(&existing).Error; err == nil {
		// 更新
		config.ID = existing.ID
		config.CreatedAt = existing.CreatedAt
		return s.db.Save(config).Error
	}
	// 新建
	return s.db.Create(config).Error
}

// TestConnection 测试 ArgoCD 连接
func (s *ArgoCDService) TestConnection(ctx context.Context, config *models.ArgoCDConfig) error {
	client := s.createHTTPClient(config.Insecure)

	// 如果使用用户名密码认证，先尝试登录获取 token
	var authToken string
	if config.Token != "" {
		authToken = config.Token
	} else if config.Username != "" && config.Password != "" {
		// 使用用户名密码登录
		token, err := s.getSessionToken(config)
		if err != nil {
			return fmt.Errorf("用户名密码认证失败: %w", err)
		}
		authToken = token
	} else {
		return fmt.Errorf("请提供 API Token 或用户名密码")
	}

	// 使用获取到的 token 验证连接
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/api/v1/session/userinfo", config.ServerURL), nil)
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", authToken))

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("连接 ArgoCD 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("认证失败: Token 无效或已过期")
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ArgoCD 响应错误 (状态码 %d): %s", resp.StatusCode, string(body))
	}

	// 更新连接状态
	now := time.Now()
	config.ConnectionStatus = "connected"
	config.LastTestAt = &now
	config.ErrorMessage = ""

	return nil
}

// ListApplications 获取应用列表
func (s *ArgoCDService) ListApplications(ctx context.Context, clusterID uint) ([]models.ArgoCDApplication, error) {
	config, err := s.GetConfig(ctx, clusterID)
	if err != nil {
		return nil, err
	}
	if !config.Enabled {
		return nil, fmt.Errorf("ArgoCD 集成未启用，请先在插件中心配置")
	}

	client := s.createHTTPClient(config.Insecure)

	// 构建查询参数，按项目过滤
	url := fmt.Sprintf("%s/api/v1/applications", config.ServerURL)
	if config.ArgoCDProject != "" && config.ArgoCDProject != "default" {
		url = fmt.Sprintf("%s?projects=%s", url, config.ArgoCDProject)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	s.setAuthHeader(req, config)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("获取应用列表失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ArgoCD API 错误 (状态码 %d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Items []argoCDAppResponse `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	// 转换为我们的模型，并过滤只属于当前集群的应用
	apps := make([]models.ArgoCDApplication, 0)
	for _, item := range result.Items {
		// 只返回目标集群匹配的应用
		if config.ArgoCDClusterName == "" ||
			item.Spec.Destination.Name == config.ArgoCDClusterName ||
			item.Spec.Destination.Server == config.ArgoCDClusterName {
			apps = append(apps, s.convertApplication(item))
		}
	}

	logger.Info("获取 ArgoCD 应用列表成功", "cluster_id", clusterID, "count", len(apps))
	return apps, nil
}

// GetApplication 获取单个应用详情
func (s *ArgoCDService) GetApplication(ctx context.Context, clusterID uint, appName string) (*models.ArgoCDApplication, error) {
	config, err := s.GetConfig(ctx, clusterID)
	if err != nil {
		return nil, err
	}
	if !config.Enabled {
		return nil, fmt.Errorf("ArgoCD 集成未启用")
	}

	client := s.createHTTPClient(config.Insecure)
	url := fmt.Sprintf("%s/api/v1/applications/%s", config.ServerURL, appName)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	s.setAuthHeader(req, config)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("获取应用详情失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("应用 %s 不存在", appName)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("获取应用详情失败: %s", string(body))
	}

	var item argoCDAppResponse
	if err := json.NewDecoder(resp.Body).Decode(&item); err != nil {
		return nil, err
	}

	app := s.convertApplication(item)
	return &app, nil
}

// CreateApplication 创建应用
func (s *ArgoCDService) CreateApplication(ctx context.Context, clusterID uint, req *models.CreateApplicationRequest) (*models.ArgoCDApplication, error) {
	config, err := s.GetConfig(ctx, clusterID)
	if err != nil {
		return nil, err
	}
	if !config.Enabled {
		return nil, fmt.Errorf("ArgoCD 集成未启用")
	}

	// 设置默认值
	namespace := req.Namespace
	if namespace == "" {
		namespace = "argocd"
	}

	project := req.Project
	if project == "" {
		project = config.ArgoCDProject
	}
	if project == "" {
		project = "default"
	}

	targetRevision := req.TargetRevision
	if targetRevision == "" {
		targetRevision = config.GitBranch
	}
	if targetRevision == "" {
		targetRevision = "HEAD"
	}

	// 构建 ArgoCD Application 请求体
	appSpec := map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      req.Name,
			"namespace": namespace,
		},
		"spec": map[string]interface{}{
			"project": project,
			"source": map[string]interface{}{
				"repoURL":        config.GitRepoURL,
				"path":           req.Path,
				"targetRevision": targetRevision,
			},
			"destination": map[string]interface{}{
				"name":      config.ArgoCDClusterName,
				"namespace": req.DestNamespace,
			},
		},
	}

	// 添加 Helm 配置
	if req.HelmValues != "" || len(req.HelmParameters) > 0 {
		source := appSpec["spec"].(map[string]interface{})["source"].(map[string]interface{})
		helm := map[string]interface{}{}
		if req.HelmValues != "" {
			helm["values"] = req.HelmValues
		}
		if len(req.HelmParameters) > 0 {
			params := make([]map[string]string, 0)
			for k, v := range req.HelmParameters {
				params = append(params, map[string]string{"name": k, "value": v})
			}
			helm["parameters"] = params
		}
		source["helm"] = helm
	}

	// 添加同步策略
	if req.AutoSync {
		spec := appSpec["spec"].(map[string]interface{})
		syncPolicy := map[string]interface{}{
			"automated": map[string]interface{}{
				"selfHeal": req.SelfHeal,
				"prune":    req.Prune,
			},
		}
		spec["syncPolicy"] = syncPolicy
	}

	body, err := json.Marshal(appSpec)
	if err != nil {
		return nil, err
	}

	client := s.createHTTPClient(config.Insecure)
	httpReq, err := http.NewRequestWithContext(ctx, "POST",
		fmt.Sprintf("%s/api/v1/applications", config.ServerURL),
		bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	s.setAuthHeader(httpReq, config)

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("创建应用失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ArgoCD 创建应用失败 (状态码 %d): %s", resp.StatusCode, string(respBody))
	}

	var item argoCDAppResponse
	if err := json.NewDecoder(resp.Body).Decode(&item); err != nil {
		return nil, err
	}

	logger.Info("创建 ArgoCD 应用成功", "cluster_id", clusterID, "app_name", req.Name)
	app := s.convertApplication(item)
	return &app, nil
}

// UpdateApplication 更新应用
func (s *ArgoCDService) UpdateApplication(ctx context.Context, clusterID uint, appName string, req *models.CreateApplicationRequest) (*models.ArgoCDApplication, error) {
	config, err := s.GetConfig(ctx, clusterID)
	if err != nil {
		return nil, err
	}
	if !config.Enabled {
		return nil, fmt.Errorf("ArgoCD 集成未启用")
	}

	// 先获取现有应用
	existingApp, err := s.GetApplication(ctx, clusterID, appName)
	if err != nil {
		return nil, fmt.Errorf("获取应用失败: %w", err)
	}

	// 构建更新请求
	targetRevision := req.TargetRevision
	if targetRevision == "" {
		targetRevision = existingApp.Source.TargetRevision
	}

	appSpec := map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      appName,
			"namespace": existingApp.Namespace,
		},
		"spec": map[string]interface{}{
			"project": existingApp.Project,
			"source": map[string]interface{}{
				"repoURL":        config.GitRepoURL,
				"path":           req.Path,
				"targetRevision": targetRevision,
			},
			"destination": map[string]interface{}{
				"name":      config.ArgoCDClusterName,
				"namespace": req.DestNamespace,
			},
		},
	}

	// 添加 Helm 配置
	if req.HelmValues != "" || len(req.HelmParameters) > 0 {
		source := appSpec["spec"].(map[string]interface{})["source"].(map[string]interface{})
		helm := map[string]interface{}{}
		if req.HelmValues != "" {
			helm["values"] = req.HelmValues
		}
		if len(req.HelmParameters) > 0 {
			params := make([]map[string]string, 0)
			for k, v := range req.HelmParameters {
				params = append(params, map[string]string{"name": k, "value": v})
			}
			helm["parameters"] = params
		}
		source["helm"] = helm
	}

	// 添加同步策略
	spec := appSpec["spec"].(map[string]interface{})
	if req.AutoSync {
		syncPolicy := map[string]interface{}{
			"automated": map[string]interface{}{
				"selfHeal": req.SelfHeal,
				"prune":    req.Prune,
			},
		}
		spec["syncPolicy"] = syncPolicy
	} else {
		spec["syncPolicy"] = nil
	}

	body, err := json.Marshal(appSpec)
	if err != nil {
		return nil, err
	}

	client := s.createHTTPClient(config.Insecure)
	httpReq, err := http.NewRequestWithContext(ctx, "PUT",
		fmt.Sprintf("%s/api/v1/applications/%s", config.ServerURL, appName),
		bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	s.setAuthHeader(httpReq, config)

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("更新应用失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ArgoCD 更新应用失败: %s", string(respBody))
	}

	var item argoCDAppResponse
	if err := json.NewDecoder(resp.Body).Decode(&item); err != nil {
		return nil, err
	}

	logger.Info("更新 ArgoCD 应用成功", "cluster_id", clusterID, "app_name", appName)
	app := s.convertApplication(item)
	return &app, nil
}

// SyncApplication 同步应用
func (s *ArgoCDService) SyncApplication(ctx context.Context, clusterID uint, appName string, revision string) error {
	config, err := s.GetConfig(ctx, clusterID)
	if err != nil {
		return err
	}
	if !config.Enabled {
		return fmt.Errorf("ArgoCD 集成未启用")
	}

	syncReq := map[string]interface{}{
		"prune": true,
	}
	if revision != "" {
		syncReq["revision"] = revision
	}

	body, _ := json.Marshal(syncReq)

	client := s.createHTTPClient(config.Insecure)
	req, err := http.NewRequestWithContext(ctx, "POST",
		fmt.Sprintf("%s/api/v1/applications/%s/sync", config.ServerURL, appName),
		bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	s.setAuthHeader(req, config)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("同步应用失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("同步失败 (状态码 %d): %s", resp.StatusCode, string(respBody))
	}

	logger.Info("触发 ArgoCD 应用同步成功", "cluster_id", clusterID, "app_name", appName)
	return nil
}

// DeleteApplication 删除应用
func (s *ArgoCDService) DeleteApplication(ctx context.Context, clusterID uint, appName string, cascade bool) error {
	config, err := s.GetConfig(ctx, clusterID)
	if err != nil {
		return err
	}
	if !config.Enabled {
		return fmt.Errorf("ArgoCD 集成未启用")
	}

	client := s.createHTTPClient(config.Insecure)
	url := fmt.Sprintf("%s/api/v1/applications/%s?cascade=%t", config.ServerURL, appName, cascade)

	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	s.setAuthHeader(req, config)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("删除应用失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("删除失败 (状态码 %d): %s", resp.StatusCode, string(respBody))
	}

	logger.Info("删除 ArgoCD 应用成功", "cluster_id", clusterID, "app_name", appName, "cascade", cascade)
	return nil
}

// RollbackApplication 回滚应用
func (s *ArgoCDService) RollbackApplication(ctx context.Context, clusterID uint, appName string, revisionID int64) error {
	config, err := s.GetConfig(ctx, clusterID)
	if err != nil {
		return err
	}
	if !config.Enabled {
		return fmt.Errorf("ArgoCD 集成未启用")
	}

	rollbackReq := map[string]interface{}{
		"id": revisionID,
	}
	body, _ := json.Marshal(rollbackReq)

	client := s.createHTTPClient(config.Insecure)
	req, err := http.NewRequestWithContext(ctx, "POST",
		fmt.Sprintf("%s/api/v1/applications/%s/rollback", config.ServerURL, appName),
		bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	s.setAuthHeader(req, config)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("回滚失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("回滚失败 (状态码 %d): %s", resp.StatusCode, string(respBody))
	}

	logger.Info("回滚 ArgoCD 应用成功", "cluster_id", clusterID, "app_name", appName, "revision_id", revisionID)
	return nil
}

// GetApplicationResources 获取应用资源树
func (s *ArgoCDService) GetApplicationResources(ctx context.Context, clusterID uint, appName string) ([]models.ArgoCDResource, error) {
	config, err := s.GetConfig(ctx, clusterID)
	if err != nil {
		return nil, err
	}
	if !config.Enabled {
		return nil, fmt.Errorf("ArgoCD 集成未启用")
	}

	client := s.createHTTPClient(config.Insecure)
	url := fmt.Sprintf("%s/api/v1/applications/%s/resource-tree", config.ServerURL, appName)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	s.setAuthHeader(req, config)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("获取资源树失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("获取资源树失败: %s", string(body))
	}

	var result struct {
		Nodes []struct {
			Group           string `json:"group"`
			Kind            string `json:"kind"`
			Namespace       string `json:"namespace"`
			Name            string `json:"name"`
			ResourceVersion string `json:"resourceVersion"`
			Health          struct {
				Status  string `json:"status"`
				Message string `json:"message"`
			} `json:"health"`
		} `json:"nodes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	resources := make([]models.ArgoCDResource, 0, len(result.Nodes))
	for _, node := range result.Nodes {
		resources = append(resources, models.ArgoCDResource{
			Group:     node.Group,
			Kind:      node.Kind,
			Namespace: node.Namespace,
			Name:      node.Name,
			Health:    node.Health.Status,
			Message:   node.Health.Message,
		})
	}

	return resources, nil
}

// 辅助方法

func (s *ArgoCDService) createHTTPClient(insecure bool) *http.Client {
	if insecure {
		return &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		}
	}
	return s.httpClient
}

func (s *ArgoCDService) setAuthHeader(req *http.Request, config *models.ArgoCDConfig) {
	// 优先使用 Token 认证
	if config.Token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", config.Token))
		return
	}

	// 如果没有 Token 但有用户名密码，尝试获取 session token
	if config.Username != "" && config.Password != "" {
		token, err := s.getSessionToken(config)
		if err != nil {
			logger.Error("获取 ArgoCD session token 失败", "error", err)
			return
		}
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	}
}

// getSessionToken 使用用户名密码获取 ArgoCD session token
func (s *ArgoCDService) getSessionToken(config *models.ArgoCDConfig) (string, error) {
	client := s.createHTTPClient(config.Insecure)

	// 构建登录请求
	loginReq := map[string]string{
		"username": config.Username,
		"password": config.Password,
	}
	body, err := json.Marshal(loginReq)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/session", config.ServerURL), bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("创建登录请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("登录请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("登录失败 (状态码 %d): %s", resp.StatusCode, string(respBody))
	}

	// 解析响应获取 token
	var result struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("解析登录响应失败: %w", err)
	}

	if result.Token == "" {
		return "", fmt.Errorf("登录成功但未返回 token")
	}

	logger.Info("ArgoCD 登录成功，获取到 session token")
	return result.Token, nil
}

// ArgoCD API 响应结构
type argoCDAppResponse struct {
	Metadata struct {
		Name              string `json:"name"`
		Namespace         string `json:"namespace"`
		CreationTimestamp string `json:"creationTimestamp"`
	} `json:"metadata"`
	Spec struct {
		Project string `json:"project"`
		Source  struct {
			RepoURL        string `json:"repoURL"`
			Path           string `json:"path"`
			TargetRevision string `json:"targetRevision"`
			Helm           *struct {
				ValueFiles []string `json:"valueFiles"`
				Values     string   `json:"values"`
				Parameters []struct {
					Name  string `json:"name"`
					Value string `json:"value"`
				} `json:"parameters"`
			} `json:"helm"`
		} `json:"source"`
		Destination struct {
			Server    string `json:"server"`
			Namespace string `json:"namespace"`
			Name      string `json:"name"`
		} `json:"destination"`
		SyncPolicy *struct {
			Automated *struct {
				SelfHeal bool `json:"selfHeal"`
				Prune    bool `json:"prune"`
			} `json:"automated"`
		} `json:"syncPolicy"`
	} `json:"spec"`
	Status struct {
		Sync struct {
			Status   string `json:"status"`
			Revision string `json:"revision"`
		} `json:"sync"`
		Health struct {
			Status string `json:"status"`
		} `json:"health"`
		ReconciledAt string `json:"reconciledAt"`
		Resources    []struct {
			Group     string `json:"group"`
			Kind      string `json:"kind"`
			Namespace string `json:"namespace"`
			Name      string `json:"name"`
			Status    string `json:"status"`
			Health    struct {
				Status  string `json:"status"`
				Message string `json:"message"`
			} `json:"health"`
		} `json:"resources"`
		History []struct {
			ID         int64  `json:"id"`
			Revision   string `json:"revision"`
			DeployedAt string `json:"deployedAt"`
			Source     struct {
				RepoURL        string `json:"repoURL"`
				Path           string `json:"path"`
				TargetRevision string `json:"targetRevision"`
			} `json:"source"`
		} `json:"history"`
	} `json:"status"`
}

func (s *ArgoCDService) convertApplication(item argoCDAppResponse) models.ArgoCDApplication {
	app := models.ArgoCDApplication{
		Name:           item.Metadata.Name,
		Namespace:      item.Metadata.Namespace,
		Project:        item.Spec.Project,
		SyncStatus:     item.Status.Sync.Status,
		HealthStatus:   item.Status.Health.Status,
		SyncedRevision: item.Status.Sync.Revision,
		TargetRevision: item.Spec.Source.TargetRevision,
		CreatedAt:      item.Metadata.CreationTimestamp,
		ReconciledAt:   item.Status.ReconciledAt,
		Source: models.ArgoCDSource{
			RepoURL:        item.Spec.Source.RepoURL,
			Path:           item.Spec.Source.Path,
			TargetRevision: item.Spec.Source.TargetRevision,
		},
		Destination: models.ArgoCDDestination{
			Server:    item.Spec.Destination.Server,
			Namespace: item.Spec.Destination.Namespace,
			Name:      item.Spec.Destination.Name,
		},
	}

	// 转换 Helm 配置
	if item.Spec.Source.Helm != nil {
		app.Source.Helm = &models.ArgoCDHelmSource{
			ValueFiles: item.Spec.Source.Helm.ValueFiles,
			Values:     item.Spec.Source.Helm.Values,
		}
		for _, p := range item.Spec.Source.Helm.Parameters {
			app.Source.Helm.Parameters = append(app.Source.Helm.Parameters, models.ArgoCDHelmParam{
				Name:  p.Name,
				Value: p.Value,
			})
		}
	}

	// 转换资源列表
	for _, res := range item.Status.Resources {
		app.Resources = append(app.Resources, models.ArgoCDResource{
			Group:     res.Group,
			Kind:      res.Kind,
			Namespace: res.Namespace,
			Name:      res.Name,
			Status:    res.Status,
			Health:    res.Health.Status,
			Message:   res.Health.Message,
		})
	}

	// 转换同步历史
	for _, h := range item.Status.History {
		app.History = append(app.History, models.ArgoCDSyncHistory{
			ID:         h.ID,
			Revision:   h.Revision,
			DeployedAt: h.DeployedAt,
			Source: models.ArgoCDSource{
				RepoURL:        h.Source.RepoURL,
				Path:           h.Source.Path,
				TargetRevision: h.Source.TargetRevision,
			},
		})
	}

	return app
}
