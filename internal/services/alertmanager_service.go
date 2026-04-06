package services

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/clay-wangzhi/Synapse/internal/models"
	"github.com/clay-wangzhi/Synapse/pkg/logger"
)

// AlertManagerService Alertmanager 服務
type AlertManagerService struct {
	httpClient *http.Client
}

// NewAlertManagerService 建立 Alertmanager 服務
func NewAlertManagerService() *AlertManagerService {
	return &AlertManagerService{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true, // #nosec G402 -- 內部叢集 AlertManager 通訊，使用者可自行配置證書
				},
			},
		},
	}
}

// TestConnection 測試 Alertmanager 連線
func (s *AlertManagerService) TestConnection(ctx context.Context, config *models.AlertManagerConfig) error {
	if !config.Enabled {
		return fmt.Errorf("alertmanager 未啟用")
	}

	// 構建測試 URL
	testURL, err := url.Parse(config.Endpoint)
	if err != nil {
		return fmt.Errorf("無效的 Alertmanager 端點: %w", err)
	}
	testURL.Path = "/api/v2/status"

	// 建立測試請求
	req, err := http.NewRequestWithContext(ctx, "GET", testURL.String(), nil)
	if err != nil {
		return fmt.Errorf("建立測試請求失敗: %w", err)
	}

	// 設定認證
	if err := s.setAuth(req, config.Auth); err != nil {
		return fmt.Errorf("設定認證失敗: %w", err)
	}

	// 執行測試請求
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("連線測試失敗: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("alertmanager 響應異常: %s, 狀態碼: %d", string(body), resp.StatusCode)
	}

	return nil
}

