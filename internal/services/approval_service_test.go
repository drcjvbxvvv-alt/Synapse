package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/shaia/Synapse/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ApprovalServiceTestSuite is the test suite for ApprovalService.
type ApprovalServiceTestSuite struct {
	suite.Suite
	db      *gorm.DB
	mock    sqlmock.Sqlmock
	service *ApprovalService
}

func (s *ApprovalServiceTestSuite) SetupTest() {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	s.Require().NoError(err)

	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn:                 db,
		PreferSimpleProtocol: true,
	}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	s.Require().NoError(err)

	s.db = gormDB
	s.mock = mock
	s.service = NewApprovalService(gormDB)
}

func (s *ApprovalServiceTestSuite) TearDownTest() {
	if err := s.mock.ExpectationsWereMet(); err != nil {
		s.T().Errorf("unmet sqlmock expectations: %v", err)
	}
	if sqlDB, _ := s.db.DB(); sqlDB != nil {
		_ = sqlDB.Close()
	}
}

// approvalRequestRows builds a sqlmock.Rows for ApprovalRequest.
func approvalRequestRows() *sqlmock.Rows {
	now := time.Now()
	return sqlmock.NewRows([]string{
		"id", "created_at", "updated_at", "deleted_at",
		"cluster_id", "cluster_name", "namespace",
		"resource_kind", "resource_name", "action",
		"requester_id", "requester_name", "approver_id", "approver_name",
		"status", "payload", "reason", "expires_at", "approved_at",
	}).AddRow(
		1, now, now, nil,
		10, "prod-cluster", "default",
		"Deployment", "my-app", "scale",
		5, "alice", nil, "",
		"pending", `{}`, "", now.Add(time.Hour), nil,
	)
}

// namespaceProtectionRows builds a sqlmock.Rows for NamespaceProtection.
func namespaceProtectionRows() *sqlmock.Rows {
	now := time.Now()
	return sqlmock.NewRows([]string{
		"id", "created_at", "updated_at", "deleted_at",
		"cluster_id", "namespace", "require_approval", "description",
	}).AddRow(1, now, now, nil, 10, "production", true, "protected namespace")
}

// newApprovalRequest returns a minimal ApprovalRequest for write tests.
func newApprovalRequest() *models.ApprovalRequest {
	return &models.ApprovalRequest{
		ClusterID:    10,
		Namespace:    "default",
		ResourceKind: "Deployment",
		ResourceName: "my-app",
		Action:       "scale",
		RequesterID:  5,
		Status:       "pending",
		ExpiresAt:    time.Now().Add(time.Hour),
	}
}

// ---- CreateApprovalRequest ----

