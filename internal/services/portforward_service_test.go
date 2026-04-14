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

// PortForwardServiceTestSuite is the test suite for PortForwardService.
type PortForwardServiceTestSuite struct {
	suite.Suite
	db      *gorm.DB
	mock    sqlmock.Sqlmock
	service *PortForwardService
}

func (s *PortForwardServiceTestSuite) SetupTest() {
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
	s.service = NewPortForwardService(gormDB)
}

func (s *PortForwardServiceTestSuite) TearDownTest() {
	if err := s.mock.ExpectationsWereMet(); err != nil {
		s.T().Errorf("unmet sqlmock expectations: %v", err)
	}
	if sqlDB, _ := s.db.DB(); sqlDB != nil {
		_ = sqlDB.Close()
	}
}

// portForwardSessionColumns is the ordered column list for PortForwardSession.
var portForwardSessionColumns = []string{
	"id", "created_at", "updated_at", "deleted_at",
	"cluster_id", "cluster_name", "namespace", "pod_name",
	"pod_port", "local_port", "user_id", "username",
	"status", "stopped_at",
}

// portForwardSessionRows builds sqlmock.Rows for PortForwardSession.
func portForwardSessionRows() *sqlmock.Rows {
	now := time.Now()
	return sqlmock.NewRows(portForwardSessionColumns).AddRow(
		1, now, now, nil,
		10, "prod-cluster", "default", "my-pod",
		8080, 54321, 5, "alice",
		"active", nil,
	)
}

// newPortForwardSession returns a minimal PortForwardSession for write tests.
func newPortForwardSession() *models.PortForwardSession {
	return &models.PortForwardSession{
		ClusterID:   10,
		ClusterName: "prod-cluster",
		Namespace:   "default",
		PodName:     "my-pod",
		PodPort:     8080,
		LocalPort:   54321,
		UserID:      5,
		Username:    "alice",
		Status:      "active",
	}
}

// ---- CreateSession ----

func (s *PortForwardServiceTestSuite) TestCreateSession_Success() {
	s.mock.ExpectBegin()
	s.mock.ExpectQuery(`INSERT INTO "port_forward_sessions"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	s.mock.ExpectCommit()

	err := s.service.CreateSession(context.Background(), newPortForwardSession())
	assert.NoError(s.T(), err)
}

func (s *PortForwardServiceTestSuite) TestCreateSession_DBError() {
	s.mock.ExpectBegin()
	s.mock.ExpectQuery(`INSERT INTO "port_forward_sessions"`).
		WillReturnError(errors.New("constraint violation"))
	s.mock.ExpectRollback()

	err := s.service.CreateSession(context.Background(), newPortForwardSession())
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "create port-forward session")
}

// ---- MarkStopped ----

func (s *PortForwardServiceTestSuite) TestMarkStopped_Success() {
	// MarkStopped does not use WithContext — match plain UPDATE
	s.mock.ExpectBegin()
	s.mock.ExpectExec(`UPDATE "port_forward_sessions"`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.mock.ExpectCommit()

	// Should not panic; fire and forget
	s.service.MarkStopped(1)
}

func (s *PortForwardServiceTestSuite) TestMarkStopped_DBError() {
	// Even on DB error, MarkStopped silently ignores it
	s.mock.ExpectBegin()
	s.mock.ExpectExec(`UPDATE "port_forward_sessions"`).
		WillReturnError(errors.New("db error"))
	s.mock.ExpectRollback()

	// Must not panic
	s.service.MarkStopped(999)
}

// ---- StopSession ----

func (s *PortForwardServiceTestSuite) TestStopSession_Success() {
	s.mock.ExpectBegin()
	s.mock.ExpectExec(`UPDATE "port_forward_sessions"`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.mock.ExpectCommit()

	err := s.service.StopSession(context.Background(), 1)
	assert.NoError(s.T(), err)
}

func (s *PortForwardServiceTestSuite) TestStopSession_DBError() {
	s.mock.ExpectBegin()
	s.mock.ExpectExec(`UPDATE "port_forward_sessions"`).
		WillReturnError(errors.New("update failed"))
	s.mock.ExpectRollback()

	err := s.service.StopSession(context.Background(), 2)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "stop port-forward session 2")
}

// ---- ListSessions ----

func (s *PortForwardServiceTestSuite) TestListSessions_AllStatuses() {
	s.mock.ExpectQuery("SELECT").
		WillReturnRows(portForwardSessionRows())

	sessions, err := s.service.ListSessions(context.Background(), 5, "")
	assert.NoError(s.T(), err)
	assert.Len(s.T(), sessions, 1)
	assert.Equal(s.T(), "active", sessions[0].Status)
	assert.Equal(s.T(), "my-pod", sessions[0].PodName)
}

func (s *PortForwardServiceTestSuite) TestListSessions_WithStatusFilter() {
	s.mock.ExpectQuery("SELECT").
		WillReturnRows(portForwardSessionRows())

	sessions, err := s.service.ListSessions(context.Background(), 5, "active")
	assert.NoError(s.T(), err)
	assert.Len(s.T(), sessions, 1)
}

func (s *PortForwardServiceTestSuite) TestListSessions_Empty() {
	s.mock.ExpectQuery("SELECT").
		WillReturnRows(sqlmock.NewRows(portForwardSessionColumns))

	sessions, err := s.service.ListSessions(context.Background(), 99, "")
	assert.NoError(s.T(), err)
	assert.Empty(s.T(), sessions)
}

func (s *PortForwardServiceTestSuite) TestListSessions_DBError() {
	s.mock.ExpectQuery("SELECT").
		WillReturnError(errors.New("connection lost"))

	sessions, err := s.service.ListSessions(context.Background(), 5, "")
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "list port-forward sessions for user 5")
	assert.Nil(s.T(), sessions)
}

func TestPortForwardServiceSuite(t *testing.T) {
	suite.Run(t, new(PortForwardServiceTestSuite))
}
