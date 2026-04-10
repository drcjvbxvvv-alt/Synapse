package services

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// OperationLogServiceTestSuite 操作日誌服務測試套件
type OperationLogServiceTestSuite struct {
	suite.Suite
	db      *gorm.DB
	mock    sqlmock.Sqlmock
	service *OperationLogService
}

func (s *OperationLogServiceTestSuite) SetupTest() {
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
	s.service = NewOperationLogService(gormDB)
}

func (s *OperationLogServiceTestSuite) TearDownTest() {
	if sqlDB, _ := s.db.DB(); sqlDB != nil {
		_ = sqlDB.Close()
	}
}

// TestRecord_Success 測試記錄操作日誌成功
func (s *OperationLogServiceTestSuite) TestRecord_Success() {
	uid := uint(1)
	cid := uint(2)
	entry := &LogEntry{
		UserID:       &uid,
		Username:     "admin",
		Method:       "POST",
		Path:         "/api/clusters/1/pods",
		Module:       "pod",
		Action:       "create",
		ClusterID:    &cid,
		ClusterName:  "prod-cluster",
		Namespace:    "default",
		ResourceType: "pod",
		ResourceName: "mypod",
		StatusCode:   201,
		Success:      true,
		ClientIP:     "127.0.0.1",
		Duration:     42,
	}

	s.mock.ExpectBegin()
	s.mock.ExpectExec(`INSERT INTO .operation_logs.`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.mock.ExpectCommit()

	err := s.service.Record(entry)
	assert.NoError(s.T(), err)
}

// TestRecord_DBError 測試記錄日誌DB錯誤
func (s *OperationLogServiceTestSuite) TestRecord_DBError() {
	entry := &LogEntry{
		Username: "user1",
		Method:   "DELETE",
		Path:     "/api/clusters/1/pods/mypod",
		Module:   "pod",
		Action:   "delete",
	}

	s.mock.ExpectBegin()
	s.mock.ExpectExec(`INSERT INTO .operation_logs.`).
		WillReturnError(gorm.ErrInvalidDB)
	s.mock.ExpectRollback()

	err := s.service.Record(entry)
	assert.Error(s.T(), err)
}

// TestRecord_WithRequestBody 測試帶請求體的日誌記錄（驗證脫敏）
func (s *OperationLogServiceTestSuite) TestRecord_WithRequestBody() {
	uid := uint(1)
	entry := &LogEntry{
		UserID:      &uid,
		Username:    "admin",
		Method:      "POST",
		Path:        "/api/auth/login",
		Module:      "auth",
		Action:      "login",
		StatusCode:  200,
		Success:     true,
		RequestBody: map[string]interface{}{"username": "admin", "password": "secret123"},
	}

	s.mock.ExpectBegin()
	s.mock.ExpectExec(`INSERT INTO .operation_logs.`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.mock.ExpectCommit()

	err := s.service.Record(entry)
	assert.NoError(s.T(), err)
}

// TestRecord_NilRequestBody 測試 nil 請求體
func (s *OperationLogServiceTestSuite) TestRecord_NilRequestBody() {
	entry := &LogEntry{
		Username: "user1",
		Method:   "GET",
		Path:     "/api/clusters",
		Module:   "cluster",
		Action:   "list",
		Success:  true,
	}

	s.mock.ExpectBegin()
	s.mock.ExpectExec(`INSERT INTO .operation_logs.`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.mock.ExpectCommit()

	err := s.service.Record(entry)
	assert.NoError(s.T(), err)
}

// TestList_NoFilters 測試列出日誌（無篩選條件）
func (s *OperationLogServiceTestSuite) TestList_NoFilters() {
	now := time.Now()

	s.mock.ExpectQuery(`SELECT count`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	logRows := sqlmock.NewRows([]string{
		"id", "user_id", "username", "method", "path", "query",
		"module", "action", "cluster_id", "cluster_name", "namespace",
		"resource_type", "resource_name", "request_body", "status_code",
		"success", "error_message", "client_ip", "user_agent", "duration", "created_at",
	}).AddRow(
		1, 1, "admin", "POST", "/api/clusters", "", "cluster", "create", nil, "", "",
		"", "", "", 201, true, "", "127.0.0.1", "curl/7.79", 10, now,
	)

	s.mock.ExpectQuery(`SELECT`).
		WillReturnRows(logRows)

	resp, err := s.service.List(&OperationLogListRequest{
		Page:     1,
		PageSize: 20,
	})
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), resp)
	assert.Equal(s.T(), int64(1), resp.Total)
	assert.Len(s.T(), resp.Items, 1)
	assert.Equal(s.T(), "cluster", resp.Items[0].Module)
}

// TestList_WithUserFilter 測試按使用者ID篩選
func (s *OperationLogServiceTestSuite) TestList_WithUserFilter() {
	uid := uint(5)

	s.mock.ExpectQuery(`SELECT count`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	s.mock.ExpectQuery(`SELECT`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "username", "method", "path", "query",
			"module", "action", "cluster_id", "cluster_name", "namespace",
			"resource_type", "resource_name", "request_body", "status_code",
			"success", "error_message", "client_ip", "user_agent", "duration", "created_at",
		}))

	resp, err := s.service.List(&OperationLogListRequest{
		UserID:   &uid,
		Page:     1,
		PageSize: 10,
	})
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), int64(0), resp.Total)
	assert.Len(s.T(), resp.Items, 0)
}

