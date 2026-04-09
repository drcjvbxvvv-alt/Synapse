package repositories

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/shaia/Synapse/internal/models"
)

// PermissionRepository is the data-access interface for cluster permissions
// and user groups. The two models are bundled into a single repository
// because every write on ClusterPermission is tightly coupled with UserGroup
// state (delete-group-checks, group-scoped queries, transaction-spanning
// writes), and splitting them would force services to coordinate two
// repositories where one is enough.
type PermissionRepository interface {
	Repository[models.ClusterPermission]

	// ---- ClusterPermission-specific reads ----

	// GetWithRelations fetches a permission row by ID with User, UserGroup
	// and Cluster preloaded. Returns ErrNotFound on miss.
	GetWithRelations(ctx context.Context, id uint) (*models.ClusterPermission, error)

	// FindByClusterUser returns the permission row for (clusterID, userID).
	// Returns ErrNotFound when no row matches — callers use this to
	// distinguish "no explicit permission" from DB errors.
	FindByClusterUser(ctx context.Context, clusterID, userID uint) (*models.ClusterPermission, error)

	// FindByClusterGroups returns the highest-priority permission the given
	// groups hold in the given cluster, ordered by permission_type rank.
	// Returns ErrNotFound when none of the groups has a row.
	FindByClusterGroups(ctx context.Context, clusterID uint, groupIDs []uint) (*models.ClusterPermission, error)

	// ListByCluster returns every permission row for the given cluster, with
	// User and UserGroup preloaded. clusterID = 0 means "all clusters".
	ListByCluster(ctx context.Context, clusterID uint) ([]*models.ClusterPermission, error)

	// ListAllWithRelations returns every permission row with User, UserGroup
	// and Cluster preloaded. Used by the admin list endpoint.
	ListAllWithRelations(ctx context.Context) ([]*models.ClusterPermission, error)

	// ListUserDirectAndGroupWithCluster returns every permission row where
	// user_id = userID OR user_group_id IN groupIDs, with Cluster preloaded.
	ListUserDirectAndGroupWithCluster(
		ctx context.Context, userID uint, groupIDs []uint,
	) ([]*models.ClusterPermission, error)

	// ExistsForClusterUser reports whether a permission already exists for
	// (clusterID, userID). Used to reject duplicate Create.
	ExistsForClusterUser(ctx context.Context, clusterID, userID uint) (bool, error)

	// ExistsForClusterGroup reports whether a permission already exists for
	// (clusterID, groupID). Used to reject duplicate Create.
	ExistsForClusterGroup(ctx context.Context, clusterID, groupID uint) (bool, error)

	// CountAdminByUser returns the number of permission rows granting admin
	// directly to the given user. Used by GetUserAccessibleClusterIDs.
	CountAdminByUser(ctx context.Context, userID uint) (int64, error)

	// CountAdminByGroups returns the number of permission rows granting
	// admin to any of the given groups.
	CountAdminByGroups(ctx context.Context, groupIDs []uint) (int64, error)

	// DistinctClusterIDsByUser returns the set of cluster IDs the user has
	// an explicit permission row for (directly or via a group).
	DistinctClusterIDsByUser(
		ctx context.Context, userID uint, groupIDs []uint,
	) ([]uint, error)

	// ---- ClusterPermission writes ----

	// CreatePermission inserts a new permission row and translates the
	// MySQL duplicate-key error into ErrAlreadyExists.
	CreatePermission(ctx context.Context, p *models.ClusterPermission) error

	// BatchDeletePermissions hard-deletes every permission row whose ID is
	// in the slice. Returns rows-affected.
	BatchDeletePermissions(ctx context.Context, ids []uint) (int64, error)

	// ---- UserGroup reads ----

	// GetUserGroupWithUsers fetches a user group by ID with Users preloaded.
	// Returns ErrNotFound on miss.
	GetUserGroupWithUsers(ctx context.Context, id uint) (*models.UserGroup, error)

	// GetUserGroup fetches a user group by ID. Returns ErrNotFound on miss.
	GetUserGroup(ctx context.Context, id uint) (*models.UserGroup, error)

	// ListUserGroupsWithUsers returns every user group with Users preloaded
	// (selecting only the columns used by the admin UI).
	ListUserGroupsWithUsers(ctx context.Context) ([]*models.UserGroup, error)

	// CountPermissionsForGroup returns the number of permission rows bound
	// to the given group. Used by DeleteUserGroup to block removal of
	// groups that still have permission rows attached.
	CountPermissionsForGroup(ctx context.Context, groupID uint) (int64, error)

	// ListGroupIDsForUser returns the IDs of every group the user belongs
	// to.
	ListGroupIDsForUser(ctx context.Context, userID uint) ([]uint, error)

	// ---- UserGroup writes ----

	// CreateUserGroup inserts a new user group and translates the MySQL
	// duplicate-key error into ErrAlreadyExists.
	CreateUserGroup(ctx context.Context, group *models.UserGroup) error

	// UpdateUserGroup saves the entity in place (GORM Save semantics).
	UpdateUserGroup(ctx context.Context, group *models.UserGroup) error

	// DeleteUserGroupTx deletes the group and its members inside a single
	// transaction. The caller must have already verified that no permission
	// rows reference the group.
	DeleteUserGroupTx(ctx context.Context, id uint) error

	// ---- UserGroupMember writes ----

	// AddUserToGroup creates a (userID, groupID) membership row. Idempotent
	// — a second call with the same pair returns nil without error.
	AddUserToGroup(ctx context.Context, userID, groupID uint) error

	// RemoveUserFromGroup deletes the (userID, groupID) membership row if
	// it exists.
	RemoveUserFromGroup(ctx context.Context, userID, groupID uint) error

	// DeleteMembershipsByUser deletes every membership row for the user.
	// Used during user deletion cleanup.
	DeleteMembershipsByUser(ctx context.Context, userID uint) (int64, error)

	// DeletePermissionsByUser deletes every cluster permission row belonging
	// directly to the user. Used during user deletion cleanup.
	DeletePermissionsByUser(ctx context.Context, userID uint) (int64, error)
}