func (s *ApprovalServiceTestSuite) TestCreateApprovalRequest_Success() {
	s.mock.ExpectBegin()
	s.mock.ExpectQuery(`INSERT INTO "approval_requests"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	s.mock.ExpectCommit()

	err := s.service.CreateApprovalRequest(context.Background(), newApprovalRequest())
	assert.NoError(s.T(), err)
}

func (s *ApprovalServiceTestSuite) TestCreateApprovalRequest_DBError() {
	s.mock.ExpectBegin()
	s.mock.ExpectQuery(`INSERT INTO "approval_requests"`).
		WillReturnError(errors.New("db error"))
	s.mock.ExpectRollback()

	err := s.service.CreateApprovalRequest(context.Background(), newApprovalRequest())
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "create approval request")
}

// ---- ListApprovalRequests ----

func (s *ApprovalServiceTestSuite) TestListApprovalRequests_NoFilter() {
	s.mock.ExpectQuery("SELECT").
		WillReturnRows(approvalRequestRows())

	items, err := s.service.ListApprovalRequests(context.Background(), "", 0)
	assert.NoError(s.T(), err)
	assert.Len(s.T(), items, 1)
	assert.Equal(s.T(), uint(10), items[0].ClusterID)
}

func (s *ApprovalServiceTestSuite) TestListApprovalRequests_WithStatusFilter() {
	s.mock.ExpectQuery("SELECT").
		WillReturnRows(approvalRequestRows())

	items, err := s.service.ListApprovalRequests(context.Background(), "pending", 0)
	assert.NoError(s.T(), err)
	assert.Len(s.T(), items, 1)
}

func (s *ApprovalServiceTestSuite) TestListApprovalRequests_WithClusterFilter() {
	s.mock.ExpectQuery("SELECT").
		WillReturnRows(approvalRequestRows())

	items, err := s.service.ListApprovalRequests(context.Background(), "", 10)
	assert.NoError(s.T(), err)
	assert.Len(s.T(), items, 1)
}

func (s *ApprovalServiceTestSuite) TestListApprovalRequests_DBError() {
	s.mock.ExpectQuery("SELECT").
		WillReturnError(errors.New("connection lost"))

	items, err := s.service.ListApprovalRequests(context.Background(), "", 0)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "list approval requests")
	assert.Nil(s.T(), items)
}

// ---- GetApprovalRequest ----

func (s *ApprovalServiceTestSuite) TestGetApprovalRequest_Success() {
	s.mock.ExpectQuery("SELECT").
		WillReturnRows(approvalRequestRows())

	ar, err := s.service.GetApprovalRequest(context.Background(), 1)
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), ar)
	assert.Equal(s.T(), uint(10), ar.ClusterID)
}

func (s *ApprovalServiceTestSuite) TestGetApprovalRequest_NotFound() {
	s.mock.ExpectQuery("SELECT").
		WillReturnError(gorm.ErrRecordNotFound)

	ar, err := s.service.GetApprovalRequest(context.Background(), 999)
	assert.Error(s.T(), err)
	assert.Nil(s.T(), ar)
	assert.Contains(s.T(), err.Error(), "get approval request 999")
}

// ---- UpdateApprovalRequest ----

func (s *ApprovalServiceTestSuite) TestUpdateApprovalRequest_Success() {
	s.mock.ExpectBegin()
	s.mock.ExpectExec(`UPDATE "approval_requests"`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.mock.ExpectCommit()

	ar := &models.ApprovalRequest{}
	ar.ID = 1
	err := s.service.UpdateApprovalRequest(context.Background(), ar, map[string]interface{}{"status": "approved"})
	assert.NoError(s.T(), err)
}

func (s *ApprovalServiceTestSuite) TestUpdateApprovalRequest_DBError() {
	s.mock.ExpectBegin()
	s.mock.ExpectExec(`UPDATE "approval_requests"`).
		WillReturnError(errors.New("update failed"))
	s.mock.ExpectRollback()

	ar := &models.ApprovalRequest{}
	ar.ID = 2
	err := s.service.UpdateApprovalRequest(context.Background(), ar, map[string]interface{}{"status": "rejected"})
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "update approval request 2")
}

// ---- GetPendingCount ----

func (s *ApprovalServiceTestSuite) TestGetPendingCount_ReturnsCount() {
	s.mock.ExpectQuery("SELECT").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(7))

	count := s.service.GetPendingCount(context.Background())
	assert.Equal(s.T(), int64(7), count)
}

func (s *ApprovalServiceTestSuite) TestGetPendingCount_ZeroOnError() {
	s.mock.ExpectQuery("SELECT").
		WillReturnError(errors.New("db error"))

	count := s.service.GetPendingCount(context.Background())
	assert.Equal(s.T(), int64(0), count)
}

// ---- ListNamespaceProtections ----

func (s *ApprovalServiceTestSuite) TestListNamespaceProtections_Success() {
	s.mock.ExpectQuery("SELECT").
		WillReturnRows(namespaceProtectionRows())

	items, err := s.service.ListNamespaceProtections(context.Background(), 10)
	assert.NoError(s.T(), err)
	assert.Len(s.T(), items, 1)
	assert.Equal(s.T(), "production", items[0].Namespace)
}

func (s *ApprovalServiceTestSuite) TestListNamespaceProtections_DBError() {
	s.mock.ExpectQuery("SELECT").
		WillReturnError(errors.New("db error"))

	items, err := s.service.ListNamespaceProtections(context.Background(), 10)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "list namespace protections for cluster 10")
	assert.Nil(s.T(), items)
}

// ---- UpsertNamespaceProtection — create path ----

func (s *ApprovalServiceTestSuite) TestUpsertNamespaceProtection_Creates() {
	// First(): not found → trigger create path
	s.mock.ExpectQuery("SELECT").
		WillReturnError(gorm.ErrRecordNotFound)
	s.mock.ExpectBegin()
	s.mock.ExpectQuery(`INSERT INTO "namespace_protections"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	s.mock.ExpectCommit()

	np, err := s.service.UpsertNamespaceProtection(context.Background(), 10, "staging", true, "needs approval")
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), np)
	assert.Equal(s.T(), "staging", np.Namespace)
	assert.True(s.T(), np.RequireApproval)
}