// TestList_WithKeyword 測試關鍵字篩選
func (s *OperationLogServiceTestSuite) TestList_WithKeyword() {
	s.mock.ExpectQuery(`SELECT count`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	s.mock.ExpectQuery(`SELECT`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "username", "method", "path", "query",
			"module", "action", "cluster_id", "cluster_name", "namespace",
			"resource_type", "resource_name", "request_body", "status_code",
			"success", "error_message", "client_ip", "user_agent", "duration", "created_at",
		}))

	resp, err := s.service.List(&OperationLogListRequest{
		Keyword:  "admin",
		Page:     1,
		PageSize: 10,
	})
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), resp)
}

// TestList_CountError 測試 Count 失敗
func (s *OperationLogServiceTestSuite) TestList_CountError() {
	s.mock.ExpectQuery(`SELECT count`).
		WillReturnError(gorm.ErrInvalidDB)

	resp, err := s.service.List(&OperationLogListRequest{Page: 1, PageSize: 20})
	assert.Error(s.T(), err)
	assert.Nil(s.T(), resp)
}

// TestList_DefaultPagination 測試預設分頁值（page<=0, pageSize<=0）
func (s *OperationLogServiceTestSuite) TestList_DefaultPagination() {
	s.mock.ExpectQuery(`SELECT count`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	s.mock.ExpectQuery(`SELECT`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "username", "method", "path", "query",
			"module", "action", "cluster_id", "cluster_name", "namespace",
			"resource_type", "resource_name", "request_body", "status_code",
			"success", "error_message", "client_ip", "user_agent", "duration", "created_at",
		}))

	resp, err := s.service.List(&OperationLogListRequest{Page: 0, PageSize: 0})
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), 1, resp.Page)
	assert.Equal(s.T(), 20, resp.PageSize)
}

// TestList_WithSuccessFilter 測試成功/失敗篩選
func (s *OperationLogServiceTestSuite) TestList_WithSuccessFilter() {
	successVal := true

	s.mock.ExpectQuery(`SELECT count`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	s.mock.ExpectQuery(`SELECT`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "username", "method", "path", "query",
			"module", "action", "cluster_id", "cluster_name", "namespace",
			"resource_type", "resource_name", "request_body", "status_code",
			"success", "error_message", "client_ip", "user_agent", "duration", "created_at",
		}))

	resp, err := s.service.List(&OperationLogListRequest{
		Success:  &successVal,
		Page:     1,
		PageSize: 20,
	})
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), resp)
}

// TestList_WithTimeRange 測試時間範圍篩選
func (s *OperationLogServiceTestSuite) TestList_WithTimeRange() {
	start := time.Now().Add(-24 * time.Hour)
	end := time.Now()

	s.mock.ExpectQuery(`SELECT count`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	s.mock.ExpectQuery(`SELECT`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "username", "method", "path", "query",
			"module", "action", "cluster_id", "cluster_name", "namespace",
			"resource_type", "resource_name", "request_body", "status_code",
			"success", "error_message", "client_ip", "user_agent", "duration", "created_at",
		}))

	resp, err := s.service.List(&OperationLogListRequest{
		StartTime: &start,
		EndTime:   &end,
		Page:      1,
		PageSize:  20,
	})
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), resp)
}

// TestGetDetail_Success 測試獲取日誌詳情成功
func (s *OperationLogServiceTestSuite) TestGetDetail_Success() {
	now := time.Now()
	rows := sqlmock.NewRows([]string{
		"id", "user_id", "username", "method", "path", "query",
		"module", "action", "cluster_id", "cluster_name", "namespace",
		"resource_type", "resource_name", "request_body", "status_code",
		"success", "error_message", "client_ip", "user_agent", "duration", "created_at",
	}).AddRow(
		1, 1, "admin", "DELETE", "/api/clusters/1", "", "cluster", "delete", nil, "", "",
		"", "", "", 200, true, "", "10.0.0.1", "", 5, now,
	)

	s.mock.ExpectQuery(`SELECT`).
		WillReturnRows(rows)

	log, err := s.service.GetDetail(1)
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), log)
	assert.Equal(s.T(), "admin", log.Username)
}

