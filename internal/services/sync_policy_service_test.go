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
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// SyncPolicyServiceTestSuite is the test suite for SyncPolicyService.
type SyncPolicyServiceTestSuite struct {
	suite.Suite
	db      *gorm.DB
	mock    sqlmock.Sqlmock
	service *SyncPolicyService
}

func (s *SyncPolicyServiceTestSuite) SetupTest() {
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
	s.service = NewSyncPolicyService(gormDB)
}

func (s *SyncPolicyServiceTestSuite) TearDownTest() {
	if err := s.mock.ExpectationsWereMet(); err != nil {
		s.T().Errorf("unmet sqlmock expectations: %v", err)
	}
	if sqlDB, _ := s.db.DB(); sqlDB != nil {
		_ = sqlDB.Close()
	}
}

// syncPolicyColumns is the column list for SyncPolicy rows.
var syncPolicyColumns = []string{
	"id", "name", "description",
	"source_cluster_id", "source_namespace", "resource_type",
	"resource_names", "target_clusters", "conflict_policy",
	"schedule", "enabled", "last_sync_at", "last_sync_status",
	"created_at", "updated_at",
}

// syncPolicyRows builds sqlmock.Rows for SyncPolicy.
func syncPolicyRows() *sqlmock.Rows {
	now := time.Now()
	return sqlmock.NewRows(syncPolicyColumns).AddRow(
		1, "prod-sync", "sync prod configs",
		10, "default", "ConfigMap",
		`["app-config"]`, `[20,30]`, "overwrite",
		"0 * * * *", true, nil, "",
		now, now,
	)
}

// syncHistoryRows builds sqlmock.Rows for SyncHistory.
func syncHistoryRows() *sqlmock.Rows {
	now := time.Now()
	return sqlmock.NewRows([]string{
		"id", "policy_id", "triggered_by", "status",
		"message", "details", "started_at", "finished_at",
	}).AddRow(
		1, 1, "manual", "success",
		"all targets synced", `{}`, now, &now,
	)
}

// newSyncPolicy returns a minimal SyncPolicy for write tests.
func newSyncPolicy() *models.SyncPolicy {
	return &models.SyncPolicy{
		Name:            "prod-sync",
		SourceClusterID: 10,
		SourceNamespace: "default",
		ResourceType:    "ConfigMap",
		ResourceNames:   `["app-config"]`,
		TargetClusters:  `[20,30]`,
		ConflictPolicy:  "overwrite",
		Enabled:         true,
	}
}

// ---- ListPolicies ----

func (s *SyncPolicyServiceTestSuite) TestListPolicies_Success() {
	s.mock.ExpectQuery("SELECT").
		WillReturnRows(syncPolicyRows())

	policies, err := s.service.ListPolicies(context.Background())
	assert.NoError(s.T(), err)
	assert.Len(s.T(), policies, 1)
	assert.Equal(s.T(), "prod-sync", policies[0].Name)
}

func (s *SyncPolicyServiceTestSuite) TestListPolicies_Empty() {
	s.mock.ExpectQuery("SELECT").
		WillReturnRows(sqlmock.NewRows(syncPolicyColumns))

	policies, err := s.service.ListPolicies(context.Background())
	assert.NoError(s.T(), err)
	assert.Empty(s.T(), policies)
}

func (s *SyncPolicyServiceTestSuite) TestListPolicies_DBError() {
	s.mock.ExpectQuery("SELECT").
		WillReturnError(errors.New("db unavailable"))

	policies, err := s.service.ListPolicies(context.Background())
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "list sync policies")
	assert.Nil(s.T(), policies)
}

// ---- CreatePolicy ----

func (s *SyncPolicyServiceTestSuite) TestCreatePolicy_Success() {
	s.mock.ExpectBegin()
	s.mock.ExpectExec("INSERT INTO `sync_policies`").
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.mock.ExpectCommit()

	err := s.service.CreatePolicy(context.Background(), newSyncPolicy())
	assert.NoError(s.T(), err)
}

