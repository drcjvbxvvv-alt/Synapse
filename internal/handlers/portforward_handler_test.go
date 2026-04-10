package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

// PortForwardHandlerTestSuite Port-Forward 處理器測試套件
// Only tests methods that do NOT require a live K8s client:
//   - ListPortForwards
//   - StopPortForward
type PortForwardHandlerTestSuite struct {
	suite.Suite
	db      *gorm.DB
	mock    sqlmock.Sqlmock
	handler *PortForwardHandler
	router  *gin.Engine
}

// pfSessionCols are the columns for PortForwardSession sqlmock rows.
// gorm.Model → id, created_at, updated_at, deleted_at
var pfSessionCols = []string{
	"id", "created_at", "updated_at", "deleted_at",
	"cluster_id", "cluster_name", "namespace", "pod_name",
	"pod_port", "local_port", "user_id", "username",
	"status", "stopped_at",
}

func (s *PortForwardHandlerTestSuite) SetupTest() {
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

	pfSvc := services.NewPortForwardService(gormDB)
	// clusterSvc and k8sMgr are only needed by StartPortForward which we skip.
	clusterSvc := services.NewClusterService(gormDB, nil)
	s.handler = NewPortForwardHandler(pfSvc, clusterSvc, nil)

	r := gin.New()

	// Inject user_id=1 for all requests in this test suite.
	r.Use(func(c *gin.Context) {
		c.Set("user_id", uint(1))
		c.Next()
	})

	r.GET("/api/portforwards", s.handler.ListPortForwards)
	r.DELETE("/api/portforwards/:sessionId", s.handler.StopPortForward)
	s.router = r
}

func (s *PortForwardHandlerTestSuite) TearDownTest() {
	if s.db != nil {
		sqlDB, _ := s.db.DB()
		if sqlDB != nil {
			_ = sqlDB.Close()
		}
	}
}

// ─── ListPortForwards ─────────────────────────────────────────────────────────

func (s *PortForwardHandlerTestSuite) TestListPortForwards_Success() {
	now := time.Now()

	s.mock.ExpectQuery(`SELECT .* FROM .port_forward_sessions.`).
		WillReturnRows(sqlmock.NewRows(pfSessionCols).AddRow(
			1, now, now, nil,
			2, "prod-cluster", "default", "nginx-abc",
			8080, 12345, 1, "alice",
			"active", nil,
		))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/portforwards", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusOK, w.Code)

	var body map[string]interface{}
	s.Require().NoError(json.Unmarshal(w.Body.Bytes(), &body))
	items, ok := body["items"].([]interface{})
	s.Require().True(ok)
	assert.Len(s.T(), items, 1)
	assert.EqualValues(s.T(), 1, body["total"])
}

func (s *PortForwardHandlerTestSuite) TestListPortForwards_Empty() {
	s.mock.ExpectQuery(`SELECT .* FROM .port_forward_sessions.`).
		WillReturnRows(sqlmock.NewRows(pfSessionCols))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/portforwards?status=active", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusOK, w.Code)

	var body map[string]interface{}
	s.Require().NoError(json.Unmarshal(w.Body.Bytes(), &body))
	assert.EqualValues(s.T(), 0, body["total"])
}

func (s *PortForwardHandlerTestSuite) TestListPortForwards_DBError() {
	s.mock.ExpectQuery(`SELECT .* FROM .port_forward_sessions.`).
		WillReturnError(gorm.ErrInvalidDB)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/portforwards", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusInternalServerError, w.Code)
}

// ─── StopPortForward ──────────────────────────────────────────────────────────

func (s *PortForwardHandlerTestSuite) TestStopPortForward_Success() {
	// StopSession UPDATE
	s.mock.ExpectBegin()
	s.mock.ExpectExec(`UPDATE .port_forward_sessions.`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.mock.ExpectCommit()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/portforwards/1", strings.NewReader(""))
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusOK, w.Code)

	var body map[string]interface{}
	s.Require().NoError(json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(s.T(), "Port-Forward 已停止", body["message"])
}

func (s *PortForwardHandlerTestSuite) TestStopPortForward_InvalidSessionID() {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/portforwards/notanumber", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusBadRequest, w.Code)
}

func (s *PortForwardHandlerTestSuite) TestStopPortForward_DBError() {
	// StopSession UPDATE fails
	s.mock.ExpectBegin()
	s.mock.ExpectExec(`UPDATE .port_forward_sessions.`).
		WillReturnError(gorm.ErrInvalidDB)
	s.mock.ExpectRollback()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/portforwards/5", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusInternalServerError, w.Code)
}

// ─── Suite runner ────────────────────────────────────────────────────────────

func TestPortForwardHandlerSuite(t *testing.T) {
	suite.Run(t, new(PortForwardHandlerTestSuite))
}