func (s *ApprovalServiceTestSuite) TestUpsertNamespaceProtection_CreateError() {
	s.mock.ExpectQuery("SELECT").
		WillReturnError(gorm.ErrRecordNotFound)
	s.mock.ExpectBegin()
	s.mock.ExpectQuery(`INSERT INTO "namespace_protections"`).
		WillReturnError(errors.New("insert failed"))
	s.mock.ExpectRollback()

	np, err := s.service.UpsertNamespaceProtection(context.Background(), 10, "staging", true, "desc")
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "create namespace protection")
	assert.Nil(s.T(), np)
}

// ---- UpsertNamespaceProtection — update path ----

func (s *ApprovalServiceTestSuite) TestUpsertNamespaceProtection_Updates() {
	// First(): found
	s.mock.ExpectQuery("SELECT").
		WillReturnRows(namespaceProtectionRows())
	// Updates()
	s.mock.ExpectBegin()
	s.mock.ExpectExec(`UPDATE "namespace_protections"`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.mock.ExpectCommit()

	np, err := s.service.UpsertNamespaceProtection(context.Background(), 10, "production", false, "unlocked")
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), np)
}

func (s *ApprovalServiceTestSuite) TestUpsertNamespaceProtection_UpdateError() {
	s.mock.ExpectQuery("SELECT").
		WillReturnRows(namespaceProtectionRows())
	s.mock.ExpectBegin()
	s.mock.ExpectExec(`UPDATE "namespace_protections"`).
		WillReturnError(errors.New("update failed"))
	s.mock.ExpectRollback()

	np, err := s.service.UpsertNamespaceProtection(context.Background(), 10, "production", false, "desc")
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "update namespace protection")
	assert.Nil(s.T(), np)
}

// ---- GetNamespaceProtection ----

func (s *ApprovalServiceTestSuite) TestGetNamespaceProtection_Success() {
	s.mock.ExpectQuery("SELECT").
		WillReturnRows(namespaceProtectionRows())

	np, err := s.service.GetNamespaceProtection(context.Background(), 10, "production")
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), np)
	assert.True(s.T(), np.RequireApproval)
}

func (s *ApprovalServiceTestSuite) TestGetNamespaceProtection_NotFound() {
	s.mock.ExpectQuery("SELECT").
		WillReturnError(gorm.ErrRecordNotFound)

	np, err := s.service.GetNamespaceProtection(context.Background(), 10, "nonexistent")
	assert.Error(s.T(), err)
	assert.Nil(s.T(), np)
	assert.True(s.T(), errors.Is(err, gorm.ErrRecordNotFound))
}

func TestApprovalServiceSuite(t *testing.T) {
	suite.Run(t, new(ApprovalServiceTestSuite))
}
