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

	"kubepolaris/internal/config"
)

// ClusterHandlerTestSuite 定义集群处理器测试套件
type ClusterHandlerTestSuite struct {
	suite.Suite
	db      *gorm.DB
	mock    sqlmock.Sqlmock
	router  *gin.Engine
	handler *ClusterHandler
}

// SetupTest 每个测试前的设置
func (s *ClusterHandlerTestSuite) SetupTest() {
	// 设置 Gin 为测试模式
	gin.SetMode(gin.TestMode)

	// 创建 SQL Mock
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	s.Require().NoError(err)

	// 使用 mock 数据库创建 GORM 实例
	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      db,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	s.Require().NoError(err)

	s.db = gormDB
	s.mock = mock

	// 创建测试配置
	cfg := &config.Config{}

	// 创建处理器（部分依赖可能为 nil，需要在具体测试中处理）
	s.handler = NewClusterHandler(gormDB, cfg, nil, nil, nil)

	// 创建路由
	s.router = gin.New()
	s.router.GET("/api/clusters", s.handler.GetClusters)
	s.router.GET("/api/clusters/:id", s.handler.GetCluster)
	s.router.DELETE("/api/clusters/:id", s.handler.DeleteCluster)
}

// TearDownTest 每个测试后的清理
func (s *ClusterHandlerTestSuite) TearDownTest() {
	if s.db != nil {
		sqlDB, _ := s.db.DB()
		if sqlDB != nil {
			sqlDB.Close()
		}
	}
}

// TestGetClusters 测试获取集群列表
func (s *ClusterHandlerTestSuite) TestGetClusters() {
	// 设置期望的数据库查询
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

	s.mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `clusters`")).
		WillReturnRows(rows)

	// 创建请求
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/clusters", nil)
	s.router.ServeHTTP(w, req)

	// 验证响应
	assert.Equal(s.T(), http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	s.Require().NoError(err)

	assert.Equal(s.T(), float64(200), response["code"])
	assert.Equal(s.T(), "获取成功", response["message"])

	data := response["data"].(map[string]interface{})
	items := data["items"].([]interface{})
	assert.Len(s.T(), items, 1)

	// 验证集群数据
	cluster := items[0].(map[string]interface{})
	assert.Equal(s.T(), float64(1), cluster["id"])
	assert.Equal(s.T(), "test-cluster", cluster["name"])
	assert.Equal(s.T(), "https://kubernetes.example.com:6443", cluster["apiServer"])
}

// TestGetClusters_Empty 测试获取空集群列表
func (s *ClusterHandlerTestSuite) TestGetClusters_Empty() {
	rows := sqlmock.NewRows([]string{
		"id", "name", "api_server", "kube_config", "version", "status",
		"description", "environment", "region", "labels", "monitoring_config",
		"alert_manager_config", "created_at", "updated_at", "last_heartbeat",
	})

	s.mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `clusters`")).
		WillReturnRows(rows)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/clusters", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	s.Require().NoError(err)

	data := response["data"].(map[string]interface{})
	items := data["items"].([]interface{})
	assert.Len(s.T(), items, 0)
	assert.Equal(s.T(), float64(0), data["total"])
}

// TestGetCluster_Success 测试获取单个集群成功
func (s *ClusterHandlerTestSuite) TestGetCluster_Success() {
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

	s.mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `clusters` WHERE `clusters`.`id` = ?")).
		WithArgs(1).
		WillReturnRows(rows)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/clusters/1", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	s.Require().NoError(err)

	assert.Equal(s.T(), float64(200), response["code"])
}

// TestGetCluster_NotFound 测试获取不存在的集群
func (s *ClusterHandlerTestSuite) TestGetCluster_NotFound() {
	s.mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `clusters` WHERE `clusters`.`id` = ?")).
		WithArgs(999).
		WillReturnError(gorm.ErrRecordNotFound)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/clusters/999", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusNotFound, w.Code)
}

// TestGetCluster_InvalidID 测试无效的集群 ID
func (s *ClusterHandlerTestSuite) TestGetCluster_InvalidID() {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/clusters/invalid", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusBadRequest, w.Code)
}

// TestDeleteCluster_Success 测试删除集群成功
func (s *ClusterHandlerTestSuite) TestDeleteCluster_Success() {
	now := time.Now()
	// 先查询集群是否存在
	rows := sqlmock.NewRows([]string{
		"id", "name", "api_server", "kube_config", "version", "status",
		"description", "environment", "region", "labels", "monitoring_config",
		"alert_manager_config", "created_at", "updated_at", "last_heartbeat",
	}).AddRow(
		1, "test-cluster", "https://kubernetes.example.com:6443", "test-config",
		"v1.28.0", "connected", "Test cluster", "dev", "cn-north-1",
		"{}", "{}", "{}", now, now, now,
	)

	s.mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `clusters` WHERE `clusters`.`id` = ?")).
		WithArgs(1).
		WillReturnRows(rows)

	// 期望删除操作
	s.mock.ExpectBegin()
	s.mock.ExpectExec(regexp.QuoteMeta("DELETE FROM `clusters` WHERE `clusters`.`id` = ?")).
		WithArgs(1).
		WillReturnResult(sqlmock.NewResult(0, 1))
	s.mock.ExpectCommit()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/clusters/1", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusOK, w.Code)
}

// TestDeleteCluster_NotFound 测试删除不存在的集群
func (s *ClusterHandlerTestSuite) TestDeleteCluster_NotFound() {
	s.mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `clusters` WHERE `clusters`.`id` = ?")).
		WithArgs(999).
		WillReturnError(gorm.ErrRecordNotFound)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/clusters/999", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusNotFound, w.Code)
}

// TestClusterHandlerSuite 运行测试套件
func TestClusterHandlerSuite(t *testing.T) {
	suite.Run(t, new(ClusterHandlerTestSuite))
}
