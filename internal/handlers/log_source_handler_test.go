package handlers

import (
	"bytes"
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

// LogSourceHandlerTestSuite 日誌源處理器測試套件
type LogSourceHandlerTestSuite struct {
	suite.Suite
	db      *gorm.DB
	mock    sqlmock.Sqlmock
	handler *LogSourceHandler
	router  *gin.Engine
}

// logSourceCols are the columns used when building sqlmock rows for LogSourceConfig.
var logSourceCols = []string{
	"id", "cluster_id", "type", "name", "url",
	"username", "password", "api_key", "enabled",
	"created_at", "updated_at", "deleted_at",
}

func (s *LogSourceHandlerTestSuite) SetupTest() {
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

	logSourceSvc := services.NewLogSourceService(gormDB)
	s.handler = NewLogSourceHandler(logSourceSvc)

	r := gin.New()
	r.GET("/api/clusters/:clusterID/log-sources", s.handler.ListLogSources)
	r.POST("/api/clusters/:clusterID/log-sources", s.handler.CreateLogSource)
	r.PUT("/api/clusters/:clusterID/log-sources/:sourceId", s.handler.UpdateLogSource)
	r.DELETE("/api/clusters/:clusterID/log-sources/:sourceId", s.handler.DeleteLogSource)
	s.router = r
}

func (s *LogSourceHandlerTestSuite) TearDownTest() {
	if s.db != nil {
		sqlDB, _ := s.db.DB()
		if sqlDB != nil {
			_ = sqlDB.Close()
		}
	}
}

// ─── ListLogSources ───────────────────────────────────────────────────────────

func (s *LogSourceHandlerTestSuite) TestListLogSources_Success() {
	now := time.Now()

	s.mock.ExpectQuery(`SELECT .* FROM .log_source_configs.`).
		WillReturnRows(sqlmock.NewRows(logSourceCols).AddRow(
			1, 2, "loki", "prod-loki", "http://loki:3100",
			"admin", "secret", "apikey123", true,
			now, now, nil,
		))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/clusters/2/log-sources", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusOK, w.Code)

	var body []interface{}
	s.Require().NoError(json.Unmarshal(w.Body.Bytes(), &body))
	assert.Len(s.T(), body, 1)
}

func (s *LogSourceHandlerTestSuite) TestListLogSources_InvalidClusterID() {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/clusters/abc/log-sources", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusBadRequest, w.Code)
}

func (s *LogSourceHandlerTestSuite) TestListLogSources_Empty() {
	s.mock.ExpectQuery(`SELECT .* FROM .log_source_configs.`).
		WillReturnRows(sqlmock.NewRows(logSourceCols))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/clusters/1/log-sources", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusOK, w.Code)
}

// ─── CreateLogSource ──────────────────────────────────────────────────────────

func (s *LogSourceHandlerTestSuite) TestCreateLogSource_Success() {
	s.mock.ExpectBegin()
	s.mock.ExpectExec(`INSERT INTO .log_source_configs.`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.mock.ExpectCommit()

	body, _ := json.Marshal(map[string]interface{}{
		"type":    "loki",
		"name":    "my-loki",
		"url":     "http://loki:3100",
		"enabled": true,
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/clusters/1/log-sources", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusOK, w.Code)
}

func (s *LogSourceHandlerTestSuite) TestCreateLogSource_InvalidType() {
	body, _ := json.Marshal(map[string]interface{}{
		"type": "xml",
		"name": "bad-source",
		"url":  "http://example.com",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/clusters/1/log-sources", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusBadRequest, w.Code)
}

func (s *LogSourceHandlerTestSuite) TestCreateLogSource_MissingRequiredFields() {
	// Missing name and url (both binding:"required")
	body, _ := json.Marshal(map[string]interface{}{
		"type": "loki",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/clusters/1/log-sources", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusBadRequest, w.Code)
}

func (s *LogSourceHandlerTestSuite) TestCreateLogSource_InvalidJSON() {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/clusters/1/log-sources", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusBadRequest, w.Code)
}

func (s *LogSourceHandlerTestSuite) TestCreateLogSource_InvalidClusterID() {
	body, _ := json.Marshal(map[string]interface{}{
		"type": "loki",
		"name": "x",
		"url":  "http://example.com",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/clusters/bad/log-sources", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusBadRequest, w.Code)
}

// ─── UpdateLogSource ──────────────────────────────────────────────────────────

func (s *LogSourceHandlerTestSuite) TestUpdateLogSource_Success() {
	now := time.Now()

	// GetLogSource SELECT
	s.mock.ExpectQuery(`SELECT .* FROM .log_source_configs.`).
		WillReturnRows(sqlmock.NewRows(logSourceCols).AddRow(
			1, 1, "loki", "old-loki", "http://loki:3100",
			"", "", "", true,
			now, now, nil,
		))
	// UpdateLogSource UPDATE
	s.mock.ExpectBegin()
	s.mock.ExpectExec(`UPDATE .log_source_configs.`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.mock.ExpectCommit()

	body, _ := json.Marshal(map[string]interface{}{
		"name": "updated-loki",
		"url":  "http://loki-new:3100",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/clusters/1/log-sources/1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusOK, w.Code)
}

func (s *LogSourceHandlerTestSuite) TestUpdateLogSource_InvalidSourceID() {
	body, _ := json.Marshal(map[string]interface{}{"name": "x"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/clusters/1/log-sources/abc", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusBadRequest, w.Code)
}

func (s *LogSourceHandlerTestSuite) TestUpdateLogSource_NotFound() {
	s.mock.ExpectQuery(`SELECT .* FROM .log_source_configs.`).
		WillReturnError(gorm.ErrRecordNotFound)

	body, _ := json.Marshal(map[string]interface{}{"name": "x"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/clusters/1/log-sources/99", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusNotFound, w.Code)
}

func (s *LogSourceHandlerTestSuite) TestUpdateLogSource_ZeroSourceID() {
	// sourceId=0 is invalid (srcID <= 0)
	body, _ := json.Marshal(map[string]interface{}{"name": "x"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/clusters/1/log-sources/0", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusBadRequest, w.Code)
}

// ─── DeleteLogSource ──────────────────────────────────────────────────────────

func (s *LogSourceHandlerTestSuite) TestDeleteLogSource_Success() {
	s.mock.ExpectBegin()
	s.mock.ExpectExec(`UPDATE .log_source_configs.`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.mock.ExpectCommit()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/clusters/1/log-sources/1", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusOK, w.Code)

	var body map[string]interface{}
	s.Require().NoError(json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(s.T(), "刪除成功", body["message"])
}

func (s *LogSourceHandlerTestSuite) TestDeleteLogSource_InvalidSourceID() {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/clusters/1/log-sources/nope", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusBadRequest, w.Code)
}

func (s *LogSourceHandlerTestSuite) TestDeleteLogSource_InvalidClusterID() {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/clusters/bad/log-sources/1", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusBadRequest, w.Code)
}

func (s *LogSourceHandlerTestSuite) TestDeleteLogSource_ZeroSourceID() {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/clusters/1/log-sources/0", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusBadRequest, w.Code)
}

// ─── Suite runner ────────────────────────────────────────────────────────────

func TestLogSourceHandlerSuite(t *testing.T) {
	suite.Run(t, new(LogSourceHandlerTestSuite))
}
