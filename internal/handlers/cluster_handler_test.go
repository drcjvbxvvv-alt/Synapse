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

// ClusterHandlerTestSuite 叢集處理器測試套件
type ClusterHandlerTestSuite struct {
	suite.Suite
	db      *gorm.DB
	mock    sqlmock.Sqlmock
	router  *gin.Engine
	handler *ClusterHandler
}

// clusterCols 是 sqlmock 回傳叢集行時使用的欄位清單，需要與 GORM 自動掃描的欄位匹配。
var clusterCols = []string{
	"id", "name", "api_server", "kubeconfig_enc", "sa_token_enc", "ca_enc",
	"version", "status", "description", "environment", "region",
	"labels", "monitoring_config", "alert_manager_config",
	"created_at", "updated_at", "deleted_at", "last_heartbeat",
	"created_by", "cert_expire_at",
}

func (s *ClusterHandlerTestSuite) SetupTest() {
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
	// nil repos → services fall back to the legacy *gorm.DB path.
	clusterSvc := services.NewClusterService(gormDB, nil)
	permSvc := services.NewPermissionService(gormDB, nil)

	s.handler = NewClusterHandler(cfg, nil, clusterSvc, nil, nil, permSvc)

	r := gin.New()
	r.GET("/api/clusters", s.handler.GetClusters)
	r.GET("/api/clusters/stats", s.handler.GetClusterStats)
	r.GET("/api/clusters/:clusterID", s.handler.GetCluster)
	s.router = r
}

func (s *ClusterHandlerTestSuite) TearDownTest() {
	if s.db != nil {
		sqlDB, _ := s.db.DB()
		if sqlDB != nil {
			_ = sqlDB.Close()
		}
	}
}

// ─── GetClusters ─────────────────────────────────────────────────────────────

// TestGetClusters_PlatformAdmin_Success: platform_admin gets all clusters.
func (s *ClusterHandlerTestSuite) TestGetClusters_PlatformAdmin_Success() {
	now := time.Now()

	// permSvc.GetUserAccessibleClusterIDs does s.db.First(&user, userID).
	// GORM adds soft-delete filter so use a broad regex to match the SELECT.
	s.mock.ExpectQuery(`SELECT \* FROM .users. WHERE .users.\..id. =`).
		WithArgs(1, 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "username", "password_hash", "salt", "email", "display_name",
			"phone", "auth_type", "status", "system_role",
			"last_login_at", "last_login_ip", "created_at", "updated_at", "deleted_at",
		}).AddRow(
			1, "admin", "", "", "", "Admin", "", "local", "active", "platform_admin",
			nil, "", now, now, nil,
		))

	// isAll == true → GetAllClusters (cache miss → db.Find)
	s.mock.ExpectQuery(`SELECT \* FROM .clusters.`).
		WillReturnRows(sqlmock.NewRows(clusterCols).AddRow(
			1, "prod-cluster", "https://k8s.example.com:6443", "", "", "",
			"v1.28.0", "connected", "", "", "",
			"{}", "{}", "{}",
			now, now, nil, &now,
			1, nil,
		))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/clusters", nil)
	// inject user_id = 1 (platform_admin) via a wrapper router
	adminRouter := gin.New()
	adminRouter.GET("/api/clusters", func(c *gin.Context) {
		c.Set("user_id", uint(1))
		s.handler.GetClusters(c)
	})
	adminRouter.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusOK, w.Code)

	var body map[string]interface{}
	s.Require().NoError(json.Unmarshal(w.Body.Bytes(), &body))
	assert.EqualValues(s.T(), 1, body["total"])
}

