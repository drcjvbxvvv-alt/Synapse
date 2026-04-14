package services

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/pkg/logger"
)

// TokenBlacklistService JWT 黑名單服務
// 為中介軟體提供 O(1) 撤銷檢查：
//   - 寫入：Logout / 密碼變更 / 強制登出
//   - 查詢：每次帶 token 的請求
//
// 為避免每次請求都打 DB，加入一層 sync.Map 本地快取，
// 寫入時同時更新快取；快取項在 token 過期後自動淘汰。
type TokenBlacklistService struct {
	db    *gorm.DB
	cache sync.Map // key: jti (string), value: expiresAt (time.Time)
}

// NewTokenBlacklistService 建立黑名單服務
func NewTokenBlacklistService(db *gorm.DB) *TokenBlacklistService {
	svc := &TokenBlacklistService{db: db}
	svc.warmCache()
	return svc
}

// warmCache 啟動時從 DB 載入未過期的黑名單到本地快取，
// 避免服務重啟後黑名單失效的時間窗
func (s *TokenBlacklistService) warmCache() {
	var items []models.TokenBlacklist
	now := time.Now()
	if err := s.db.Where("expires_at > ?", now).Find(&items).Error; err != nil {
		logger.Warn("載入 token 黑名單快取失敗", "error", err)
		return
	}
	for _, item := range items {
		s.cache.Store(item.JTI, item.ExpiresAt)
	}
	if len(items) > 0 {
		logger.Info("token 黑名單快取已載入", "count", len(items))
	}
}

// Revoke 將 token 加入黑名單
func (s *TokenBlacklistService) Revoke(ctx context.Context, jti string, userID uint, expiresAt time.Time, reason string) error {
	if jti == "" {
		return fmt.Errorf("revoke token: jti 不能為空")
	}
	// 已過期的 token 不需要加入黑名單（保留也沒用，且會佔空間）
	if expiresAt.Before(time.Now()) {
		return nil
	}

	entry := &models.TokenBlacklist{
		JTI:       jti,
		UserID:    userID,
		Reason:    reason,
		ExpiresAt: expiresAt,
	}
	// 使用 ON CONFLICT DO NOTHING 語意（ErrDuplicatedKey 視為成功）
	if err := s.db.WithContext(ctx).Create(entry).Error; err != nil {
		// 重複插入不是錯誤，代表同一個 token 被多次登出
		if !isDuplicateKeyError(err) {
			return fmt.Errorf("寫入 token 黑名單: %w", err)
		}
	}

	s.cache.Store(jti, expiresAt)
	return nil
}

// IsRevoked 檢查 token 是否已被撤銷
// 呼叫頻率高（每次認證請求），優先查本地快取
func (s *TokenBlacklistService) IsRevoked(jti string) bool {
	if jti == "" {
		return false
	}
	if v, ok := s.cache.Load(jti); ok {
		expiresAt, _ := v.(time.Time)
		// 快取中的條目若已過期，清理並視為未撤銷（token 本身也應該已過期，由 JWT exp 驗證攔截）
		if expiresAt.Before(time.Now()) {
			s.cache.Delete(jti)
			return false
		}
		return true
	}
	return false
}

// CleanupExpired 清理已過期的黑名單條目（由背景 worker 定期呼叫）
// 回傳清理的筆數
func (s *TokenBlacklistService) CleanupExpired(ctx context.Context) (int64, error) {
	now := time.Now()
	result := s.db.WithContext(ctx).
		Where("expires_at < ?", now).
		Delete(&models.TokenBlacklist{})
	if result.Error != nil {
		return 0, fmt.Errorf("清理過期黑名單: %w", result.Error)
	}

	// 同步清理本地快取
	s.cache.Range(func(key, value any) bool {
		if expiresAt, ok := value.(time.Time); ok && expiresAt.Before(now) {
			s.cache.Delete(key)
		}
		return true
	})

	return result.RowsAffected, nil
}

// isDuplicateKeyError 判斷是否為 GORM 的唯一鍵重複錯誤
// 不同驅動錯誤訊息不同，統一用字串檢查
func isDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "UNIQUE constraint failed") ||
		strings.Contains(msg, "Duplicate entry") ||
		strings.Contains(msg, "duplicated key")
}