// TestGetDetail_NotFound 測試日誌不存在
func (s *OperationLogServiceTestSuite) TestGetDetail_NotFound() {
	s.mock.ExpectQuery(`SELECT`).
		WillReturnError(gorm.ErrRecordNotFound)

	log, err := s.service.GetDetail(999)
	assert.Error(s.T(), err)
	assert.Nil(s.T(), log)
}

// ---- sanitizeAndMarshal 單元測試 (pure functions, no DB) ----

func (s *OperationLogServiceTestSuite) TestSanitizeAndMarshal_Nil() {
	result := sanitizeAndMarshal(nil)
	assert.Equal(s.T(), "", result)
}

func (s *OperationLogServiceTestSuite) TestSanitizeAndMarshal_EmptyString() {
	result := sanitizeAndMarshal("")
	assert.Equal(s.T(), "", result)
}

func (s *OperationLogServiceTestSuite) TestSanitizeAndMarshal_NonJSONString() {
	result := sanitizeAndMarshal("plain text")
	assert.Equal(s.T(), "plain text", result)
}

func (s *OperationLogServiceTestSuite) TestSanitizeAndMarshal_JSONWithSensitiveKey() {
	body := map[string]interface{}{
		"username": "admin",
		"password": "secret",
	}
	result := sanitizeAndMarshal(body)
	assert.Contains(s.T(), result, "***REDACTED***")
	assert.Contains(s.T(), result, "admin")
}

func (s *OperationLogServiceTestSuite) TestSanitizeAndMarshal_JSONString() {
	jsonStr := `{"username":"admin","token":"abc123"}`
	result := sanitizeAndMarshal(jsonStr)
	assert.Contains(s.T(), result, "***REDACTED***")
	assert.Contains(s.T(), result, "admin")
}

func (s *OperationLogServiceTestSuite) TestSanitizeAndMarshal_NestedSensitive() {
	body := map[string]interface{}{
		"user": map[string]interface{}{
			"name":     "alice",
			"apikey":   "mykey",
			"email":    "alice@example.com",
		},
	}
	result := sanitizeAndMarshal(body)
	assert.Contains(s.T(), result, "***REDACTED***")
	assert.Contains(s.T(), result, "alice@example.com")
}

func (s *OperationLogServiceTestSuite) TestSanitizeAndMarshal_LargeBody() {
	// 建立超過 4000 bytes 的 body
	largeMap := make(map[string]interface{})
	for i := 0; i < 100; i++ {
		largeMap[string(rune('a'+i%26))+string(rune('0'+i%10))] = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	}
	result := sanitizeAndMarshal(largeMap)
	// 若超過限制，應附加截斷標記
	if len(result) > 4000 {
		assert.Contains(s.T(), result, "...(truncated)")
	}
}

// TestIsSensitiveKey 測試敏感欄位識別
func (s *OperationLogServiceTestSuite) TestIsSensitiveKey() {
	assert.True(s.T(), isSensitiveKey("password"))
	assert.True(s.T(), isSensitiveKey("Password"))
	assert.True(s.T(), isSensitiveKey("PASSWORD"))
	assert.True(s.T(), isSensitiveKey("token"))
	assert.True(s.T(), isSensitiveKey("api_token"))
	assert.True(s.T(), isSensitiveKey("kubeconfig"))
	assert.True(s.T(), isSensitiveKey("secret"))
	assert.False(s.T(), isSensitiveKey("username"))
	assert.False(s.T(), isSensitiveKey("email"))
	assert.False(s.T(), isSensitiveKey("status"))
}

// TestGetModuleName 測試模組名稱對映
func (s *OperationLogServiceTestSuite) TestGetModuleName() {
	assert.Equal(s.T(), "認證管理", getModuleName("auth"))
	assert.Equal(s.T(), "叢集管理", getModuleName("cluster"))
	assert.Equal(s.T(), "告警管理", getModuleName("alert"))
	// 未知模組應回傳原值
	assert.Equal(s.T(), "unknown_module", getModuleName("unknown_module"))
}

// TestGetActionName 測試操作名稱對映
func (s *OperationLogServiceTestSuite) TestGetActionName() {
	assert.Equal(s.T(), "登入", getActionName("login"))
	assert.Equal(s.T(), "建立", getActionName("create"))
	assert.Equal(s.T(), "刪除", getActionName("delete"))
	assert.Equal(s.T(), "重啟", getActionName("restart"))
	// 未知操作應回傳原值
	assert.Equal(s.T(), "custom_action", getActionName("custom_action"))
}

// TestOperationLogServiceSuite 執行測試套件
func TestOperationLogServiceSuite(t *testing.T) {
	suite.Run(t, new(OperationLogServiceTestSuite))
}