// TestGetClusters_RegularUser_GetsAllClusters: a regular user with no explicit
// permissions falls through to isAll=true (service default) and sees all clusters.
func (s *ClusterHandlerTestSuite) TestGetClusters_RegularUser_GetsAllClusters() {
	now := time.Now()

	// user lookup — regular user (not platform_admin)
	s.mock.ExpectQuery(`SELECT \* FROM .users. WHERE .users.\..id. =`).
		WithArgs(2, 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "username", "password_hash", "salt", "email", "display_name",
			"phone", "auth_type", "status", "system_role",
			"last_login_at", "last_login_ip", "created_at", "updated_at", "deleted_at",
		}).AddRow(
			2, "carol", "", "", "", "Carol", "", "local", "active", "user",
			nil, "", now, now, nil,
		))

	// COUNT admin permissions (none) — uses Model(&ClusterPermission{}) so no LIMIT arg
	s.mock.ExpectQuery(`SELECT count\(\*\) FROM .cluster_permissions.`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	// Pluck user group IDs from user_group_members (none)
	s.mock.ExpectQuery(`SELECT .user_group_id. FROM .user_group_members.`).
		WillReturnRows(sqlmock.NewRows([]string{"user_group_id"}))

	// Distinct Pluck cluster_id from cluster_permissions WHERE user_id = ? (none)
	s.mock.ExpectQuery(`SELECT DISTINCT .cluster_id. FROM .cluster_permissions.`).
		WillReturnRows(sqlmock.NewRows([]string{"cluster_id"}))

	// isAll=true → GetAllClusters (cache may be fresh from PlatformAdmin test — use a
	// separate Suite run so cache is empty, or just allow the Find to be skipped)
	// The cache TTL is 30s; each SetupTest creates a fresh ClusterService, so no stale cache.
	s.mock.ExpectQuery(`SELECT \* FROM .clusters.`).
		WillReturnRows(sqlmock.NewRows(clusterCols))

	noAccessRouter := gin.New()
	noAccessRouter.GET("/api/clusters", func(c *gin.Context) {
		c.Set("user_id", uint(2))
		s.handler.GetClusters(c)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/clusters", nil)
	noAccessRouter.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusOK, w.Code)

	var body map[string]interface{}
	s.Require().NoError(json.Unmarshal(w.Body.Bytes(), &body))
	assert.EqualValues(s.T(), 0, body["total"])
}

// TestGetClusters_UserNotFound: permission lookup fails → 500.
func (s *ClusterHandlerTestSuite) TestGetClusters_UserNotFound() {
	s.mock.ExpectQuery(`SELECT \* FROM .users. WHERE .users.\..id. =`).
		WithArgs(99, 1).
		WillReturnError(gorm.ErrRecordNotFound)

	notFoundRouter := gin.New()
	notFoundRouter.GET("/api/clusters", func(c *gin.Context) {
		c.Set("user_id", uint(99))
		s.handler.GetClusters(c)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/clusters", nil)
	notFoundRouter.ServeHTTP(w, req)

	// PermissionService returns ErrUserNotFound (404 AppError), but
	// ClusterHandler wraps it as InternalError (500).
	assert.Equal(s.T(), http.StatusInternalServerError, w.Code)
}

// ─── GetClusterStats ─────────────────────────────────────────────────────────

// TestGetClusterStats_Success: returns a JSON object with the stat fields.
func (s *ClusterHandlerTestSuite) TestGetClusterStats_Success() {
	// COUNT total
	s.mock.ExpectQuery(`SELECT count\(\*\) FROM .clusters.`).
		WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(3))
	// COUNT healthy
	s.mock.ExpectQuery(`SELECT count\(\*\) FROM .clusters.`).
		WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(2))
	// COUNT unhealthy
	s.mock.ExpectQuery(`SELECT count\(\*\) FROM .clusters.`).
		WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(1))
	// GetAllClusters (for real-time metrics) — return empty list so goroutines
	// finish immediately without needing a K8s client.
	s.mock.ExpectQuery(`SELECT \* FROM .clusters.`).
		WillReturnRows(sqlmock.NewRows(clusterCols))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/clusters/stats", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusOK, w.Code)

	var body map[string]interface{}
	s.Require().NoError(json.Unmarshal(w.Body.Bytes(), &body))
	assert.EqualValues(s.T(), 3, body["total_clusters"])
	assert.EqualValues(s.T(), 2, body["healthy_clusters"])
	assert.EqualValues(s.T(), 1, body["unhealthy_clusters"])
}

// TestGetClusterStats_DBError: count query fails → 500.
func (s *ClusterHandlerTestSuite) TestGetClusterStats_DBError() {
	s.mock.ExpectQuery(`SELECT count\(\*\) FROM .clusters.`).
		WillReturnError(gorm.ErrInvalidDB)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/clusters/stats", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusInternalServerError, w.Code)
}

// ─── GetCluster (single) ─────────────────────────────────────────────────────

func (s *ClusterHandlerTestSuite) TestGetCluster_Success() {
	now := time.Now()
	s.mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `clusters` WHERE `clusters`.`id` = ? AND `clusters`.`deleted_at` IS NULL ORDER BY `clusters`.`id` LIMIT ?")).
		WithArgs(1, 1).
		WillReturnRows(sqlmock.NewRows(clusterCols).AddRow(
			1, "prod-cluster", "https://k8s.example.com:6443", "", "", "",
			"v1.28.0", "connected", "", "", "",
			"{}", "{}", "{}",
			now, now, nil, &now,
			1, nil,
		))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/clusters/1", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusOK, w.Code)

	var body map[string]interface{}
	s.Require().NoError(json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(s.T(), "prod-cluster", body["name"])
	assert.Equal(s.T(), "connected", body["status"])
}

func (s *ClusterHandlerTestSuite) TestGetCluster_InvalidID() {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/clusters/not-a-number", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusBadRequest, w.Code)
}

func (s *ClusterHandlerTestSuite) TestGetCluster_NotFound() {
	s.mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `clusters` WHERE `clusters`.`id` = ? AND `clusters`.`deleted_at` IS NULL ORDER BY `clusters`.`id` LIMIT ?")).
		WithArgs(999, 1).
		WillReturnError(gorm.ErrRecordNotFound)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/clusters/999", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusNotFound, w.Code)

	var body map[string]interface{}
	s.Require().NoError(json.Unmarshal(w.Body.Bytes(), &body))
	errObj, ok := body["error"].(map[string]interface{})
	s.Require().True(ok)
	assert.Equal(s.T(), "NOT_FOUND", errObj["code"])
}

// ─── Suite runner ─────────────────────────────────────────────────────────────

func TestClusterHandlerSuite(t *testing.T) {
	suite.Run(t, new(ClusterHandlerTestSuite))
}
