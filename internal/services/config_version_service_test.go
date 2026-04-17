package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ConfigVersionServiceTestSuite is the test suite for ConfigVersionService.
type ConfigVersionServiceTestSuite struct {
	suite.Suite
	db      *gorm.DB
	mock    sqlmock.Sqlmock
	service *ConfigVersionService
}

func (s *ConfigVersionServiceTestSuite) SetupTest() {
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
	s.service = NewConfigVersionService(gormDB)
}

func (s *ConfigVersionServiceTestSuite) TearDownTest() {
	if err := s.mock.ExpectationsWereMet(); err != nil {
		s.T().Errorf("unmet sqlmock expectations: %v", err)
	}
	if sqlDB, _ := s.db.DB(); sqlDB != nil {
		_ = sqlDB.Close()
	}
}

// configVersionRows builds sqlmock.Rows for ConfigVersion.
func configVersionRows() *sqlmock.Rows {
	now := time.Now()
	return sqlmock.NewRows([]string{
		"id", "cluster_id", "resource_type", "namespace", "name",
		"version", "content_json", "changed_by", "changed_at",
	}).AddRow(
		1, 10, "configmap", "default", "my-config",
		2, `{"key":"value"}`, "alice", now,
	)
}

// ---- SaveConfigMapVersion ----

func (s *ConfigVersionServiceTestSuite) TestSaveConfigMapVersion_Success() {
	// saveVersion: BEGIN → SELECT MAX → INSERT → COMMIT (atomic)
	s.mock.ExpectBegin()
	s.mock.ExpectQuery("SELECT").
		WillReturnRows(sqlmock.NewRows([]string{"COALESCE(MAX(version),0) + 1"}).AddRow(1))
	s.mock.ExpectQuery(`INSERT INTO "config_versions"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	s.mock.ExpectCommit()

	data := map[string]string{"key": "value", "env": "prod"}
	s.service.SaveConfigMapVersion(context.Background(), 10, "default", "my-config", "alice", data)
}

func (s *ConfigVersionServiceTestSuite) TestSaveConfigMapVersion_InsertError() {
	// INSERT fails → whole transaction is rolled back
	s.mock.ExpectBegin()
	s.mock.ExpectQuery("SELECT").
		WillReturnRows(sqlmock.NewRows([]string{"COALESCE(MAX(version),0) + 1"}).AddRow(3))
	s.mock.ExpectQuery(`INSERT INTO "config_versions"`).
		WillReturnError(errors.New("disk full"))
	s.mock.ExpectRollback()

	// Should not panic — logs Warn and continues
	s.service.SaveConfigMapVersion(context.Background(), 10, "default", "my-config", "alice", map[string]string{"k": "v"})
}

func (s *ConfigVersionServiceTestSuite) TestSaveConfigMapVersion_ZeroVersionFallback() {
	// SELECT returns 0 → code falls back to nextVer=1
	s.mock.ExpectBegin()
	s.mock.ExpectQuery("SELECT").
		WillReturnRows(sqlmock.NewRows([]string{"COALESCE(MAX(version),0) + 1"}).AddRow(0))
	s.mock.ExpectQuery(`INSERT INTO "config_versions"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	s.mock.ExpectCommit()

	s.service.SaveConfigMapVersion(context.Background(), 10, "default", "my-config", "alice", map[string]string{})
}

// ---- SaveSecretVersion ----

func (s *ConfigVersionServiceTestSuite) TestSaveSecretVersion_Success() {
	s.mock.ExpectBegin()
	s.mock.ExpectQuery("SELECT").
		WillReturnRows(sqlmock.NewRows([]string{"COALESCE(MAX(version),0) + 1"}).AddRow(1))
	s.mock.ExpectQuery(`INSERT INTO "config_versions"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	s.mock.ExpectCommit()

	data := map[string][]byte{"password": []byte("s3cr3t"), "token": []byte("tok")}
	// Secret values must NOT be stored (only key names)
	s.service.SaveSecretVersion(context.Background(), 10, "default", "my-secret", "bob", data)
}

func (s *ConfigVersionServiceTestSuite) TestSaveSecretVersion_InsertError() {
	s.mock.ExpectBegin()
	s.mock.ExpectQuery("SELECT").
		WillReturnRows(sqlmock.NewRows([]string{"COALESCE(MAX(version),0) + 1"}).AddRow(2))
	s.mock.ExpectQuery(`INSERT INTO "config_versions"`).
		WillReturnError(errors.New("constraint violation"))
	s.mock.ExpectRollback()

	s.service.SaveSecretVersion(context.Background(), 10, "default", "my-secret", "bob", map[string][]byte{"k": []byte("v")})
}

// ---- ListVersions ----

func (s *ConfigVersionServiceTestSuite) TestListVersions_Success() {
	s.mock.ExpectQuery("SELECT").
		WillReturnRows(configVersionRows())

	versions, err := s.service.ListVersions(context.Background(), 10, "configmap", "default", "my-config")
	assert.NoError(s.T(), err)
	assert.Len(s.T(), versions, 1)
	assert.Equal(s.T(), 2, versions[0].Version)
	assert.Equal(s.T(), "alice", versions[0].ChangedBy)
}

func (s *ConfigVersionServiceTestSuite) TestListVersions_Empty() {
	s.mock.ExpectQuery("SELECT").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "cluster_id", "resource_type", "namespace", "name",
			"version", "content_json", "changed_by", "changed_at",
		}))

	versions, err := s.service.ListVersions(context.Background(), 10, "secret", "kube-system", "nonexistent")
	assert.NoError(s.T(), err)
	assert.Empty(s.T(), versions)
}

func (s *ConfigVersionServiceTestSuite) TestListVersions_DBError() {
	s.mock.ExpectQuery("SELECT").
		WillReturnError(errors.New("query failed"))

	versions, err := s.service.ListVersions(context.Background(), 10, "configmap", "default", "my-config")
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "list versions for configmap/default/my-config")
	assert.Nil(s.T(), versions)
}

// ---- GetVersion ----

func (s *ConfigVersionServiceTestSuite) TestGetVersion_Success() {
	s.mock.ExpectQuery("SELECT").
		WillReturnRows(configVersionRows())

	ver, err := s.service.GetVersion(context.Background(), 10, "configmap", "default", "my-config", 2)
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), ver)
	assert.Equal(s.T(), 2, ver.Version)
	assert.Equal(s.T(), `{"key":"value"}`, ver.ContentJSON)
}

func (s *ConfigVersionServiceTestSuite) TestGetVersion_NotFound() {
	s.mock.ExpectQuery("SELECT").
		WillReturnError(gorm.ErrRecordNotFound)

	ver, err := s.service.GetVersion(context.Background(), 10, "configmap", "default", "my-config", 99)
	assert.Error(s.T(), err)
	assert.Nil(s.T(), ver)
	assert.Contains(s.T(), err.Error(), "get version 99 for configmap/default/my-config")
}

func (s *ConfigVersionServiceTestSuite) TestGetVersion_DBError() {
	s.mock.ExpectQuery("SELECT").
		WillReturnError(errors.New("network timeout"))

	ver, err := s.service.GetVersion(context.Background(), 10, "secret", "default", "my-secret", 1)
	assert.Error(s.T(), err)
	assert.Nil(s.T(), ver)
	assert.Contains(s.T(), err.Error(), "get version 1 for secret/default/my-secret")
}

func TestConfigVersionServiceSuite(t *testing.T) {
	suite.Run(t, new(ConfigVersionServiceTestSuite))
}
