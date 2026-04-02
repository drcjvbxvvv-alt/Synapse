package services

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	permModels "github.com/clay-wangzhi/KubePolaris/internal/models"
)

// PermissionServiceTestSuite 定义权限服务测试套件
type PermissionServiceTestSuite struct {
	suite.Suite
	db      *gorm.DB
	mock    sqlmock.Sqlmock
	service *PermissionService
}

// SetupTest 每个测试前的设置
func (s *PermissionServiceTestSuite) SetupTest() {
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
	s.service = NewPermissionService(gormDB)
}

// TearDownTest 每个测试后的清理
func (s *PermissionServiceTestSuite) TearDownTest() {
	if s.db != nil {
		sqlDB, _ := s.db.DB()
		if sqlDB != nil {
			_ = sqlDB.Close()
		}
	}
}

// TestCreateUserGroup 测试创建用户组
func (s *PermissionServiceTestSuite) TestCreateUserGroup() {
	s.mock.ExpectBegin()
	s.mock.ExpectExec(`INSERT INTO .user_groups.`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.mock.ExpectCommit()

	group, err := s.service.CreateUserGroup("test-group", "Test group description")
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), group)
	assert.Equal(s.T(), "test-group", group.Name)
}

// TestGetUserGroup_Success 测试获取用户组成功
func (s *PermissionServiceTestSuite) TestGetUserGroup_Success() {
	now := time.Now()

	// 使用非常宽松的正则表达式来匹配 GORM 生成的 SQL
	// GORM First 生成: SELECT * FROM `user_groups` WHERE `user_groups`.`id` = ? AND `user_groups`.`deleted_at` IS NULL ORDER BY `user_groups`.`id` LIMIT 1
	groupRows := sqlmock.NewRows([]string{"id", "name", "description", "created_at", "updated_at", "deleted_at"}).
		AddRow(1, "test-group", "Test description", now, now, nil)

	// 使用 AnyArg 来匹配任意参数
	s.mock.ExpectQuery(`SELECT`).
		WillReturnRows(groupRows)

	// Preload Users: GORM 首先查询 user_group_members 关联表
	memberRows := sqlmock.NewRows([]string{"user_id", "user_group_id"})
	s.mock.ExpectQuery(`SELECT`).
		WillReturnRows(memberRows)

	group, err := s.service.GetUserGroup(1)
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), group)
	if group != nil {
		assert.Equal(s.T(), "test-group", group.Name)
	}
}

// TestGetUserGroup_NotFound 测试获取不存在的用户组
func (s *PermissionServiceTestSuite) TestGetUserGroup_NotFound() {
	s.mock.ExpectQuery(`SELECT`).
		WillReturnError(gorm.ErrRecordNotFound)

	group, err := s.service.GetUserGroup(999)
	assert.Error(s.T(), err)
	assert.Nil(s.T(), group)
}

// TestListUserGroups 测试列出所有用户组
func (s *PermissionServiceTestSuite) TestListUserGroups() {
	now := time.Now()
	// 主查询：获取所有用户组
	groupRows := sqlmock.NewRows([]string{"id", "name", "description", "created_at", "updated_at", "deleted_at"}).
		AddRow(1, "group-1", "Group 1", now, now, nil).
		AddRow(2, "group-2", "Group 2", now, now, nil)

	s.mock.ExpectQuery(`SELECT`).
		WillReturnRows(groupRows)

	// Preload Users 查询 - 查询用户组成员关联
	s.mock.ExpectQuery(`SELECT`).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "user_group_id"}))

	groups, err := s.service.ListUserGroups()
	assert.NoError(s.T(), err)
	assert.Len(s.T(), groups, 2)
}