// GetAlerts 獲取告警列表
func (s *AlertManagerService) GetAlerts(ctx context.Context, config *models.AlertManagerConfig, filter map[string]string) ([]models.Alert, error) {
	if !config.Enabled {
		return nil, fmt.Errorf("alertmanager 未啟用")
	}

	// 構建 URL
	alertsURL, err := url.Parse(config.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("無效的 Alertmanager 端點: %w", err)
	}
	alertsURL.Path = "/api/v2/alerts"

	// 新增過濾參數
	params := url.Values{}
	for key, value := range filter {
		if value != "" {
			params.Add("filter", fmt.Sprintf("%s=%s", key, value))
		}
	}
	if len(params) > 0 {
		alertsURL.RawQuery = params.Encode()
	}

	// 建立請求
	req, err := http.NewRequestWithContext(ctx, "GET", alertsURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("建立請求失敗: %w", err)
	}

	// 設定認證
	if err := s.setAuth(req, config.Auth); err != nil {
		return nil, fmt.Errorf("設定認證失敗: %w", err)
	}

	// 執行請求
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("獲取告警失敗: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// 讀取響應
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("讀取響應失敗: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("獲取告警失敗: %s, 狀態碼: %d", string(body), resp.StatusCode)
	}

	// 解析響應 - Alertmanager v2 API 直接返回陣列
	var alerts []models.Alert
	if err := json.Unmarshal(body, &alerts); err != nil {
		return nil, fmt.Errorf("解析告警響應失敗: %w", err)
	}

	return alerts, nil
}

// GetAlertGroups 獲取告警分組
func (s *AlertManagerService) GetAlertGroups(ctx context.Context, config *models.AlertManagerConfig) ([]models.AlertGroup, error) {
	if !config.Enabled {
		return nil, fmt.Errorf("alertmanager 未啟用")
	}

	// 構建 URL
	groupsURL, err := url.Parse(config.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("無效的 Alertmanager 端點: %w", err)
	}
	groupsURL.Path = "/api/v2/alerts/groups"

	// 建立請求
	req, err := http.NewRequestWithContext(ctx, "GET", groupsURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("建立請求失敗: %w", err)
	}

	// 設定認證
	if err := s.setAuth(req, config.Auth); err != nil {
		return nil, fmt.Errorf("設定認證失敗: %w", err)
	}

	// 執行請求
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("獲取告警分組失敗: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// 讀取響應
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("讀取響應失敗: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("獲取告警分組失敗: %s, 狀態碼: %d", string(body), resp.StatusCode)
	}

	// 解析響應
	var groups []models.AlertGroup
	if err := json.Unmarshal(body, &groups); err != nil {
		return nil, fmt.Errorf("解析告警分組響應失敗: %w", err)
	}

	return groups, nil
}

// GetSilences 獲取靜默規則列表
func (s *AlertManagerService) GetSilences(ctx context.Context, config *models.AlertManagerConfig) ([]models.Silence, error) {
	if !config.Enabled {
		return nil, fmt.Errorf("alertmanager 未啟用")
	}

	// 構建 URL
	silencesURL, err := url.Parse(config.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("無效的 Alertmanager 端點: %w", err)
	}
	silencesURL.Path = "/api/v2/silences"

	// 建立請求
	req, err := http.NewRequestWithContext(ctx, "GET", silencesURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("建立請求失敗: %w", err)
	}

	// 設定認證
	if err := s.setAuth(req, config.Auth); err != nil {
		return nil, fmt.Errorf("設定認證失敗: %w", err)
	}

	// 執行請求
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("獲取靜默規則失敗: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// 讀取響應
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("讀取響應失敗: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("獲取靜默規則失敗: %s, 狀態碼: %d", string(body), resp.StatusCode)
	}

	// 解析響應
	var silences []models.Silence
	if err := json.Unmarshal(body, &silences); err != nil {
		return nil, fmt.Errorf("解析靜默規則響應失敗: %w", err)
	}

	return silences, nil
}

// CreateSilence 建立靜默規則
func (s *AlertManagerService) CreateSilence(ctx context.Context, config *models.AlertManagerConfig, silence *models.CreateSilenceRequest) (*models.Silence, error) {
	if !config.Enabled {
		return nil, fmt.Errorf("alertmanager 未啟用")
	}

	// 構建 URL
	silencesURL, err := url.Parse(config.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("無效的 Alertmanager 端點: %w", err)
	}
	silencesURL.Path = "/api/v2/silences"

	// 序列化請求體
	reqBody, err := json.Marshal(silence)
	if err != nil {
		return nil, fmt.Errorf("序列化請求失敗: %w", err)
	}

	// 建立請求
	req, err := http.NewRequestWithContext(ctx, "POST", silencesURL.String(), strings.NewReader(string(reqBody)))
	if err != nil {
		return nil, fmt.Errorf("建立請求失敗: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// 設定認證
	if err := s.setAuth(req, config.Auth); err != nil {
		return nil, fmt.Errorf("設定認證失敗: %w", err)
	}

	// 執行請求
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("建立靜默規則失敗: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// 讀取響應
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("讀取響應失敗: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("建立靜默規則失敗: %s, 狀態碼: %d", string(body), resp.StatusCode)
	}

	// 解析響應
	var result struct {
		SilenceID string `json:"silenceID"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析響應失敗: %w", err)
	}

	logger.Info("建立靜默規則成功", "silenceID", result.SilenceID)

	// 返回建立的靜默規則
	return &models.Silence{
		ID:        result.SilenceID,
		Matchers:  silence.Matchers,
		StartsAt:  silence.StartsAt,
		EndsAt:    silence.EndsAt,
		CreatedBy: silence.CreatedBy,
		Comment:   silence.Comment,
		Status: models.SilenceStatus{
			State: "active",
		},
	}, nil
}

// DeleteSilence 刪除靜默規則
func (s *AlertManagerService) DeleteSilence(ctx context.Context, config *models.AlertManagerConfig, silenceID string) error {
	if !config.Enabled {
		return fmt.Errorf("alertmanager 未啟用")
	}

	// 構建 URL
	silenceURL, err := url.Parse(config.Endpoint)
	if err != nil {
		return fmt.Errorf("無效的 Alertmanager 端點: %w", err)
	}
	silenceURL.Path = fmt.Sprintf("/api/v2/silence/%s", silenceID)

	// 建立請求
	req, err := http.NewRequestWithContext(ctx, "DELETE", silenceURL.String(), nil)
	if err != nil {
		return fmt.Errorf("建立請求失敗: %w", err)
	}

	// 設定認證
	if err := s.setAuth(req, config.Auth); err != nil {
		return fmt.Errorf("設定認證失敗: %w", err)
	}

	// 執行請求
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("刪除靜默規則失敗: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("刪除靜默規則失敗: %s, 狀態碼: %d", string(body), resp.StatusCode)
	}

	logger.Info("刪除靜默規則成功", "silenceID", silenceID)
	return nil
}

// GetStatus 獲取 Alertmanager 狀態
func (s *AlertManagerService) GetStatus(ctx context.Context, config *models.AlertManagerConfig) (*models.AlertManagerStatus, error) {
	if !config.Enabled {
		return nil, fmt.Errorf("alertmanager 未啟用")
	}

	// 構建 URL
	statusURL, err := url.Parse(config.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("無效的 Alertmanager 端點: %w", err)
	}
	statusURL.Path = "/api/v2/status"

	// 建立請求
	req, err := http.NewRequestWithContext(ctx, "GET", statusURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("建立請求失敗: %w", err)
	}

	// 設定認證
	if err := s.setAuth(req, config.Auth); err != nil {
		return nil, fmt.Errorf("設定認證失敗: %w", err)
	}

	// 執行請求
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("獲取狀態失敗: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// 讀取響應
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("讀取響應失敗: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("獲取狀態失敗: %s, 狀態碼: %d", string(body), resp.StatusCode)
	}

	// 解析響應
	var status models.AlertManagerStatus
	if err := json.Unmarshal(body, &status); err != nil {
		return nil, fmt.Errorf("解析狀態響應失敗: %w", err)
	}

	return &status, nil
}

// GetReceivers 獲取接收器列表
func (s *AlertManagerService) GetReceivers(ctx context.Context, config *models.AlertManagerConfig) ([]models.Receiver, error) {
	if !config.Enabled {
		return nil, fmt.Errorf("alertmanager 未啟用")
	}

	// 構建 URL
	receiversURL, err := url.Parse(config.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("無效的 Alertmanager 端點: %w", err)
	}
	receiversURL.Path = "/api/v2/receivers"

	// 建立請求
	req, err := http.NewRequestWithContext(ctx, "GET", receiversURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("建立請求失敗: %w", err)
	}

	// 設定認證
	if err := s.setAuth(req, config.Auth); err != nil {
		return nil, fmt.Errorf("設定認證失敗: %w", err)
	}

	// 執行請求
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("獲取接收器失敗: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// 讀取響應
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("讀取響應失敗: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("獲取接收器失敗: %s, 狀態碼: %d", string(body), resp.StatusCode)
	}

	// 解析響應
	var receivers []models.Receiver
	if err := json.Unmarshal(body, &receivers); err != nil {
		return nil, fmt.Errorf("解析接收器響應失敗: %w", err)
	}

	return receivers, nil
}

// GetAlertStats 獲取告警統計資訊
func (s *AlertManagerService) GetAlertStats(ctx context.Context, config *models.AlertManagerConfig) (*models.AlertStats, error) {
	alerts, err := s.GetAlerts(ctx, config, nil)
	if err != nil {
		return nil, err
	}

	stats := &models.AlertStats{
		Total:      len(alerts),
		Firing:     0,
		Pending:    0,
		Resolved:   0,
		Suppressed: 0,
		BySeverity: make(map[string]int),
	}

	for _, alert := range alerts {
		// 統計狀態
		switch alert.Status.State {
		case "active":
			stats.Firing++
		case "suppressed":
			stats.Suppressed++
		case "resolved":
			stats.Resolved++
		}

		// 統計嚴重程度
		if severity, ok := alert.Labels["severity"]; ok {
			stats.BySeverity[severity]++
		}
	}

	return stats, nil
}

// setAuth 設定認證
func (s *AlertManagerService) setAuth(req *http.Request, auth *models.MonitoringAuth) error {
	return SetMonitoringAuth(req, auth)
}
