package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"

	"github.com/shaia/Synapse/pkg/logger"
)

// DashboardSyncStatus Dashboard 同步狀態
type DashboardSyncStatus struct {
	FolderExists bool                  `json:"folder_exists"`
	Dashboards   []DashboardStatusItem `json:"dashboards"`
	AllSynced    bool                  `json:"all_synced"`
}

// DashboardStatusItem 單個 Dashboard 的狀態
type DashboardStatusItem struct {
	UID    string `json:"uid"`
	Title  string `json:"title"`
	Exists bool   `json:"exists"`
}

// EnsureDashboards 確保 Synapse 資料夾和 Dashboard 已匯入到 Grafana
func (s *GrafanaService) EnsureDashboards() (*DashboardSyncStatus, error) {
	if !s.IsEnabled() {
		return nil, fmt.Errorf("grafana 服務未配置")
	}

	status := &DashboardSyncStatus{
		Dashboards: []DashboardStatusItem{},
	}

	// 1. 建立 Synapse 資料夾（冪等）
	folderExists, err := s.ensureFolder("synapse-folder", "Synapse")
	if err != nil {
		return nil, fmt.Errorf("建立 Synapse 資料夾失敗: %w", err)
	}
	status.FolderExists = folderExists

	// 2. 讀取嵌入的 Dashboard JSON 檔案並逐個匯入
	entries, err := dashboardFS.ReadDir("dashboards")
	if err != nil {
		return nil, fmt.Errorf("讀取嵌入的 Dashboard 檔案失敗: %w", err)
	}

	status.AllSynced = true
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		data, err := dashboardFS.ReadFile("dashboards/" + entry.Name())
		if err != nil {
			logger.Error("讀取 Dashboard 檔案失敗", "file", entry.Name(), "error", err)
			status.AllSynced = false
			continue
		}

		// 解析 Dashboard JSON 獲取 UID 和 Title
		var dashboardJSON map[string]interface{}
		if err := json.Unmarshal(data, &dashboardJSON); err != nil {
			logger.Error("解析 Dashboard JSON 失敗", "file", entry.Name(), "error", err)
			status.AllSynced = false
			continue
		}

		uid, _ := dashboardJSON["uid"].(string)
		title, _ := dashboardJSON["title"].(string)

		// 匯入 Dashboard
		if err := s.importDashboard(dashboardJSON, "synapse-folder"); err != nil {
			logger.Error("匯入 Dashboard 失敗", "uid", uid, "title", title, "error", err)
			status.Dashboards = append(status.Dashboards, DashboardStatusItem{
				UID: uid, Title: title, Exists: false,
			})
			status.AllSynced = false
			continue
		}

		logger.Info("Dashboard 匯入成功", "uid", uid, "title", title)
		status.Dashboards = append(status.Dashboards, DashboardStatusItem{
			UID: uid, Title: title, Exists: true,
		})
	}

	return status, nil
}

// GetDashboardSyncStatus 獲取 Dashboard 同步狀態（只檢查不匯入）
func (s *GrafanaService) GetDashboardSyncStatus() (*DashboardSyncStatus, error) {
	if !s.IsEnabled() {
		return nil, fmt.Errorf("grafana 服務未配置")
	}

	status := &DashboardSyncStatus{
		AllSynced:  true,
		Dashboards: []DashboardStatusItem{},
	}

	// 檢查資料夾
	status.FolderExists = s.folderExists("synapse-folder")

	// 檢查每個 Dashboard
	entries, err := dashboardFS.ReadDir("dashboards")
	if err != nil {
		return nil, fmt.Errorf("讀取嵌入的 Dashboard 檔案失敗: %w", err)
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

// ensureFolder 確保 Grafana 資料夾存在（冪等）
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
		return false, fmt.Errorf("建立資料夾請求失敗: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// 200/412(已存在) 都算成功
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusPreconditionFailed {
		return true, nil
	}

	body, _ := io.ReadAll(resp.Body)
	return false, fmt.Errorf("建立資料夾失敗: status=%d, body=%s", resp.StatusCode, string(body))
}

// folderExists 檢查資料夾是否存在
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

// dashboardExists 檢查 Dashboard 是否存在
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

// importDashboard 匯入 Dashboard 到指定資料夾
func (s *GrafanaService) importDashboard(dashboardJSON map[string]interface{}, folderUID string) error {
	// 移除 id 欄位以確保新建或覆蓋
	delete(dashboardJSON, "id")

	reqBody, err := json.Marshal(map[string]interface{}{
		"dashboard": dashboardJSON,
		"folderUid": folderUID,
		"overwrite": true,
	})
	if err != nil {
		return fmt.Errorf("序列化請求失敗: %w", err)
	}

	apiURL := fmt.Sprintf("%s/api/dashboards/db", s.baseURL)
	req, err := http.NewRequest("POST", apiURL, bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("建立請求失敗: %w", err)
	}
	s.setHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("匯入請求失敗: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("匯入失敗: status=%d, body=%s", resp.StatusCode, string(body))
	}

	return nil
}
