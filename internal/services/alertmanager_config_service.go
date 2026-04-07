package services

import (
	"encoding/json"
	"fmt"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/pkg/logger"

	"gorm.io/gorm"
)

// AlertManagerConfigService Alertmanager 配置服務
type AlertManagerConfigService struct {
	db *gorm.DB
}

// NewAlertManagerConfigService 建立 Alertmanager 配置服務
func NewAlertManagerConfigService(db *gorm.DB) *AlertManagerConfigService {
	return &AlertManagerConfigService{db: db}
}

// GetAlertManagerConfig 獲取叢集 Alertmanager 配置
func (s *AlertManagerConfigService) GetAlertManagerConfig(clusterID uint) (*models.AlertManagerConfig, error) {
	var cluster models.Cluster
	if err := s.db.Select("alert_manager_config").First(&cluster, clusterID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("叢集不存在: %d", clusterID)
		}
		return nil, fmt.Errorf("獲取叢集失敗: %w", err)
	}

	if cluster.AlertManagerConfig == "" {
		// 返回預設配置（禁用）
		return &models.AlertManagerConfig{
			Enabled: false,
		}, nil
	}

	var config models.AlertManagerConfig
	if err := json.Unmarshal([]byte(cluster.AlertManagerConfig), &config); err != nil {
		logger.Error("解析 Alertmanager 配置失敗", "cluster_id", clusterID, "error", err)
		return &models.AlertManagerConfig{
			Enabled: false,
		}, nil
	}

	return &config, nil
}

// UpdateAlertManagerConfig 更新叢集 Alertmanager 配置
func (s *AlertManagerConfigService) UpdateAlertManagerConfig(clusterID uint, config *models.AlertManagerConfig) error {
	// 驗證配置
	if err := s.validateConfig(config); err != nil {
		return fmt.Errorf("配置驗證失敗: %w", err)
	}

	// 序列化配置
	configJSON, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("序列化配置失敗: %w", err)
	}

	// 更新資料庫
	result := s.db.Model(&models.Cluster{}).Where("id = ?", clusterID).Update("alert_manager_config", string(configJSON))
	if result.Error != nil {
		return fmt.Errorf("更新 Alertmanager 配置失敗: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("叢集不存在: %d", clusterID)
	}

	logger.Info("Alertmanager 配置更新成功", "cluster_id", clusterID, "enabled", config.Enabled)
	return nil
}

// DeleteAlertManagerConfig 刪除叢集 Alertmanager 配置
func (s *AlertManagerConfigService) DeleteAlertManagerConfig(clusterID uint) error {
	result := s.db.Model(&models.Cluster{}).Where("id = ?", clusterID).Update("alert_manager_config", "")
	if result.Error != nil {
		return fmt.Errorf("刪除 Alertmanager 配置失敗: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("叢集不存在: %d", clusterID)
	}

	logger.Info("Alertmanager 配置刪除成功", "cluster_id", clusterID)
	return nil
}

// validateConfig 驗證 Alertmanager 配置
func (s *AlertManagerConfigService) validateConfig(config *models.AlertManagerConfig) error {
	if !config.Enabled {
		return nil // 禁用狀態不需要驗證
	}

	if config.Endpoint == "" {
		return fmt.Errorf("alertmanager 端點地址不能為空")
	}

	// 驗證認證配置
	if config.Auth != nil {
		switch config.Auth.Type {
		case "none", "":
			// 無需認證，不需要驗證額外欄位
		case "basic":
			if config.Auth.Username == "" || config.Auth.Password == "" {
				return fmt.Errorf("basic 認證需要使用者名稱和密碼")
			}
		case "bearer":
			if config.Auth.Token == "" {
				return fmt.Errorf("bearer 認證需要 token")
			}
		default:
			return fmt.Errorf("不支援的認證型別: %s", config.Auth.Type)
		}
	}

	return nil
}

// GetDefaultConfig 獲取預設 Alertmanager 配置
func (s *AlertManagerConfigService) GetDefaultConfig() *models.AlertManagerConfig {
	return &models.AlertManagerConfig{
		Enabled: false,
	}
}

// GetAlertManagerConfigTemplate 獲取 Alertmanager 配置模板
func (s *AlertManagerConfigService) GetAlertManagerConfigTemplate() *models.AlertManagerConfig {
	return &models.AlertManagerConfig{
		Enabled:  true,
		Endpoint: "http://alertmanager:9093",
		Auth: &models.MonitoringAuth{
			Type: "none",
		},
	}
}
