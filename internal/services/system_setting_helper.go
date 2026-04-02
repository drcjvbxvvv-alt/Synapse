package services

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/clay-wangzhi/Synapse/internal/models"

	"gorm.io/gorm"
)

// GetSystemSetting 从 system_settings 表读取 JSON 配置并反序列化到 dest
func GetSystemSetting(db *gorm.DB, key string, dest interface{}) (bool, error) {
	var setting models.SystemSetting
	if err := db.Where("config_key = ?", key).First(&setting).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}

	if err := json.Unmarshal([]byte(setting.Value), dest); err != nil {
		return false, fmt.Errorf("解析配置 %s 失败: %w", key, err)
	}
	return true, nil
}

// SaveSystemSetting 将配置序列化为 JSON 并保存到 system_settings 表
func SaveSystemSetting(db *gorm.DB, key, settingType string, value interface{}) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("序列化配置 %s 失败: %w", key, err)
	}

	var setting models.SystemSetting
	result := db.Where("config_key = ?", key).First(&setting)

	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		setting = models.SystemSetting{
			ConfigKey: key,
			Value:     string(data),
			Type:      settingType,
		}
		return db.Create(&setting).Error
	} else if result.Error != nil {
		return result.Error
	}

	setting.Value = string(data)
	return db.Save(&setting).Error
}