// permissionRepository is the concrete GORM-backed implementation.
type permissionRepository struct {
	*BaseRepository[models.ClusterPermission]
}

// NewPermissionRepository constructs a PermissionRepository bound to the given
// DB handle.
func NewPermissionRepository(db *gorm.DB) PermissionRepository {
	return &permissionRepository{
		BaseRepository: NewBaseRepository[models.ClusterPermission](db),
	}
}

// ---- ClusterPermission reads ----

func (r *permissionRepository) GetWithRelations(
	ctx context.Context, id uint,
) (*models.ClusterPermission, error) {
	if id == 0 {
		return nil, fmt.Errorf("%w: id must be non-zero", ErrInvalidArgument)
	}
	var p models.ClusterPermission
	err := r.DB(ctx).
		Preload("User").
		Preload("UserGroup").
		Preload("Cluster").
		First(&p, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("permission get-with-relations: %w", err)
	}
	return &p, nil
}

func (r *permissionRepository) FindByClusterUser(
	ctx context.Context, clusterID, userID uint,
) (*models.ClusterPermission, error) {
	return r.FindOne(ctx, "cluster_id = ? AND user_id = ?", clusterID, userID)
}

func (r *permissionRepository) FindByClusterGroups(
	ctx context.Context, clusterID uint, groupIDs []uint,
) (*models.ClusterPermission, error) {
	if len(groupIDs) == 0 {
		return nil, ErrNotFound
	}
	var p models.ClusterPermission
	err := r.DB(ctx).
		Where("cluster_id = ? AND user_group_id IN ?", clusterID, groupIDs).
		Order("FIELD(permission_type, 'admin', 'ops', 'dev', 'readonly', 'custom')").
		First(&p).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("permission find-by-cluster-groups: %w", err)
	}
	return &p, nil
}

func (r *permissionRepository) ListByCluster(
	ctx context.Context, clusterID uint,
) ([]*models.ClusterPermission, error) {
	q := r.DB(ctx).Preload("User").Preload("UserGroup")
	if clusterID > 0 {
		q = q.Where("cluster_id = ?", clusterID)
	}
	items := make([]*models.ClusterPermission, 0)
	if err := q.Find(&items).Error; err != nil {
		return nil, fmt.Errorf("permission list-by-cluster: %w", err)
	}
	return items, nil
}

func (r *permissionRepository) ListAllWithRelations(
	ctx context.Context,
) ([]*models.ClusterPermission, error) {
	items := make([]*models.ClusterPermission, 0)
	err := r.DB(ctx).
		Preload("User").
		Preload("UserGroup").
		Preload("Cluster").
		Find(&items).Error
	if err != nil {
		return nil, fmt.Errorf("permission list-all: %w", err)
	}
	return items, nil
}

