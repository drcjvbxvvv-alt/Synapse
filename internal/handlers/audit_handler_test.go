package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/shaia/Synapse/internal/config"
	"github.com/shaia/Synapse/internal/services"
)

// AuditHandlerTestSuite 審計處理器測試套件
type AuditHandlerTestSuite struct {
	suite.Suite
	db      *gorm.DB
	mock    sqlmock.Sqlmock
	handler *AuditHandler
	router  *gin.Engine
}

func (s *AuditHandlerTestSuite) SetupTest() {
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

	cfg := &config.Config{}
	auditSvc := services.NewAuditService(gormDB)
	opLogSvc := services.NewOperationLogService(gormDB)
	s.handler = NewAuditHandler(cfg, auditSvc, opLogSvc)

	r := gin.New()
	r.GET("/api/audit/logs", s.handler.GetAuditLogs)
	r.GET("/api/audit/sessions", s.handler.GetTerminalSessions)
	r.GET("/api/audit/sessions/:sessionId", s.handler.GetTerminalSession)
	r.GET("/api/audit/sessions/:sessionId/commands", s.handler.GetTerminalCommands)
	r.GET("/api/audit/stats", s.handler.GetTerminalStats)
	s.router = r
}

func (s *AuditHandlerTestSuite) TearDownTest() {
	if s.db != nil {
		sqlDB, _ := s.db.DB()
		if sqlDB != nil {
			_ = sqlDB.Close()
		}
	}
}

// ─── GetAuditLogs ─────────────────────────────────────────────────────────────

func (s *AuditHandlerTestSuite) TestGetAuditLogs_Success() {
	// OperationLogService.List → COUNT + SELECT
	s.mock.ExpectQuery(`SELECT count`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	s.mock.ExpectQuery(`SELECT`).
		WillReturnRows(sqlmock.NewRows(opLogCols))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/audit/logs?page=1&pageSize=20", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusOK, w.Code)
}

func (s *AuditHandlerTestSuite) TestGetAuditLogs_DBError() {
	s.mock.ExpectQuery(`SELECT count`).
		WillReturnError(gorm.ErrInvalidDB)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/audit/logs", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusInternalServerError, w.Code)
}

func (s *AuditHandlerTestSuite) TestGetAuditLogs_WithFilters() {
	s.mock.ExpectQuery(`SELECT count`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	s.mock.ExpectQuery(`SELECT`).
		WillReturnRows(sqlmock.NewRows(opLogCols))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/audit/logs?username=alice&result=success&module=cluster", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusOK, w.Code)
}

// ─── GetTerminalSessions ──────────────────────────────────────────────────────

func (s *AuditHandlerTestSuite) TestGetTerminalSessions_Success() {
	// GetSessions → COUNT + SELECT with JOINs
	s.mock.ExpectQuery(`SELECT count`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	s.mock.ExpectQuery(`SELECT`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "cluster_id", "target_type", "target_ref",
			"namespace", "pod", "container", "node", "start_at", "end_at",
			"input_size", "status",
			"username", "display_name", "cluster_name", "command_count",
		}))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/audit/sessions", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusOK, w.Code)
}

func (s *AuditHandlerTestSuite) TestGetTerminalSessions_DBError() {
	s.mock.ExpectQuery(`SELECT count`).
		WillReturnError(gorm.ErrInvalidDB)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/audit/sessions", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusInternalServerError, w.Code)
}

// ─── GetTerminalSession ───────────────────────────────────────────────────────

func (s *AuditHandlerTestSuite) TestGetTerminalSession_InvalidID() {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/audit/sessions/notanumber", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusBadRequest, w.Code)
}

func (s *AuditHandlerTestSuite) TestGetTerminalSession_NotFound() {
	s.mock.ExpectQuery(`SELECT`).
		WillReturnError(gorm.ErrRecordNotFound)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/audit/sessions/99", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusInternalServerError, w.Code)
}

// ─── GetTerminalCommands ──────────────────────────────────────────────────────

func (s *AuditHandlerTestSuite) TestGetTerminalCommands_InvalidID() {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/audit/sessions/badid/commands", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusBadRequest, w.Code)
}

func (s *AuditHandlerTestSuite) TestGetTerminalCommands_Success() {
	// GetSessionCommands → COUNT + SELECT
	s.mock.ExpectQuery(`SELECT count`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	s.mock.ExpectQuery(`SELECT`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "session_id", "timestamp", "raw_input", "parsed_cmd", "exit_code",
		}))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/audit/sessions/1/commands", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusOK, w.Code)
}

// ─── GetTerminalStats ─────────────────────────────────────────────────────────

func (s *AuditHandlerTestSuite) TestGetTerminalStats_Success() {
	// GetSessionStats → 6 COUNT queries
	for i := 0; i < 6; i++ {
		s.mock.ExpectQuery(`SELECT count`).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/audit/stats", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusOK, w.Code)

	var body map[string]interface{}
	s.Require().NoError(json.Unmarshal(w.Body.Bytes(), &body))
	// stats should have numeric fields
	assert.Contains(s.T(), body, "totalSessions")
}

func (s *AuditHandlerTestSuite) TestGetTerminalStats_DBError() {
	s.mock.ExpectQuery(`SELECT count`).
		WillReturnError(gorm.ErrInvalidDB)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/audit/stats", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusInternalServerError, w.Code)
}

// ─── Suite runner ────────────────────────────────────────────────────────────

func TestAuditHandlerSuite(t *testing.T) {
	suite.Run(t, new(AuditHandlerTestSuite))
}
