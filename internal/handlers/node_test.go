package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

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

// NodeHandlerTestSuite 定義節點處理器測試套件
type NodeHandlerTestSuite struct {
	suite.Suite
	db      *gorm.DB
	mock    sqlmock.Sqlmock
	router  *gin.Engine
	handler *NodeHandler
}

// SetupTest 每個測試前的設定
func (s *NodeHandlerTestSuite) SetupTest() {
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
	// nil repo → ClusterService falls back to the legacy *gorm.DB path.
	clusterService := services.NewClusterService(gormDB, nil)
	s.handler = NewNodeHandler(cfg, clusterService, nil, nil, nil)

	s.router = gin.New()
	s.router.GET("/api/clusters/:clusterID/nodes", s.handler.GetNodes)
	s.router.GET("/api/clusters/:clusterID/nodes/:name", s.handler.GetNode)
}

// TearDownTest 每個測試後的清理
func (s *NodeHandlerTestSuite) TearDownTest() {
	if s.db != nil {
		sqlDB, _ := s.db.DB()
		if sqlDB != nil {
			_ = sqlDB.Close()
		}
	}
}

// TestGetNodes_ClusterNotFound 測試獲取節點列表時叢集不存在
func (s *NodeHandlerTestSuite) TestGetNodes_ClusterNotFound() {
	s.mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `clusters` WHERE `clusters`.`id` = ? AND `clusters`.`deleted_at` IS NULL ORDER BY `clusters`.`id` LIMIT ?")).
		WithArgs(999, 1).
		WillReturnError(gorm.ErrRecordNotFound)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/clusters/999/nodes", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusNotFound, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	s.Require().NoError(err)

	errObj, ok := response["error"].(map[string]interface{})
	s.Require().True(ok, "response should contain error object")
	assert.Equal(s.T(), "NOT_FOUND", errObj["code"])
}

// TestGetNodes_InvalidClusterID 測試無效的叢集 ID
func (s *NodeHandlerTestSuite) TestGetNodes_InvalidClusterID() {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/clusters/invalid/nodes", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusBadRequest, w.Code)
}

// TestGetNode_ClusterExists 測試獲取節點詳情時叢集存在
func (s *NodeHandlerTestSuite) TestGetNode_ClusterExists() {
	now := time.Now()
	rows := sqlmock.NewRows([]string{
		"id", "name", "api_server", "kube_config", "version", "status",
		"description", "environment", "region", "labels", "monitoring_config",
		"alert_manager_config", "created_at", "updated_at", "last_heartbeat",
	}).AddRow(
		1, "test-cluster", "https://kubernetes.example.com:6443", "test-config",
		"v1.28.0", "connected", "Test cluster", "dev", "cn-north-1",
		"{}", "{}", "{}", now, now, now,
	)

	s.mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `clusters` WHERE `clusters`.`id` = ? AND `clusters`.`deleted_at` IS NULL ORDER BY `clusters`.`id` LIMIT ?")).
		WithArgs(1, 1).
		WillReturnRows(rows)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/clusters/1/nodes/test-node", nil)
	s.router.ServeHTTP(w, req)

	// 由於 K8s 客戶端為 nil，應該返回錯誤（503 Service Unavailable）
	assert.True(s.T(), w.Code == http.StatusServiceUnavailable || w.Code == http.StatusInternalServerError || w.Code == http.StatusNotFound)
}

// TestNodeHandlerSuite 執行測試套件
func TestNodeHandlerSuite(t *testing.T) {
	suite.Run(t, new(NodeHandlerTestSuite))
}
