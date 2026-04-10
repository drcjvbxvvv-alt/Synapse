package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/shaia/Synapse/internal/services"
)

// OperationLogHandlerTestSuite 操作日誌處理器測試套件
type OperationLogHandlerTestSuite struct {
	suite.Suite
	db      *gorm.DB
	mock    sqlmock.Sqlmock
	handler *OperationLogHandler
	router  *gin.Engine
}

// opLogCols are the columns for OperationLog sqlmock rows.
var opLogCols = []string{
	"id", "user_id", "username", "method", "path", "query",
	"module", "action", "cluster_id", "cluster_name", "namespace",
	"resource_type", "resource_name", "request_body", "status_code",
	"success", "error_message", "client_ip", "user_agent", "duration", "created_at",
}

func (s *OperationLogHandlerTestSuite) SetupTest() {
	gin.SetMode(gin.TestMode)

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

	opLogSvc := services.NewOperationLogService(gormDB)
	s.handler = NewOperationLogHandler(opLogSvc)

	r := gin.New()
	r.GET("/api/operation-logs", s.handler.GetOperationLogs)
	r.GET("/api/operation-logs/:id", s.handler.GetOperationLog)
	r.GET("/api/operation-logs/modules", s.handler.GetModules)
	r.GET("/api/operation-logs/actions", s.handler.GetActions)
	s.router = r
}

func (s *OperationLogHandlerTestSuite) TearDownTest() {
	if s.db != nil {
		sqlDB, _ := s.db.DB()
		if sqlDB != nil {
			_ = sqlDB.Close()
		}
	}
}

// ─── GetOperationLogs ──────────────────────────────────���──────────────────────

func (s *OperationLogHandlerTestSuite) TestGetOperationLogs_Success() {
	now := time.Now()

	s.mock.ExpectQuery(`SELECT count`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	s.mock.ExpectQuery(`SELECT`).
		WillReturnRows(sqlmock.NewRows(opLogCols).AddRow(
			1, 1, "admin", "POST", "/api/clusters", "",
			"cluster", "create", nil, "", "",
			"", "", "", 201, true, "", "127.0.0.1", "curl/7.79", 10, now,
		))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/operation-logs?page=1&pageSize=20", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusOK, w.Code)

	var body map[string]interface{}
	s.Require().NoError(json.Unmarshal(w.Body.Bytes(), &body))
	assert.EqualValues(s.T(), 1, body["total"])
}

func (s *OperationLogHandlerTestSuite) TestGetOperationLogs_WithFilters() {
	s.mock.ExpectQuery(`SELECT count`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	s.mock.ExpectQuery(`SELECT`).
		WillReturnRows(sqlmock.NewRows(opLogCols))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/operation-logs?username=alice&module=cluster&success=true", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusOK, w.Code)

	var body map[string]interface{}
	s.Require().NoError(json.Unmarshal(w.Body.Bytes(), &body))
	assert.EqualValues(s.T(), 0, body["total"])
}

func (s *OperationLogHandlerTestSuite) TestGetOperationLogs_DBError() {
	s.mock.ExpectQuery(`SELECT count`).
		WillReturnError(gorm.ErrInvalidDB)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/operation-logs", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusInternalServerError, w.Code)
}

// ─── GetOperationLog ───────────────────────────────��──────────────────────────

func (s *OperationLogHandlerTestSuite) TestGetOperationLog_InvalidID() {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/operation-logs/notanumber", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusBadRequest, w.Code)
}

func (s *OperationLogHandlerTestSuite) TestGetOperationLog_NotFound() {
	s.mock.ExpectQuery(`SELECT`).
		WillReturnError(gorm.ErrRecordNotFound)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/operation-logs/999", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusNotFound, w.Code)
}

func (s *OperationLogHandlerTestSuite) TestGetOperationLog_Success() {
	now := time.Now()

	s.mock.ExpectQuery(`SELECT`).
		WillReturnRows(sqlmock.NewRows(opLogCols).AddRow(
			42, 1, "admin", "DELETE", "/api/clusters/1/pods/default/nginx", "",
			"pod", "delete", 1, "prod", "default",
			"pod", "nginx", "", 200, true, "", "192.168.1.1", "Mozilla/5", 5, now,
		))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/operation-logs/42", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusOK, w.Code)
}

// ─── GetModules ─────────────────────────��────────────────────────────────���────

func (s *OperationLogHandlerTestSuite) TestGetModules_Success() {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/operation-logs/modules", nil)
	s.router.ServeHTTP(w, req)

	// GetModules reads from constants — no DB involved
	assert.Equal(s.T(), http.StatusOK, w.Code)

	var body []interface{}
	s.Require().NoError(json.Unmarshal(w.Body.Bytes(), &body))
	assert.NotEmpty(s.T(), body)
}

// ─── GetActions ───────────────────────────────────────────────────────────────

func (s *OperationLogHandlerTestSuite) TestGetActions_Success() {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/operation-logs/actions", nil)
	s.router.ServeHTTP(w, req)

	// GetActions reads from constants — no DB involved
	assert.Equal(s.T(), http.StatusOK, w.Code)

	var body []interface{}
	s.Require().NoError(json.Unmarshal(w.Body.Bytes(), &body))
	assert.NotEmpty(s.T(), body)
}

// ─── Suite runner ───────────────────────────────��────────────────────────────

func TestOperationLogHandlerSuite(t *testing.T) {
	suite.Run(t, new(OperationLogHandlerTestSuite))
}
