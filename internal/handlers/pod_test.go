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

	"github.com/clay-wangzhi/Synapse/internal/config"
	"github.com/clay-wangzhi/Synapse/internal/services"
)

// PodHandlerTestSuite 定義 Pod 處理器測試套件
type PodHandlerTestSuite struct {
	suite.Suite
	db      *gorm.DB
	mock    sqlmock.Sqlmock
	router  *gin.Engine
	handler *PodHandler
}

// SetupTest 每個測試前的設定
func (s *PodHandlerTestSuite) SetupTest() {
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
	clusterService := services.NewClusterService(gormDB)
	s.handler = NewPodHandler(gormDB, cfg, clusterService, nil)

	s.router = gin.New()
	// 新增叢集 ID 路由參數
	s.router.GET("/api/clusters/:clusterID/pods", s.handler.GetPods)
	s.router.GET("/api/clusters/:clusterID/namespaces/:namespace/pods/:name", s.handler.GetPod)
}

// TearDownTest 每個測試後的清理
func (s *PodHandlerTestSuite) TearDownTest() {
	if s.db != nil {
		sqlDB, _ := s.db.DB()
		if sqlDB != nil {
			_ = sqlDB.Close()
		}
	}
}

// TestGetPods_ClusterNotFound 測試獲取 Pod 列表時叢集不存在
func (s *PodHandlerTestSuite) TestGetPods_ClusterNotFound() {
	s.mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `clusters` WHERE `clusters`.`id` = ? AND `clusters`.`deleted_at` IS NULL ORDER BY `clusters`.`id` LIMIT ?")).
		WithArgs(999, 1).
		WillReturnError(gorm.ErrRecordNotFound)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/clusters/999/pods", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusNotFound, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	s.Require().NoError(err)

	errObj, ok := response["error"].(map[string]interface{})
	s.Require().True(ok, "response should contain error object")
	assert.Equal(s.T(), "NOT_FOUND", errObj["code"])
}

// TestGetPods_InvalidClusterID 測試無效的叢集 ID
func (s *PodHandlerTestSuite) TestGetPods_InvalidClusterID() {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/clusters/invalid/pods", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusBadRequest, w.Code)
}

// TestGetPod_InvalidParams 測試無效的參數
func (s *PodHandlerTestSuite) TestGetPod_InvalidParams() {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/clusters/invalid/namespaces/default/pods/test-pod", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusBadRequest, w.Code)
}

// TestGetPod_ClusterExists 測試獲取 Pod 詳情時叢集存在但 K8s 連線為空
func (s *PodHandlerTestSuite) TestGetPod_ClusterExists() {
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
	req, _ := http.NewRequest("GET", "/api/clusters/1/namespaces/default/pods/test-pod", nil)
	s.router.ServeHTTP(w, req)

	// 由於 K8s 客戶端為 nil，應該返回錯誤（503 或 500）
	assert.True(s.T(), w.Code == http.StatusInternalServerError || w.Code == http.StatusServiceUnavailable || w.Code == http.StatusNotFound)
}

// TestPodHandlerSuite 執行測試套件
func TestPodHandlerSuite(t *testing.T) {
	suite.Run(t, new(PodHandlerTestSuite))
}
