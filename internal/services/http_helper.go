package services

import (
	"fmt"
	"net/http"

	"github.com/shaia/Synapse/internal/models"
)

// SetMonitoringAuth 為 HTTP 請求設定監控系統認證（Prometheus/AlertManager 共用）
func SetMonitoringAuth(req *http.Request, auth *models.MonitoringAuth) error {
	if auth == nil {
		return nil
	}

	switch auth.Type {
	case "none", "":
		return nil
	case "basic":
		req.SetBasicAuth(auth.Username, auth.Password)
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+auth.Token)
	default:
		return fmt.Errorf("不支援的認證型別: %s", auth.Type)
	}

	return nil
}
