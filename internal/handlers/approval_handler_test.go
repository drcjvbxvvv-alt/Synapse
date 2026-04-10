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

// ApprovalHandlerTestSuite 審批處理器測試套件
type ApprovalHandlerTestSuite struct {
	suite.Suite
	db      *gorm.DB
	mock    sqlmock.Sqlmock
	handler *ApprovalHandler
	router  *gin.Engine
}

// approvalRequestCols is the column list for ApprovalRequest sqlmock rows.
// gorm.Model expands to: id, created_at, updated_at, deleted_at
var approvalRequestCols = []string{
	"id", "created_at", "updated_at", "deleted_at",
	"cluster_id", "cluster_name", "namespace", "resource_kind", "resource_name",
	"action", "requester_id", "requester_name", "approver_id", "approver_name",
	"status", "payload", "reason", "expires_at", "approved_at",
}

// namespaceProtectionCols is the column list for NamespaceProtection sqlmock rows.
var namespaceProtectionCols = []string{
	"id", "created_at", "updated_at", "deleted_at",
	"cluster_id", "namespace", "require_approval", "description",
}

func (s *ApprovalHandlerTestSuite) SetupTest() {
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

	approvalSvc := services.NewApprovalService(gormDB)
	// nil repo → ClusterService falls back to legacy *gorm.DB path.
	clusterSvc := services.NewClusterService(gormDB, nil)
	s.handler = NewApprovalHandler(approvalSvc, clusterSvc)

	r := gin.New()
	r.GET("/api/approvals", s.handler.ListApprovalRequests)
	r.GET("/api/approvals/pending-count", s.handler.GetPendingCount)
	r.GET("/api/clusters/:clusterID/namespace-protections", s.handler.GetNamespaceProtections)
	r.GET("/api/clusters/:clusterID/namespace-protections/:namespace", s.handler.GetNamespaceProtectionStatus)
	r.PUT("/api/clusters/:clusterID/namespace-protections/:namespace", s.handler.SetNamespaceProtection)
	s.router = r
}

func (s *ApprovalHandlerTestSuite) TearDownTest() {
	if s.db != nil {
		sqlDB, _ := s.db.DB()
		if sqlDB != nil {
			_ = sqlDB.Close()
		}
	}
}

// ─── ListApprovalRequests ────────────────────────────────────────────────────

func (s *ApprovalHandlerTestSuite) TestListApprovalRequests_Success() {
	now := time.Now()

	// ExpireStaleRequests: BEGIN + UPDATE + COMMIT
	s.mock.ExpectBegin()
	s.mock.ExpectExec("UPDATE `approval_requests`").
		WillReturnResult(sqlmock.NewResult(0, 0))
	s.mock.ExpectCommit()

	// ListApprovalRequests SELECT
	s.mock.ExpectQuery(`SELECT .* FROM .approval_requests.`).
		WillReturnRows(sqlmock.NewRows(approvalRequestCols).AddRow(
			1, now, now, nil,
			2, "prod-cluster", "default", "Deployment", "nginx",
			"scale", 1, "alice", nil, "",
			"pending", "", "", now.Add(24*time.Hour), nil,
		))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/approvals", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusOK, w.Code)

	var body map[string]interface{}
	s.Require().NoError(json.Unmarshal(w.Body.Bytes(), &body))
	items, ok := body["items"].([]interface{})
	s.Require().True(ok)
	assert.Len(s.T(), items, 1)
	assert.EqualValues(s.T(), 1, body["total"])
}

func (s *ApprovalHandlerTestSuite) TestListApprovalRequests_FilterByStatus() {
	// ExpireStaleRequests: BEGIN + UPDATE + COMMIT
	s.mock.ExpectBegin()
	s.mock.ExpectExec("UPDATE `approval_requests`").
		WillReturnResult(sqlmock.NewResult(0, 0))
	s.mock.ExpectCommit()

	// SELECT with status filter
	s.mock.ExpectQuery(`SELECT .* FROM .approval_requests.`).
		WillReturnRows(sqlmock.NewRows(approvalRequestCols))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/approvals?status=approved", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusOK, w.Code)

	var body map[string]interface{}
	s.Require().NoError(json.Unmarshal(w.Body.Bytes(), &body))
	assert.EqualValues(s.T(), 0, body["total"])
}

func (s *ApprovalHandlerTestSuite) TestListApprovalRequests_DBError() {
	// ExpireStaleRequests: BEGIN + UPDATE + COMMIT
	s.mock.ExpectBegin()
	s.mock.ExpectExec("UPDATE `approval_requests`").
		WillReturnResult(sqlmock.NewResult(0, 0))
	s.mock.ExpectCommit()

	// SELECT returns DB error
	s.mock.ExpectQuery(`SELECT .* FROM .approval_requests.`).
		WillReturnError(gorm.ErrInvalidDB)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/approvals", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusInternalServerError, w.Code)
}

// ─── GetPendingCount ─────────────────────────────────────────────────────────

func (s *ApprovalHandlerTestSuite) TestGetPendingCount_Success() {
	// ExpireStaleRequests: BEGIN + UPDATE + COMMIT
	s.mock.ExpectBegin()
	s.mock.ExpectExec("UPDATE `approval_requests`").
		WillReturnResult(sqlmock.NewResult(0, 0))
	s.mock.ExpectCommit()

	// COUNT query
	s.mock.ExpectQuery(`SELECT count\(\*\) FROM .approval_requests.`).
		WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(5))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/approvals/pending-count", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusOK, w.Code)

	var body map[string]interface{}
	s.Require().NoError(json.Unmarshal(w.Body.Bytes(), &body))
	assert.EqualValues(s.T(), 5, body["count"])
}

