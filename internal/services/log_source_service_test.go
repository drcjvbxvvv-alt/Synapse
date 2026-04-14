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

// LogSourceServiceTestSuite is the test suite for LogSourceService.
type LogSourceServiceTestSuite struct {
	suite.Suite
	db      *gorm.DB
	mock    sqlmock.Sqlmock
	service *LogSourceService
}

func (s *LogSourceServiceTestSuite) SetupTest() {
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
	s.service = NewLogSourceService(gormDB)
}

func (s *LogSourceServiceTestSuite) TearDownTest() {
	if err := s.mock.ExpectationsWereMet(); err != nil {
		s.T().Errorf("unmet sqlmock expectations: %v", err)
	}
	if sqlDB, _ := s.db.DB(); sqlDB != nil {
		_ = sqlDB.Close()
	}
}

// logSourceRows builds sqlmock.Rows for LogSourceConfig.
func logSourceRows() *sqlmock.Rows {
	now := time.Now()
	return sqlmock.NewRows([]string{
		"id", "cluster_id", "type", "name", "url",
		"username", "password", "api_key", "enabled",
		"created_at", "updated_at", "deleted_at",
	}).AddRow(
		1, 10, "loki", "prod-loki", "http://loki:3100",
		"user", "secret", "apikey123", true,
		now, now, nil,
	)
}

// newLogSource returns a minimal LogSourceConfig for write tests.
func newLogSource() *models.LogSourceConfig {
	return &models.LogSourceConfig{
		ClusterID: 10,
		Type:      "loki",
		Name:      "prod-loki",
		URL:       "http://loki:3100",
		Username:  "user",
		Password:  "secret",
		APIKey:    "apikey123",
		Enabled:   true,
	}
}

// ---- ListLogSources ----

func (s *LogSourceServiceTestSuite) TestListLogSources_Success() {
	s.mock.ExpectQuery("SELECT").
		WillReturnRows(logSourceRows())

	sources, err := s.service.ListLogSources(context.Background(), 10)
	assert.NoError(s.T(), err)
	assert.Len(s.T(), sources, 1)
	// Credentials must be masked
	assert.Empty(s.T(), sources[0].Password)
	assert.Empty(s.T(), sources[0].APIKey)
	assert.Equal(s.T(), "prod-loki", sources[0].Name)
}

func (s *LogSourceServiceTestSuite) TestListLogSources_Empty() {
	s.mock.ExpectQuery("SELECT").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "cluster_id", "type", "name", "url",
			"username", "password", "api_key", "enabled",
			"created_at", "updated_at", "deleted_at",
		}))

	sources, err := s.service.ListLogSources(context.Background(), 99)
	assert.NoError(s.T(), err)
	assert.Empty(s.T(), sources)
}

func (s *LogSourceServiceTestSuite) TestListLogSources_DBError() {
	s.mock.ExpectQuery("SELECT").
		WillReturnError(errors.New("connection refused"))

	sources, err := s.service.ListLogSources(context.Background(), 10)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "list log sources for cluster 10")
	assert.Nil(s.T(), sources)
}

// ---- CreateLogSource ----

func (s *LogSourceServiceTestSuite) TestCreateLogSource_Success() {
	s.mock.ExpectBegin()
	s.mock.ExpectQuery(`INSERT INTO "log_source_configs"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	s.mock.ExpectCommit()

	src := newLogSource()
	err := s.service.CreateLogSource(context.Background(), src)
	assert.NoError(s.T(), err)
	// Credentials must be masked after create
	assert.Empty(s.T(), src.Password)
	assert.Empty(s.T(), src.APIKey)
}

func (s *LogSourceServiceTestSuite) TestCreateLogSource_DBError() {
	s.mock.ExpectBegin()
	s.mock.ExpectQuery(`INSERT INTO "log_source_configs"`).
		WillReturnError(errors.New("duplicate entry"))
	s.mock.ExpectRollback()

	err := s.service.CreateLogSource(context.Background(), newLogSource())
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "create log source")
}

// ---- GetLogSource ----

func (s *LogSourceServiceTestSuite) TestGetLogSource_Success() {
	s.mock.ExpectQuery("SELECT").
		WillReturnRows(logSourceRows())

	src, err := s.service.GetLogSource(context.Background(), 1, 10)
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), src)
	assert.Equal(s.T(), "loki", src.Type)
	assert.Equal(s.T(), uint(10), src.ClusterID)
}

func (s *LogSourceServiceTestSuite) TestGetLogSource_NotFound() {
	s.mock.ExpectQuery("SELECT").
		WillReturnError(gorm.ErrRecordNotFound)

	src, err := s.service.GetLogSource(context.Background(), 999, 10)
	assert.Error(s.T(), err)
	assert.Nil(s.T(), src)
	assert.Contains(s.T(), err.Error(), "get log source 999")
}

// ---- GetEnabledLogSource ----

func (s *LogSourceServiceTestSuite) TestGetEnabledLogSource_Success() {
	s.mock.ExpectQuery("SELECT").
		WillReturnRows(logSourceRows())

	src, err := s.service.GetEnabledLogSource(context.Background(), 1, 10)
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), src)
	assert.True(s.T(), src.Enabled)
}

func (s *LogSourceServiceTestSuite) TestGetEnabledLogSource_NotFound() {
	s.mock.ExpectQuery("SELECT").
		WillReturnError(gorm.ErrRecordNotFound)

	src, err := s.service.GetEnabledLogSource(context.Background(), 1, 10)
	assert.Error(s.T(), err)
	assert.Nil(s.T(), src)
	assert.Contains(s.T(), err.Error(), "get enabled log source 1")
}

// ---- UpdateLogSource ----

func (s *LogSourceServiceTestSuite) TestUpdateLogSource_Success() {
	s.mock.ExpectBegin()
	s.mock.ExpectExec(`UPDATE "log_source_configs"`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.mock.ExpectCommit()

	src := &models.LogSourceConfig{}
	src.ID = 1
	err := s.service.UpdateLogSource(context.Background(), src, map[string]interface{}{"enabled": false})
	assert.NoError(s.T(), err)
}

func (s *LogSourceServiceTestSuite) TestUpdateLogSource_DBError() {
	s.mock.ExpectBegin()
	s.mock.ExpectExec(`UPDATE "log_source_configs"`).
		WillReturnError(errors.New("update failed"))
	s.mock.ExpectRollback()

	src := &models.LogSourceConfig{}
	src.ID = 2
	err := s.service.UpdateLogSource(context.Background(), src, map[string]interface{}{"url": "http://new-loki"})
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "update log source 2")
}

// ---- DeleteLogSource ----

func (s *LogSourceServiceTestSuite) TestDeleteLogSource_Success() {
	s.mock.ExpectBegin()
	s.mock.ExpectExec(`UPDATE "log_source_configs"`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.mock.ExpectCommit()

	err := s.service.DeleteLogSource(context.Background(), 1, 10)
	assert.NoError(s.T(), err)
}

func (s *LogSourceServiceTestSuite) TestDeleteLogSource_DBError() {
	s.mock.ExpectBegin()
	s.mock.ExpectExec(`UPDATE "log_source_configs"`).
		WillReturnError(errors.New("delete failed"))
	s.mock.ExpectRollback()

	err := s.service.DeleteLogSource(context.Background(), 1, 10)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "delete log source 1")
}

func TestLogSourceServiceSuite(t *testing.T) {
	suite.Run(t, new(LogSourceServiceTestSuite))
}