func (r *permissionRepository) ListUserDirectAndGroupWithCluster(
	ctx context.Context, userID uint, groupIDs []uint,
) ([]*models.ClusterPermission, error) {
	items := make([]*models.ClusterPermission, 0)
	q := r.DB(ctx).Preload("Cluster")
	if len(groupIDs) > 0 {
		q = q.Where("user_id = ? OR user_group_id IN ?", userID, groupIDs)
	} else {
		q = q.Where("user_id = ?", userID)
	}
	if err := q.Find(&items).Error; err != nil {
		return nil, fmt.Errorf("permission list-user-direct-and-group: %w", err)
	}
	return items, nil
}

func (r *permissionRepository) ExistsForClusterUser(
	ctx context.Context, clusterID, userID uint,
) (bool, error) {
	return r.Exists(ctx, "cluster_id = ? AND user_id = ?", clusterID, userID)
}

func (r *permissionRepository) ExistsForClusterGroup(
	ctx context.Context, clusterID, groupID uint,
) (bool, error) {
	return r.Exists(ctx, "cluster_id = ? AND user_group_id = ?", clusterID, groupID)
}

func (r *permissionRepository) CountAdminByUser(
	ctx context.Context, userID uint,
) (int64, error) {
	return r.Count(ctx,
		"user_id = ? AND permission_type = ?", userID, models.PermissionTypeAdmin,
	)
}

func (r *permissionRepository) CountAdminByGroups(
	ctx context.Context, groupIDs []uint,
) (int64, error) {
	if len(groupIDs) == 0 {
		return 0, nil
	}
	return r.Count(ctx,
		"user_group_id IN ? AND permission_type = ?", groupIDs, models.PermissionTypeAdmin,
	)
}

func (r *permissionRepository) DistinctClusterIDsByUser(
	ctx context.Context, userID uint, groupIDs []uint,
) ([]uint, error) {
	q := r.DB(ctx).Model(&models.ClusterPermission{})
	if len(groupIDs) > 0 {
		q = q.Where("user_id = ? OR user_group_id IN ?", userID, groupIDs)
	} else {
		q = q.Where("user_id = ?", userID)
	}
	var ids []uint
	if err := q.Distinct().Pluck("cluster_id", &ids).Error; err != nil {
		return nil, fmt.Errorf("permission distinct-cluster-ids: %w", err)
	}
	return ids, nil
}

// ---- ClusterPermission writes ----

func (r *permissionRepository) CreatePermission(
	ctx context.Context, p *models.ClusterPermission,
) error {
	if p == nil {
		return fmt.Errorf("%w: permission must not be nil", ErrInvalidArgument)
	}
	if err := r.DB(ctx).Create(p).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return ErrAlreadyExists
		}
		return fmt.Errorf("permission create: %w", err)
	}
	return nil
}

func (r *permissionRepository) BatchDeletePermissions(
	ctx context.Context, ids []uint,
) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	result := r.DB(ctx).Delete(&models.ClusterPermission{}, ids)
	if result.Error != nil {
		return 0, fmt.Errorf("permission batch-delete: %w", result.Error)
	}
	return result.RowsAffected, nil
}

// ---- UserGroup reads ----

func (r *permissionRepository) GetUserGroupWithUsers(
	ctx context.Context, id uint,
) (*models.UserGroup, error) {
	if id == 0 {
		return nil, fmt.Errorf("%w: id must be non-zero", ErrInvalidArgument)
	}
	var g models.UserGroup
	err := r.DB(ctx).Preload("Users").First(&g, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("usergroup get-with-users: %w", err)
	}
	return &g, nil
}

func (r *permissionRepository) GetUserGroup(
	ctx context.Context, id uint,
) (*models.UserGroup, error) {
	if id == 0 {
		return nil, fmt.Errorf("%w: id must be non-zero", ErrInvalidArgument)
	}
	var g models.UserGroup
	err := r.DB(ctx).First(&g, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("usergroup get: %w", err)
	}
	return &g, nil
}