// TestDeleteUserGroup_Success 测试删除用户组成功
func (s *PermissionServiceTestSuite) TestDeleteUserGroup_Success() {
	// 1. 检查关联的权限配置 - Count 查询
	s.mock.ExpectQuery(`SELECT count`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	// 2. 删除关联的用户组成员（GORM 的 Where().Delete() 会启动事务）
	s.mock.ExpectBegin()
	s.mock.ExpectExec(`DELETE FROM`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	s.mock.ExpectCommit()

	// 3. 删除用户组（GORM 软删除 - 直接执行 UPDATE deleted_at）
	s.mock.ExpectBegin()
	s.mock.ExpectExec(`UPDATE`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	s.mock.ExpectCommit()

	err := s.service.DeleteUserGroup(1)
	assert.NoError(s.T(), err)
}

// TestHasClusterAccess 测试检查集群访问权限
func (s *PermissionServiceTestSuite) TestHasClusterAccess() {
	now := time.Now()

	// 1. 先查找用户直接权限（不存在）
	s.mock.ExpectQuery(`SELECT`).
		WillReturnError(gorm.ErrRecordNotFound)

	// 2. 查找用户组权限（返回空结果）
	s.mock.ExpectQuery(`SELECT`).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "user_group_id"}))

	// 3. 查询用户信息以获取默认权限（admin 用户会有管理员权限）
	// 用户模型字段顺序：id, username, password_hash, salt, email, display_name, auth_type, status, last_login_at, last_login_ip, created_at, updated_at, deleted_at
	userRows := sqlmock.NewRows([]string{
		"id", "username", "password_hash", "salt", "email", "display_name", "auth_type", "status",
		"last_login_at", "last_login_ip", "created_at", "updated_at", "deleted_at",
	}).AddRow(
		1, "admin", "hashedpassword", "salt123", "admin@example.com", "Admin User", "local", "active",
		now, "", now, now, nil,
	)

	s.mock.ExpectQuery(`SELECT`).
		WillReturnRows(userRows)

	// 管理员应该有所有集群的访问权限
	hasAccess := s.service.HasClusterAccess(1, 1)
	assert.True(s.T(), hasAccess)
}

// TestListUsers 测试列出用户
func (s *PermissionServiceTestSuite) TestListUsers() {
	now := time.Now()
	rows := sqlmock.NewRows([]string{
		"id", "username", "password_hash", "salt", "email", "display_name", "auth_type", "status",
		"last_login_at", "last_login_ip", "created_at", "updated_at", "deleted_at",
	}).
		AddRow(1, "user1", "hash1", "salt1", "user1@example.com", "User 1", "local", "active", now, "", now, now, nil).
		AddRow(2, "user2", "hash2", "salt2", "user2@example.com", "User 2", "local", "active", now, "", now, now, nil)

	s.mock.ExpectQuery(`SELECT`).
		WillReturnRows(rows)

	users, err := s.service.ListUsers()
	assert.NoError(s.T(), err)
	assert.Len(s.T(), users, 2)
}

// TestCanPerformAction_Admin 管理員可執行所有操作
func (s *PermissionServiceTestSuite) TestCanPerformAction_Admin() {
	perm := &permModels.ClusterPermission{
		PermissionType: permModels.PermissionTypeAdmin,
		Namespaces:     `["*"]`,
	}
	assert.True(s.T(), CanPerformAction(perm, "node:drain"))
	assert.True(s.T(), CanPerformAction(perm, "pv:delete"))
	assert.True(s.T(), CanPerformAction(perm, "pod:delete"))
}

// TestCanPerformAction_Ops 運維不可操作節點/存儲/配額敏感操作
func (s *PermissionServiceTestSuite) TestCanPerformAction_Ops() {
	perm := &permModels.ClusterPermission{
		PermissionType: permModels.PermissionTypeOps,
		Namespaces:     `["*"]`,
	}
	assert.False(s.T(), CanPerformAction(perm, "node:drain"))
	assert.False(s.T(), CanPerformAction(perm, "pv:delete"))
	assert.False(s.T(), CanPerformAction(perm, "quota:update"))
	// 允許的操作
	assert.True(s.T(), CanPerformAction(perm, "deployment:update"))
	assert.True(s.T(), CanPerformAction(perm, "pod:delete"))
}

// TestCanPerformAction_Dev 開發只允許特定資源
func (s *PermissionServiceTestSuite) TestCanPerformAction_Dev() {
	perm := &permModels.ClusterPermission{
		PermissionType: permModels.PermissionTypeDev,
		Namespaces:     `["dev"]`,
	}
	assert.True(s.T(), CanPerformAction(perm, "pod:delete"))
	assert.True(s.T(), CanPerformAction(perm, "deployment:update"))
	assert.False(s.T(), CanPerformAction(perm, "node:drain"))
	assert.False(s.T(), CanPerformAction(perm, "view"))
}

