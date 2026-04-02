package services

import (
	"fmt"
	"net/http"

	"github.com/clay-wangzhi/Synapse/internal/models"
)

// SetMonitoringAuth 为 HTTP 请求设置监控系统认证（Prometheus/AlertManager 共用）
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
		return fmt.Errorf("不支持的认证类型: %s", auth.Type)
	}

	return nil
}