func (r *permissionRepository) ListUserGroupsWithUsers(
	ctx context.Context,
) ([]*models.UserGroup, error) {
	groups := make([]*models.UserGroup, 0)
	err := r.DB(ctx).
		Preload("Users", func(db *gorm.DB) *gorm.DB {
			return db.Select("id, username, email, display_name")
		}).
		Find(&groups).Error
	if err != nil {
		return nil, fmt.Errorf("usergroup list: %w", err)
	}
	return groups, nil
}

func (r *permissionRepository) CountPermissionsForGroup(
	ctx context.Context, groupID uint,
) (int64, error) {
	return r.Count(ctx, "user_group_id = ?", groupID)
}

func (r *permissionRepository) ListGroupIDsForUser(
	ctx context.Context, userID uint,
) ([]uint, error) {
	var ids []uint
	err := r.DB(ctx).
		Model(&models.UserGroupMember{}).
		Where("user_id = ?", userID).
		Pluck("user_group_id", &ids).Error
	if err != nil {
		return nil, fmt.Errorf("usergroupmember list-groups-for-user: %w", err)
	}
	return ids, nil
}

// ---- UserGroup writes ----

func (r *permissionRepository) CreateUserGroup(
	ctx context.Context, group *models.UserGroup,
) error {
	if group == nil {
		return fmt.Errorf("%w: group must not be nil", ErrInvalidArgument)
	}
	if err := r.DB(ctx).Create(group).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return ErrAlreadyExists
		}
		return fmt.Errorf("usergroup create: %w", err)
	}
	return nil
}

func (r *permissionRepository) UpdateUserGroup(
	ctx context.Context, group *models.UserGroup,
) error {
	if group == nil {
		return fmt.Errorf("%w: group must not be nil", ErrInvalidArgument)
	}
	if err := r.DB(ctx).Save(group).Error; err != nil {
		return fmt.Errorf("usergroup update: %w", err)
	}
	return nil
}

func (r *permissionRepository) DeleteUserGroupTx(
	ctx context.Context, id uint,
) error {
	if id == 0 {
		return fmt.Errorf("%w: id must be non-zero", ErrInvalidArgument)
	}
	return r.DB(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_group_id = ?", id).
			Delete(&models.UserGroupMember{}).Error; err != nil {
			return fmt.Errorf("delete usergroup members: %w", err)
		}
		if err := tx.Delete(&models.UserGroup{}, id).Error; err != nil {
			return fmt.Errorf("delete usergroup: %w", err)
		}
		return nil
	})
}

// ---- UserGroupMember writes ----

func (r *permissionRepository) AddUserToGroup(
	ctx context.Context, userID, groupID uint,
) error {
	// Idempotent: skip if the pair already exists.
	var count int64
	err := r.DB(ctx).
		Model(&models.UserGroupMember{}).
		Where("user_id = ? AND user_group_id = ?", userID, groupID).
		Count(&count).Error
	if err != nil {
		return fmt.Errorf("usergroupmember exists-check: %w", err)
	}
	if count > 0 {
		return nil
	}
	member := &models.UserGroupMember{UserID: userID, UserGroupID: groupID}
	if err := r.DB(ctx).Create(member).Error; err != nil {
		return fmt.Errorf("usergroupmember create: %w", err)
	}
	return nil
}

func (r *permissionRepository) RemoveUserFromGroup(
	ctx context.Context, userID, groupID uint,
) error {
	err := r.DB(ctx).
		Where("user_id = ? AND user_group_id = ?", userID, groupID).
		Delete(&models.UserGroupMember{}).Error
	if err != nil {
		return fmt.Errorf("usergroupmember delete: %w", err)
	}
	return nil
}

func (r *permissionRepository) DeleteMembershipsByUser(
	ctx context.Context, userID uint,
) (int64, error) {
	result := r.DB(ctx).
		Where("user_id = ?", userID).
		Delete(&models.UserGroupMember{})
	if result.Error != nil {
		return 0, fmt.Errorf("usergroupmember delete-by-user: %w", result.Error)
	}
	return result.RowsAffected, nil
}

func (r *permissionRepository) DeletePermissionsByUser(
	ctx context.Context, userID uint,
) (int64, error) {
	result := r.DB(ctx).
		Where("user_id = ?", userID).
		Delete(&models.ClusterPermission{})
	if result.Error != nil {
		return 0, fmt.Errorf("permission delete-by-user: %w", result.Error)
	}
	return result.RowsAffected, nil
}
