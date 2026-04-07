package services

import (
	"encoding/json"
	"fmt"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/pkg/logger"

	"gorm.io/gorm"
)

// MonitoringConfigService 監控配置服務
type MonitoringConfigService struct {
	db             *gorm.DB
	grafanaService *GrafanaService
}

// NewMonitoringConfigService 建立監控配置服務
func NewMonitoringConfigService(db *gorm.DB) *MonitoringConfigService {
	return &MonitoringConfigService{db: db}
}

// NewMonitoringConfigServiceWithGrafana 建立帶 Grafana 同步功能的監控配置服務
func NewMonitoringConfigServiceWithGrafana(db *gorm.DB, grafanaService *GrafanaService) *MonitoringConfigService {
	return &MonitoringConfigService{
		db:             db,
		grafanaService: grafanaService,
	}
}

// GetMonitoringConfig 獲取叢集監控配置
func (s *MonitoringConfigService) GetMonitoringConfig(clusterID uint) (*models.MonitoringConfig, error) {
	var cluster models.Cluster
	if err := s.db.Select("monitoring_config").First(&cluster, clusterID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("叢集不存在: %d", clusterID)
		}
		return nil, fmt.Errorf("獲取叢集失敗: %w", err)
	}

	if cluster.MonitoringConfig == "" {
		// 返回預設配置（禁用監控）
		return &models.MonitoringConfig{
			Type: "disabled",
		}, nil
	}

	var config models.MonitoringConfig
	if err := json.Unmarshal([]byte(cluster.MonitoringConfig), &config); err != nil {
		logger.Error("解析監控配置失敗", "cluster_id", clusterID, "error", err)
		return &models.MonitoringConfig{
			Type: "disabled",
		}, nil
	}

	return &config, nil
}

// UpdateMonitoringConfig 更新叢集監控配置
func (s *MonitoringConfigService) UpdateMonitoringConfig(clusterID uint, config *models.MonitoringConfig) error {
	// 驗證配置
	if err := s.validateConfig(config); err != nil {
		return fmt.Errorf("配置驗證失敗: %w", err)
	}

	// 獲取叢集名稱（用於 Grafana 資料來源命名）
	var cluster models.Cluster
	if err := s.db.Select("name").First(&cluster, clusterID).Error; err != nil {
		return fmt.Errorf("獲取叢集資訊失敗: %w", err)
	}

	// 序列化配置
	configJSON, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("序列化配置失敗: %w", err)
	}

	// 更新資料庫
	result := s.db.Model(&models.Cluster{}).Where("id = ?", clusterID).Update("monitoring_config", string(configJSON))
	if result.Error != nil {
		return fmt.Errorf("更新監控配置失敗: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("叢集不存在: %d", clusterID)
	}

	// 同步 Grafana 資料來源
	if s.grafanaService != nil && s.grafanaService.IsEnabled() {
		if config.Type == "disabled" {
			// 監控禁用時刪除資料來源
			if err := s.grafanaService.DeleteDataSource(cluster.Name); err != nil {
				logger.Error("刪除 Grafana 資料來源失敗", "cluster", cluster.Name, "error", err)
				// 不返回錯誤，只記錄日誌
			}
		} else {
			// 同步資料來源
			if err := s.grafanaService.SyncDataSource(cluster.Name, config.Endpoint); err != nil {
				logger.Error("同步 Grafana 資料來源失敗", "cluster", cluster.Name, "error", err)
				// 不返回錯誤，只記錄日誌
			}
		}
	}

	logger.Info("監控配置更新成功", "cluster_id", clusterID, "type", config.Type)
	return nil
}

// DeleteMonitoringConfig 刪除叢集監控配置
func (s *MonitoringConfigService) DeleteMonitoringConfig(clusterID uint) error {
	result := s.db.Model(&models.Cluster{}).Where("id = ?", clusterID).Update("monitoring_config", "")
	if result.Error != nil {
		return fmt.Errorf("刪除監控配置失敗: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("叢集不存在: %d", clusterID)
	}

	logger.Info("監控配置刪除成功", "cluster_id", clusterID)
	return nil
}

// validateConfig 驗證監控配置
func (s *MonitoringConfigService) validateConfig(config *models.MonitoringConfig) error {
	if config.Type == "disabled" {
		return nil // 禁用監控不需要驗證
	}

	if config.Endpoint == "" {
		return fmt.Errorf("監控端點不能為空")
	}

	// 驗證認證配置
	if config.Auth != nil {
		switch config.Auth.Type {
		case "none":
			// 無需認證，不需要驗證額外欄位
		case "basic":
			if config.Auth.Username == "" || config.Auth.Password == "" {
				return fmt.Errorf("basic 認證需要使用者名稱和密碼")
			}
		case "bearer":
			if config.Auth.Token == "" {
				return fmt.Errorf("bearer 認證需要 Token")
			}
		case "mtls":
			if config.Auth.CertFile == "" || config.Auth.KeyFile == "" {
				return fmt.Errorf("mTLS 認證需要證書檔案和金鑰檔案")
			}
		default:
			return fmt.Errorf("不支援的認證型別: %s", config.Auth.Type)
		}
	}

	return nil
}

// GetDefaultConfig 獲取預設監控配置
func (s *MonitoringConfigService) GetDefaultConfig() *models.MonitoringConfig {
	return &models.MonitoringConfig{
		Type: "disabled",
	}
}

// GetPrometheusConfig 獲取 Prometheus 配置模板
func (s *MonitoringConfigService) GetPrometheusConfig() *models.MonitoringConfig {
	return &models.MonitoringConfig{
		Type:     "prometheus",
		Endpoint: "http://prometheus:9090",
		Auth: &models.MonitoringAuth{
			Type: "none",
		},
		Labels: map[string]string{
			"cluster": "",
		},
	}
}

// GetVictoriaMetricsConfig 獲取 VictoriaMetrics 配置模板
func (s *MonitoringConfigService) GetVictoriaMetricsConfig() *models.MonitoringConfig {
	return &models.MonitoringConfig{
		Type:     "victoriametrics",
		Endpoint: "http://victoriametrics:8428",
		Auth: &models.MonitoringAuth{
			Type: "none",
		},
		Labels: map[string]string{
			"cluster": "",
		},
	}
}
