package services

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/pkg/crypto"

	"gorm.io/gorm"
)

// sensitiveSettingKeys lists config_key values whose JSON Value blob is
// encrypted at rest. All three contain credentials (passwords / API keys /
// private keys) that must not appear in plain text in DB backups.
var sensitiveSettingKeys = map[string]bool{
	"ldap_config":    true, // LDAPConfig.BindPassword
	"ssh_config":     true, // SSHConfig.Password, SSHConfig.PrivateKey
	"grafana_config": true, // GrafanaSettingConfig.APIKey
}

// GetSystemSetting 從 system_settings 表讀取 JSON 配置並反序列化到 dest
func GetSystemSetting(db *gorm.DB, key string, dest interface{}) (bool, error) {
	var setting models.SystemSetting
	if err := db.Where("config_key = ?", key).First(&setting).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}

	value := setting.Value
	// Sensitive setting blobs are stored encrypted; decrypt before parsing.
	// Legacy plaintext values pass through crypto.Decrypt unchanged.
	if sensitiveSettingKeys[key] && crypto.IsEnabled() {
		decrypted, err := crypto.Decrypt(value)
		if err != nil {
			return false, fmt.Errorf("decrypt system setting %s: %w", key, err)
		}
		value = decrypted
	}

	if err := json.Unmarshal([]byte(value), dest); err != nil {
		return false, fmt.Errorf("解析配置 %s 失敗: %w", key, err)
	}
	return true, nil
}

// SaveSystemSetting 將配置序列化為 JSON 並儲存到 system_settings 表
func SaveSystemSetting(db *gorm.DB, key, settingType string, value interface{}) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("序列化配置 %s 失敗: %w", key, err)
	}

	jsonStr := string(data)
	// Encrypt the entire JSON blob for sensitive config types (P2-3).
	if sensitiveSettingKeys[key] && crypto.IsEnabled() {
		encrypted, err := crypto.Encrypt(jsonStr)
		if err != nil {
			return fmt.Errorf("encrypt system setting %s: %w", key, err)
		}
		jsonStr = encrypted
	}

	var setting models.SystemSetting
	result := db.Where("config_key = ?", key).First(&setting)

	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		setting = models.SystemSetting{
			ConfigKey: key,
			Value:     jsonStr,
			Type:      settingType,
		}
		return db.Create(&setting).Error
	} else if result.Error != nil {
		return result.Error
	}

	setting.Value = jsonStr
	return db.Save(&setting).Error
}
