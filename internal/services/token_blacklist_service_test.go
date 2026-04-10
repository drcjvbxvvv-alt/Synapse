package services

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TokenBlacklistServiceTestSuite JWT 黑名單服務測試套件
type TokenBlacklistServiceTestSuite struct {
	suite.Suite
	db      *gorm.DB
	mock    sqlmock.Sqlmock
	service *TokenBlacklistService
}

func (s *TokenBlacklistServiceTestSuite) SetupTest() {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	s.Require().NoError(err)

	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      db,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	s.Require().NoError(err)

	s.db = gormDB
	s.mock = mock

	// warmCache 會在建構時呼叫 — 預期一次 SELECT
	mock.ExpectQuery(`SELECT`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "jti", "user_id", "reason", "expires_at", "created_at"}))

	s.service = NewTokenBlacklistService(gormDB)
}

func (s *TokenBlacklistServiceTestSuite) TearDownTest() {
	if sqlDB, _ := s.db.DB(); sqlDB != nil {
		_ = sqlDB.Close()
	}
}

// TestRevoke_Success 測試撤銷 token 成功
func (s *TokenBlacklistServiceTestSuite) TestRevoke_Success() {
	jti := "test-jti-001"
	expiresAt := time.Now().Add(1 * time.Hour)

	s.mock.ExpectBegin()
	s.mock.ExpectExec(`INSERT INTO .token_blacklists.`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.mock.ExpectCommit()

	err := s.service.Revoke(context.Background(), jti, 1, expiresAt, "logout")
	assert.NoError(s.T(), err)

	// 撤銷後快取中應已存在
	assert.True(s.T(), s.service.IsRevoked(jti))
}

// TestRevoke_EmptyJTI 測試空 JTI 回傳錯誤
func (s *TokenBlacklistServiceTestSuite) TestRevoke_EmptyJTI() {
	err := s.service.Revoke(context.Background(), "", 1, time.Now().Add(1*time.Hour), "logout")
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "jti 不能為空")
}

// TestRevoke_AlreadyExpired 已過期的 token 不應寫入 DB
func (s *TokenBlacklistServiceTestSuite) TestRevoke_AlreadyExpired() {
	// expiresAt 在過去 — 直接 return nil，不呼叫 DB
	err := s.service.Revoke(context.Background(), "expired-jti", 1, time.Now().Add(-1*time.Hour), "logout")
	assert.NoError(s.T(), err)
	// mock 不應收到任何呼叫
	assert.NoError(s.T(), s.mock.ExpectationsWereMet())
}

// TestRevoke_DuplicateKey 重複撤銷應視為成功（不報錯）
func (s *TokenBlacklistServiceTestSuite) TestRevoke_DuplicateKey() {
	jti := "dup-jti"
	expiresAt := time.Now().Add(1 * time.Hour)

	// 模擬 UNIQUE constraint 錯誤
	s.mock.ExpectBegin()
	s.mock.ExpectExec(`INSERT INTO .token_blacklists.`).
		WillReturnError(gorm.ErrDuplicatedKey)
	s.mock.ExpectRollback()

	err := s.service.Revoke(context.Background(), jti, 1, expiresAt, "logout")
	// isDuplicateKeyError 攔截 → 不回傳錯誤
	// gorm.ErrDuplicatedKey 訊息包含 "duplicated key"
	assert.NoError(s.T(), err)
}

// TestIsRevoked_CacheHit 快取命中返回 true
func (s *TokenBlacklistServiceTestSuite) TestIsRevoked_CacheHit() {
	jti := "cached-jti"
	expiresAt := time.Now().Add(2 * time.Hour)
	s.service.cache.Store(jti, expiresAt)

	assert.True(s.T(), s.service.IsRevoked(jti))
}

// TestIsRevoked_CacheMiss 快取不存在返回 false（不查 DB，直接 false）
func (s *TokenBlacklistServiceTestSuite) TestIsRevoked_CacheMiss() {
	assert.False(s.T(), s.service.IsRevoked("nonexistent-jti"))
}

// TestIsRevoked_EmptyJTI 空 JTI 返回 false
func (s *TokenBlacklistServiceTestSuite) TestIsRevoked_EmptyJTI() {
	assert.False(s.T(), s.service.IsRevoked(""))
}

// TestIsRevoked_ExpiredCacheEntry 快取中過期條目應清理並返回 false
func (s *TokenBlacklistServiceTestSuite) TestIsRevoked_ExpiredCacheEntry() {
	jti := "expired-cached-jti"
	// 存入已過期的時間
	s.service.cache.Store(jti, time.Now().Add(-1*time.Hour))

	assert.False(s.T(), s.service.IsRevoked(jti))

	// 確認快取條目已被刪除
	_, ok := s.service.cache.Load(jti)
	assert.False(s.T(), ok)
}

// TestCleanupExpired_Success 測試清理過期黑名單成功
func (s *TokenBlacklistServiceTestSuite) TestCleanupExpired_Success() {
	s.mock.ExpectBegin()
	s.mock.ExpectExec(`DELETE FROM .token_blacklists.`).
		WillReturnResult(sqlmock.NewResult(0, 3))
	s.mock.ExpectCommit()

	count, err := s.service.CleanupExpired(context.Background())
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), int64(3), count)
}

// TestCleanupExpired_DBError 測試清理失敗
func (s *TokenBlacklistServiceTestSuite) TestCleanupExpired_DBError() {
	s.mock.ExpectBegin()
	s.mock.ExpectExec(`DELETE FROM .token_blacklists.`).
		WillReturnError(gorm.ErrInvalidDB)
	s.mock.ExpectRollback()

	count, err := s.service.CleanupExpired(context.Background())
	assert.Error(s.T(), err)
	assert.Equal(s.T(), int64(0), count)
	assert.Contains(s.T(), err.Error(), "清理過期黑名單")
}

// TestCleanupExpired_AlsoClearsCache 測試清理同時清除過期快取
func (s *TokenBlacklistServiceTestSuite) TestCleanupExpired_AlsoClearsCache() {
	// 插入一條已過期的快取條目
	s.service.cache.Store("old-jti", time.Now().Add(-2*time.Hour))

	s.mock.ExpectBegin()
	s.mock.ExpectExec(`DELETE FROM .token_blacklists.`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	s.mock.ExpectCommit()

	_, err := s.service.CleanupExpired(context.Background())
	assert.NoError(s.T(), err)

	// 快取中的過期條目應已被清理
	_, ok := s.service.cache.Load("old-jti")
	assert.False(s.T(), ok)
}

// TestIsDuplicateKeyError 測試重複鍵錯誤識別
func (s *TokenBlacklistServiceTestSuite) TestIsDuplicateKeyError() {
	assert.False(s.T(), isDuplicateKeyError(nil))
	assert.True(s.T(), isDuplicateKeyError(gorm.ErrDuplicatedKey))
}

// TestTokenBlacklistServiceSuite 執行測試套件
func TestTokenBlacklistServiceSuite(t *testing.T) {
	suite.Run(t, new(TokenBlacklistServiceTestSuite))
}
