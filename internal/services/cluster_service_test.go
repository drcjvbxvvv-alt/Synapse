package services

import (
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"kubepolaris/internal/models"
)

// ClusterServiceTestSuite 定义集群服务测试套件
type ClusterServiceTestSuite struct {
	suite.Suite
	db      *gorm.DB
	mock    sqlmock.Sqlmock
	service *ClusterService
}

// SetupTest 每个测试前的设置
func (s *ClusterServiceTestSuite) SetupTest() {
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
	s.service = NewClusterService(gormDB)
}

// TearDownTest 每个测试后的清理
func (s *ClusterServiceTestSuite) TearDownTest() {
	if s.db != nil {
		sqlDB, _ := s.db.DB()
		if sqlDB != nil {
			sqlDB.Close()
		}
	}
}

// TestCreateCluster 测试创建集群
func (s *ClusterServiceTestSuite) TestCreateCluster() {
	cluster := &models.Cluster{
		Name:      "test-cluster",
		APIServer: "https://kubernetes.example.com:6443",
		Status:    "pending",
	}

	s.mock.ExpectBegin()
	s.mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `clusters`")).
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.mock.ExpectCommit()

	err := s.service.CreateCluster(cluster)
	assert.NoError(s.T(), err)
	assert.NotZero(s.T(), cluster.ID)
	assert.NotZero(s.T(), cluster.CreatedAt)
	assert.NotZero(s.T(), cluster.UpdatedAt)
}

// TestCreateCluster_EmptyName 测试创建空名称集群
func (s *ClusterServiceTestSuite) TestCreateCluster_DBError() {
	cluster := &models.Cluster{
		Name:      "test-cluster",
		APIServer: "https://kubernetes.example.com:6443",
	}

	s.mock.ExpectBegin()
	s.mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `clusters`")).
		WillReturnError(gorm.ErrDuplicatedKey)
	s.mock.ExpectRollback()

	err := s.service.CreateCluster(cluster)
	assert.Error(s.T(), err)
}

// TestGetCluster_Success 测试获取集群成功
func (s *ClusterServiceTestSuite) TestGetCluster_Success() {
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

	cluster, err := s.service.GetCluster(1)
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), cluster)
	assert.Equal(s.T(), "test-cluster", cluster.Name)
	assert.Equal(s.T(), "https://kubernetes.example.com:6443", cluster.APIServer)
}

// TestGetCluster_NotFound 测试获取不存在的集群
func (s *ClusterServiceTestSuite) TestGetCluster_NotFound() {
	s.mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `clusters` WHERE `clusters`.`id` = ?")).
		WithArgs(999).
		WillReturnError(gorm.ErrRecordNotFound)

	cluster, err := s.service.GetCluster(999)
	assert.Error(s.T(), err)
	assert.Nil(s.T(), cluster)
	assert.Contains(s.T(), err.Error(), "集群不存在")
}

// TestGetAllClusters_Success 测试获取所有集群成功
func (s *ClusterServiceTestSuite) TestGetAllClusters_Success() {
	now := time.Now()
	rows := sqlmock.NewRows([]string{
		"id", "name", "api_server", "kube_config", "version", "status",
		"description", "environment", "region", "labels", "monitoring_config",
		"alert_manager_config", "created_at", "updated_at", "last_heartbeat",
	}).
		AddRow(1, "cluster-1", "https://k8s1.example.com:6443", "config1",
			"v1.28.0", "connected", "", "", "", "{}", "{}", "{}", now, now, now).
		AddRow(2, "cluster-2", "https://k8s2.example.com:6443", "config2",
			"v1.29.0", "connected", "", "", "", "{}", "{}", "{}", now, now, now)

	s.mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `clusters`")).
		WillReturnRows(rows)

	clusters, err := s.service.GetAllClusters()
	assert.NoError(s.T(), err)
	assert.Len(s.T(), clusters, 2)
	assert.Equal(s.T(), "cluster-1", clusters[0].Name)
	assert.Equal(s.T(), "cluster-2", clusters[1].Name)
}

// TestGetAllClusters_Empty 测试获取空集群列表
func (s *ClusterServiceTestSuite) TestGetAllClusters_Empty() {
	rows := sqlmock.NewRows([]string{
		"id", "name", "api_server", "kube_config", "version", "status",
		"description", "environment", "region", "labels", "monitoring_config",
		"alert_manager_config", "created_at", "updated_at", "last_heartbeat",
	})

	s.mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `clusters`")).
		WillReturnRows(rows)

	clusters, err := s.service.GetAllClusters()
	assert.NoError(s.T(), err)
	assert.Len(s.T(), clusters, 0)
}

// TestUpdateClusterStatus_Success 测试更新集群状态成功
func (s *ClusterServiceTestSuite) TestUpdateClusterStatus_Success() {
	s.mock.ExpectBegin()
	s.mock.ExpectExec(regexp.QuoteMeta("UPDATE `clusters`")).
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.mock.ExpectCommit()

	err := s.service.UpdateClusterStatus(1, "connected", "v1.29.0")
	assert.NoError(s.T(), err)
}

// TestDeleteCluster_Success 测试删除集群成功
func (s *ClusterServiceTestSuite) TestDeleteCluster_Success() {
	s.mock.ExpectBegin()
	s.mock.ExpectExec(regexp.QuoteMeta("DELETE FROM `clusters` WHERE `clusters`.`id` = ?")).
		WithArgs(1).
		WillReturnResult(sqlmock.NewResult(0, 1))
	s.mock.ExpectCommit()

	err := s.service.DeleteCluster(1)
	assert.NoError(s.T(), err)
}

// TestDeleteCluster_NotFound 测试删除不存在的集群
func (s *ClusterServiceTestSuite) TestDeleteCluster_NotFound() {
	s.mock.ExpectBegin()
	s.mock.ExpectExec(regexp.QuoteMeta("DELETE FROM `clusters` WHERE `clusters`.`id` = ?")).
		WithArgs(999).
		WillReturnResult(sqlmock.NewResult(0, 0))
	s.mock.ExpectCommit()

	err := s.service.DeleteCluster(999)
	// 即使没有删除任何行，GORM 也不会返回错误
	assert.NoError(s.T(), err)
}

// TestClusterServiceSuite 运行测试套件
func TestClusterServiceSuite(t *testing.T) {
	suite.Run(t, new(ClusterServiceTestSuite))
}