func (s *ApprovalHandlerTestSuite) TestGetPendingCount_Zero() {
	// ExpireStaleRequests: BEGIN + UPDATE + COMMIT
	s.mock.ExpectBegin()
	s.mock.ExpectExec("UPDATE `approval_requests`").
		WillReturnResult(sqlmock.NewResult(0, 0))
	s.mock.ExpectCommit()

	// COUNT returns 0
	s.mock.ExpectQuery(`SELECT count\(\*\) FROM .approval_requests.`).
		WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(0))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/approvals/pending-count", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusOK, w.Code)

	var body map[string]interface{}
	s.Require().NoError(json.Unmarshal(w.Body.Bytes(), &body))
	assert.EqualValues(s.T(), 0, body["count"])
}

// ─── GetNamespaceProtections ─────────────────────────────────────────────────

func (s *ApprovalHandlerTestSuite) TestGetNamespaceProtections_Success() {
	now := time.Now()

	s.mock.ExpectQuery(`SELECT .* FROM .namespace_protections.`).
		WillReturnRows(sqlmock.NewRows(namespaceProtectionCols).AddRow(
			1, now, now, nil,
			1, "default", true, "production namespace",
		))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/clusters/1/namespace-protections", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusOK, w.Code)

	var body map[string]interface{}
	s.Require().NoError(json.Unmarshal(w.Body.Bytes(), &body))
	items, ok := body["items"].([]interface{})
	s.Require().True(ok)
	assert.Len(s.T(), items, 1)
}

func (s *ApprovalHandlerTestSuite) TestGetNamespaceProtections_InvalidClusterID() {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/clusters/abc/namespace-protections", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusBadRequest, w.Code)
}

func (s *ApprovalHandlerTestSuite) TestGetNamespaceProtections_Empty() {
	s.mock.ExpectQuery(`SELECT .* FROM .namespace_protections.`).
		WillReturnRows(sqlmock.NewRows(namespaceProtectionCols))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/clusters/1/namespace-protections", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusOK, w.Code)

	var body map[string]interface{}
	s.Require().NoError(json.Unmarshal(w.Body.Bytes(), &body))
	items, ok := body["items"].([]interface{})
	s.Require().True(ok)
	assert.Len(s.T(), items, 0)
}

// ─── GetNamespaceProtectionStatus ────────────────────────────────────────────

func (s *ApprovalHandlerTestSuite) TestGetNamespaceProtectionStatus_Found() {
	now := time.Now()

	s.mock.ExpectQuery(`SELECT .* FROM .namespace_protections.`).
		WillReturnRows(sqlmock.NewRows(namespaceProtectionCols).AddRow(
			1, now, now, nil,
			1, "default", true, "critical namespace",
		))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/clusters/1/namespace-protections/default", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusOK, w.Code)

	var body map[string]interface{}
	s.Require().NoError(json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(s.T(), true, body["requireApproval"])
	assert.Equal(s.T(), "critical namespace", body["description"])
}

func (s *ApprovalHandlerTestSuite) TestGetNamespaceProtectionStatus_NotFound() {
	s.mock.ExpectQuery(`SELECT .* FROM .namespace_protections.`).
		WillReturnError(gorm.ErrRecordNotFound)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/clusters/1/namespace-protections/staging", nil)
	s.router.ServeHTTP(w, req)

	// Not found returns 200 with requireApproval=false (graceful default)
	assert.Equal(s.T(), http.StatusOK, w.Code)

	var body map[string]interface{}
	s.Require().NoError(json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(s.T(), false, body["requireApproval"])
	assert.Equal(s.T(), "", body["description"])
}

func (s *ApprovalHandlerTestSuite) TestGetNamespaceProtectionStatus_InvalidClusterID() {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/clusters/notanid/namespace-protections/default", nil)
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusBadRequest, w.Code)
}

// ─── SetNamespaceProtection ───────────────────────────────────────────────────

func (s *ApprovalHandlerTestSuite) TestSetNamespaceProtection_Create() {
	// UpsertNamespaceProtection: First → not found, then Create
	s.mock.ExpectQuery(`SELECT .* FROM .namespace_protections.`).
		WillReturnError(gorm.ErrRecordNotFound)
	s.mock.ExpectBegin()
	s.mock.ExpectExec(`INSERT INTO .namespace_protections.`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.mock.ExpectCommit()

	body := `{"requireApproval":true,"description":"production environment"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/clusters/1/namespace-protections/production",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusOK, w.Code)
}

func (s *ApprovalHandlerTestSuite) TestSetNamespaceProtection_Update() {
	now := time.Now()

	// UpsertNamespaceProtection: First → found, then Update
	s.mock.ExpectQuery(`SELECT .* FROM .namespace_protections.`).
		WillReturnRows(sqlmock.NewRows(namespaceProtectionCols).AddRow(
			1, now, now, nil,
			1, "default", true, "old description",
		))
	s.mock.ExpectBegin()
	s.mock.ExpectExec(`UPDATE .namespace_protections.`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.mock.ExpectCommit()

	body := `{"requireApproval":false,"description":"no longer protected"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/clusters/1/namespace-protections/default",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusOK, w.Code)
}

func (s *ApprovalHandlerTestSuite) TestSetNamespaceProtection_InvalidClusterID() {
	body := `{"requireApproval":true}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/clusters/bad/namespace-protections/default",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	s.router.ServeHTTP(w, req)

	assert.Equal(s.T(), http.StatusBadRequest, w.Code)
}

// ─── Suite runner ────────────────────────────────────────────────────────────

func TestApprovalHandlerSuite(t *testing.T) {
	suite.Run(t, new(ApprovalHandlerTestSuite))
}
