package services

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/shaia/Synapse/pkg/logger"
)

//go:embed dashboards/*.json
var dashboardFS embed.FS

// GrafanaService Grafana API 服務
type GrafanaService struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// DataSourceRequest Grafana 資料來源請求
type DataSourceRequest struct {
	Name      string                 `json:"name"`
	UID       string                 `json:"uid,omitempty"`
	Type      string                 `json:"type"`
	URL       string                 `json:"url"`
	Access    string                 `json:"access"`
	IsDefault bool                   `json:"isDefault"`
	JSONData  map[string]interface{} `json:"jsonData,omitempty"`
}

// GenerateDataSourceUID 根據叢集名生成資料來源 UID
func GenerateDataSourceUID(clusterName string) string {
	// 轉為小寫，替換特殊字元為連字元
	uid := strings.ToLower(clusterName)
	uid = strings.ReplaceAll(uid, " ", "-")
	uid = strings.ReplaceAll(uid, "_", "-")
	return fmt.Sprintf("prometheus-%s", uid)
}

// DataSourceResponse Grafana 資料來源響應
type DataSourceResponse struct {
	ID        int    `json:"id"`
	UID       string `json:"uid"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	URL       string `json:"url"`
	IsDefault bool   `json:"isDefault"`
}

// NewGrafanaService 建立 Grafana 服務
func NewGrafanaService(baseURL, apiKey string) *GrafanaService {
	return &GrafanaService{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// IsEnabled 檢查 Grafana 服務是否啟用
func (s *GrafanaService) IsEnabled() bool {
	return s.baseURL != "" && s.apiKey != ""
}

// UpdateConfig 熱更新 Grafana 連線配置
func (s *GrafanaService) UpdateConfig(baseURL, apiKey string) {
	s.baseURL = strings.TrimSuffix(baseURL, "/")
	s.apiKey = apiKey
}

// GetBaseURL 獲取當前 Grafana 地址
func (s *GrafanaService) GetBaseURL() string {
	return s.baseURL
}

// SyncDataSource 同步資料來源（建立或更新）
func (s *GrafanaService) SyncDataSource(clusterName, prometheusURL string) error {
	if !s.IsEnabled() {
		logger.Info("Grafana 服務未啟用，跳過資料來源同步")
		return nil
	}

	if prometheusURL == "" {
		logger.Info("Prometheus URL 為空，跳過資料來源同步", "cluster", clusterName)
		return nil
	}

	dataSourceName := fmt.Sprintf("Prometheus-%s", clusterName)

	// 先檢查資料來源是否存在
	exists, err := s.dataSourceExists(dataSourceName)
	if err != nil {
		logger.Error("檢查資料來源是否存在失敗", "error", err)
		// 繼續嘗試建立
	}

	if exists {
		// 更新現有資料來源
		return s.updateDataSource(dataSourceName, clusterName, prometheusURL)
	}

	// 建立新資料來源
	return s.createDataSource(dataSourceName, clusterName, prometheusURL)
}

// DeleteDataSource 刪除資料來源
func (s *GrafanaService) DeleteDataSource(clusterName string) error {
	if !s.IsEnabled() {
		logger.Info("Grafana 服務未啟用，跳過資料來源刪除")
		return nil
	}

	dataSourceName := fmt.Sprintf("Prometheus-%s", clusterName)

	url := fmt.Sprintf("%s/api/datasources/name/%s", s.baseURL, dataSourceName)
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("建立刪除請求失敗: %w", err)
	}

	s.setHeaders(req)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("刪除資料來源請求失敗: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode == http.StatusNotFound {
		logger.Info("資料來源不存在，無需刪除", "name", dataSourceName)
		return nil
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("刪除資料來源失敗: status=%d, body=%s", resp.StatusCode, string(body))
	}

	logger.Info("Grafana 資料來源刪除成功", "name", dataSourceName)
	return nil
}

// dataSourceExists 檢查資料來源是否存在
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

// createDataSource 建立資料來源
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
		return fmt.Errorf("序列化資料來源請求失敗: %w", err)
	}

	url := fmt.Sprintf("%s/api/datasources", s.baseURL)
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("建立請求失敗: %w", err)
	}

	s.setHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("建立資料來源請求失敗: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("建立資料來源失敗: status=%d, body=%s", resp.StatusCode, string(respBody))
	}

	logger.Info("Grafana 資料來源建立成功", "name", name, "url", prometheusURL)
	return nil
}

// updateDataSource 更新資料來源
func (s *GrafanaService) updateDataSource(name, clusterName, prometheusURL string) error {
	// 先獲取資料來源 ID
	url := fmt.Sprintf("%s/api/datasources/name/%s", s.baseURL, name)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("建立獲取請求失敗: %w", err)
	}

	s.setHeaders(req)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("獲取資料來源失敗: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("資料來源不存在: %s", name)
	}

	var ds DataSourceResponse
	if err := json.NewDecoder(resp.Body).Decode(&ds); err != nil {
		return fmt.Errorf("解析資料來源響應失敗: %w", err)
	}

	// 更新資料來源
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
		return fmt.Errorf("序列化資料來源請求失敗: %w", err)
	}

	updateURL := fmt.Sprintf("%s/api/datasources/%d", s.baseURL, ds.ID)
	updateReq, err := http.NewRequest("PUT", updateURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("建立更新請求失敗: %w", err)
	}

	s.setHeaders(updateReq)
	updateReq.Header.Set("Content-Type", "application/json")

	updateResp, err := s.httpClient.Do(updateReq)
	if err != nil {
		return fmt.Errorf("更新資料來源請求失敗: %w", err)
	}
	defer func() {
		_ = updateResp.Body.Close()
	}()

	if updateResp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(updateResp.Body)
		return fmt.Errorf("更新資料來源失敗: status=%d, body=%s", updateResp.StatusCode, string(respBody))
	}

	logger.Info("Grafana 資料來源更新成功", "name", name, "url", prometheusURL)
	return nil
}

// setHeaders 設定請求頭
func (s *GrafanaService) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.apiKey))
	req.Header.Set("Accept", "application/json")
}

// TestConnection 測試 Grafana 連線
func (s *GrafanaService) TestConnection() error {
	if !s.IsEnabled() {
		return fmt.Errorf("grafana 服務未配置")
	}

	url := fmt.Sprintf("%s/api/health", s.baseURL)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("建立請求失敗: %w", err)
	}

	s.setHeaders(req)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("連線 Grafana 失敗: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("grafana 健康檢查失敗: status=%d", resp.StatusCode)
	}

	return nil
}

// DataSourceSyncStatus 資料來源同步狀態
type DataSourceSyncStatus struct {
	DataSources []DataSourceStatusItem `json:"datasources"`
	AllSynced   bool                   `json:"all_synced"`
}

// DataSourceStatusItem 單個資料來源的狀態
type DataSourceStatusItem struct {
	ClusterName   string `json:"cluster_name"`
	DataSourceUID string `json:"datasource_uid"`
	PrometheusURL string `json:"prometheus_url"`
	Exists        bool   `json:"exists"`
}

// GetDataSourceSyncStatus 獲取所有叢集的資料來源同步狀態
func (s *GrafanaService) GetDataSourceSyncStatus(clusters []DataSourceClusterInfo) (*DataSourceSyncStatus, error) {
	if !s.IsEnabled() {
		return nil, fmt.Errorf("grafana 服務未配置")
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

// SyncAllDataSources 批次同步所有叢集的資料來源
func (s *GrafanaService) SyncAllDataSources(clusters []DataSourceClusterInfo) (*DataSourceSyncStatus, error) {
	if !s.IsEnabled() {
		return nil, fmt.Errorf("grafana 服務未配置")
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
			logger.Error("同步資料來源失敗", "cluster", c.ClusterName, "error", err)
			status.AllSynced = false
		}
	}

	if len(clusters) == 0 {
		status.AllSynced = false
	}

	return status, nil
}

// DataSourceClusterInfo 用於資料來源同步的叢集資訊
type DataSourceClusterInfo struct {
	ClusterName   string
	PrometheusURL string
}

// ListDataSources 列出所有資料來源
func (s *GrafanaService) ListDataSources() ([]DataSourceResponse, error) {
	if !s.IsEnabled() {
		return nil, fmt.Errorf("grafana 服務未配置")
	}

	url := fmt.Sprintf("%s/api/datasources", s.baseURL)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("建立請求失敗: %w", err)
	}

	s.setHeaders(req)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("獲取資料來源列表失敗: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("獲取資料來源列表失敗: status=%d", resp.StatusCode)
	}

	var dataSources []DataSourceResponse
	if err := json.NewDecoder(resp.Body).Decode(&dataSources); err != nil {
		return nil, fmt.Errorf("解析響應失敗: %w", err)
	}

	return dataSources, nil
}