// TestCanPerformAction_Readonly 只讀只允許 view/list/get
func (s *PermissionServiceTestSuite) TestCanPerformAction_Readonly() {
	perm := &permModels.ClusterPermission{
		PermissionType: permModels.PermissionTypeReadonly,
		Namespaces:     `["*"]`,
	}
	assert.True(s.T(), CanPerformAction(perm, "view"))
	assert.True(s.T(), CanPerformAction(perm, "list"))
	assert.True(s.T(), CanPerformAction(perm, "get"))
	assert.False(s.T(), CanPerformAction(perm, "pod:delete"))
	assert.False(s.T(), CanPerformAction(perm, "deployment:update"))
}

// TestHasNamespaceAccess_Wildcard 萬用字元應允許所有命名空間
func (s *PermissionServiceTestSuite) TestHasNamespaceAccess_Wildcard() {
	perm := &permModels.ClusterPermission{Namespaces: `["*"]`}
	assert.True(s.T(), HasNamespaceAccess(perm, "production"))
	assert.True(s.T(), HasNamespaceAccess(perm, "kube-system"))
}

// TestHasNamespaceAccess_Specific 指定命名空間只允許該命名空間
func (s *PermissionServiceTestSuite) TestHasNamespaceAccess_Specific() {
	perm := &permModels.ClusterPermission{Namespaces: `["dev","staging"]`}
	assert.True(s.T(), HasNamespaceAccess(perm, "dev"))
	assert.True(s.T(), HasNamespaceAccess(perm, "staging"))
	assert.False(s.T(), HasNamespaceAccess(perm, "production"))
	assert.False(s.T(), HasNamespaceAccess(perm, "kube-system"))
}

// TestCreateClusterPermission_DuplicateRejected 同一使用者不可重複設定同集群權限
func (s *PermissionServiceTestSuite) TestCreateClusterPermission_DuplicateRejected() {
	userID := uint(1)
	req := &CreateClusterPermissionRequest{
		ClusterID:      1,
		UserID:         &userID,
		PermissionType: permModels.PermissionTypeAdmin,
	}

	// 模擬已有相同設定
	s.mock.ExpectQuery(`SELECT count`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	_, err := s.service.CreateClusterPermission(req)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "已有权限配置")
}

// TestCreateClusterPermission_InvalidType 非法權限類型應被拒絕
func (s *PermissionServiceTestSuite) TestCreateClusterPermission_InvalidType() {
	userID := uint(1)
	req := &CreateClusterPermissionRequest{
		ClusterID:      1,
		UserID:         &userID,
		PermissionType: "superadmin", // invalid
	}

	_, err := s.service.CreateClusterPermission(req)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "无效的权限类型")
}

// TestCreateClusterPermission_CustomWithoutRole 自訂權限必須指定 ClusterRole
func (s *PermissionServiceTestSuite) TestCreateClusterPermission_CustomWithoutRole() {
	userID := uint(1)
	req := &CreateClusterPermissionRequest{
		ClusterID:      1,
		UserID:         &userID,
		PermissionType: permModels.PermissionTypeCustom,
		CustomRoleRef:  "", // missing
	}

	_, err := s.service.CreateClusterPermission(req)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "必须指定ClusterRole")
}

// TestCreateClusterPermission_BothUserAndGroup 不可同時指定使用者與群組
func (s *PermissionServiceTestSuite) TestCreateClusterPermission_BothUserAndGroup() {
	userID := uint(1)
	groupID := uint(2)
	req := &CreateClusterPermissionRequest{
		ClusterID:      1,
		UserID:         &userID,
		UserGroupID:    &groupID,
		PermissionType: permModels.PermissionTypeAdmin,
	}

	_, err := s.service.CreateClusterPermission(req)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "不能同时指定用户和用户组")
}

// TestPermissionServiceSuite 运行测试套件
func TestPermissionServiceSuite(t *testing.T) {
	suite.Run(t, new(PermissionServiceTestSuite))
}