func (s *SyncPolicyServiceTestSuite) TestCreatePolicy_DBError() {
	s.mock.ExpectBegin()
	s.mock.ExpectExec("INSERT INTO `sync_policies`").
		WillReturnError(errors.New("unique constraint"))
	s.mock.ExpectRollback()

	err := s.service.CreatePolicy(context.Background(), newSyncPolicy())
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "create sync policy")
}

// ---- GetPolicy ----

func (s *SyncPolicyServiceTestSuite) TestGetPolicy_Success() {
	s.mock.ExpectQuery("SELECT").
		WillReturnRows(syncPolicyRows())

	policy, err := s.service.GetPolicy(context.Background(), 1)
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), policy)
	assert.Equal(s.T(), uint(10), policy.SourceClusterID)
}

func (s *SyncPolicyServiceTestSuite) TestGetPolicy_NotFound() {
	s.mock.ExpectQuery("SELECT").
		WillReturnError(gorm.ErrRecordNotFound)

	policy, err := s.service.GetPolicy(context.Background(), 999)
	assert.Error(s.T(), err)
	assert.Nil(s.T(), policy)
	assert.Contains(s.T(), err.Error(), "get sync policy 999")
}

// ---- UpdatePolicy ----

func (s *SyncPolicyServiceTestSuite) TestUpdatePolicy_Success() {
	// GORM Save on a record with ID issues an UPDATE (with full field set)
	s.mock.ExpectBegin()
	s.mock.ExpectExec("UPDATE `sync_policies`").
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.mock.ExpectCommit()

	p := newSyncPolicy()
	p.ID = 1
	p.Name = "renamed-sync"
	err := s.service.UpdatePolicy(context.Background(), p)
	assert.NoError(s.T(), err)
}

func (s *SyncPolicyServiceTestSuite) TestUpdatePolicy_DBError() {
	s.mock.ExpectBegin()
	s.mock.ExpectExec("UPDATE `sync_policies`").
		WillReturnError(errors.New("update failed"))
	s.mock.ExpectRollback()

	p := newSyncPolicy()
	p.ID = 2
	err := s.service.UpdatePolicy(context.Background(), p)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "update sync policy 2")
}

// ---- DeletePolicy ----

func (s *SyncPolicyServiceTestSuite) TestDeletePolicy_Success() {
	s.mock.ExpectBegin()
	s.mock.ExpectExec("DELETE FROM `sync_policies`").
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.mock.ExpectCommit()

	err := s.service.DeletePolicy(context.Background(), 1)
	assert.NoError(s.T(), err)
}

func (s *SyncPolicyServiceTestSuite) TestDeletePolicy_DBError() {
	s.mock.ExpectBegin()
	s.mock.ExpectExec("DELETE FROM `sync_policies`").
		WillReturnError(errors.New("delete failed"))
	s.mock.ExpectRollback()

	err := s.service.DeletePolicy(context.Background(), 1)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "delete sync policy 1")
}

// ---- ListSyncHistory ----

func (s *SyncPolicyServiceTestSuite) TestListSyncHistory_Success() {
	s.mock.ExpectQuery("SELECT").
		WillReturnRows(syncHistoryRows())

	history, err := s.service.ListSyncHistory(context.Background(), 1, 10)
	assert.NoError(s.T(), err)
	assert.Len(s.T(), history, 1)
	assert.Equal(s.T(), uint(1), history[0].PolicyID)
	assert.Equal(s.T(), "success", history[0].Status)
}

func (s *SyncPolicyServiceTestSuite) TestListSyncHistory_Empty() {
	s.mock.ExpectQuery("SELECT").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "policy_id", "triggered_by", "status",
			"message", "details", "started_at", "finished_at",
		}))

	history, err := s.service.ListSyncHistory(context.Background(), 99, 10)
	assert.NoError(s.T(), err)
	assert.Empty(s.T(), history)
}

func (s *SyncPolicyServiceTestSuite) TestListSyncHistory_DBError() {
	s.mock.ExpectQuery("SELECT").
		WillReturnError(errors.New("query timeout"))

	history, err := s.service.ListSyncHistory(context.Background(), 1, 10)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "list sync history for policy 1")
	assert.Nil(s.T(), history)
}

func TestSyncPolicyServiceSuite(t *testing.T) {
	suite.Run(t, new(SyncPolicyServiceTestSuite))
}
