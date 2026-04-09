package repositories

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
)

// BaseRepository is the generic GORM-backed implementation of Repository[T].
//
// Domain repositories should embed *BaseRepository[T] to inherit the full
// CRUD surface, then add their own methods for domain queries:
//
//	type ClusterRepository struct {
//	    *repositories.BaseRepository[models.Cluster]
//	}
//
//	func NewClusterRepository(db *gorm.DB) *ClusterRepository {
//	    return &ClusterRepository{
//	        BaseRepository: repositories.NewBaseRepository[models.Cluster](db),
//	    }
//	}
//
//	func (r *ClusterRepository) FindByName(ctx context.Context, name string) (*models.Cluster, error) {
//	    return r.FindOne(ctx, "name = ?", name)
//	}
//
// The base unconditionally attaches .WithContext(ctx) to every query. This is
// the single most important invariant of the repository layer and must not
// be bypassed.
type BaseRepository[T any] struct {
	db *gorm.DB
}

// NewBaseRepository constructs a BaseRepository[T] bound to the given DB
// handle. The handle should be the application-wide *gorm.DB; pass a tx in
// when constructing a transactional repo (usually via WithTx).
func NewBaseRepository[T any](db *gorm.DB) *BaseRepository[T] {
	return &BaseRepository[T]{db: db}
}

// session returns r.db with ctx attached. Every method MUST start here.
func (r *BaseRepository[T]) session(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx)
}

// ---- Read methods ----

// Get fetches a single row by primary key.
func (r *BaseRepository[T]) Get(ctx context.Context, id uint) (*T, error) {
	if id == 0 {
		return nil, fmt.Errorf("%w: id must be non-zero", ErrInvalidArgument)
	}
	var entity T
	err := r.session(ctx).First(&entity, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("repository get: %w", err)
	}
	return &entity, nil
}

// FindOne fetches the first row matching the given conditions.
func (r *BaseRepository[T]) FindOne(ctx context.Context, conds ...any) (*T, error) {
	var entity T
	err := r.session(ctx).First(&entity, conds...).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("repository find-one: %w", err)
	}
	return &entity, nil
}

// Find fetches all rows matching the given conditions. Returns an empty
// slice (never nil) when nothing matches.
func (r *BaseRepository[T]) Find(ctx context.Context, conds ...any) ([]*T, error) {
	entities := make([]*T, 0)
	if err := r.session(ctx).Find(&entities, conds...).Error; err != nil {
		return nil, fmt.Errorf("repository find: %w", err)
	}
	return entities, nil
}

// List fetches a paginated slice plus total count.
func (r *BaseRepository[T]) List(ctx context.Context, opts ListOptions) ([]*T, int64, error) {
	// Build base query from opts.Where, counted before pagination is applied.
	var zero T
	query := r.session(ctx).Model(&zero)
	if opts.Where.Query != nil {
		query = query.Where(opts.Where.Query, opts.Where.Args...)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("repository list count: %w", err)
	}

	// Apply preloads + ordering + pagination on a fresh chain to avoid
	// polluting the count query.
	data := r.session(ctx).Model(&zero)
	if opts.Where.Query != nil {
		data = data.Where(opts.Where.Query, opts.Where.Args...)
	}
	for _, preload := range opts.Preloads {
		data = data.Preload(preload)
	}
	for _, order := range opts.OrderBy {
		direction := "ASC"
		if order.Desc {
			direction = "DESC"
		}
		data = data.Order(order.Column + " " + direction)
	}
	if opts.Page > 0 && opts.PageSize > 0 {
		data = data.Offset((opts.Page - 1) * opts.PageSize).Limit(opts.PageSize)
	}

	entities := make([]*T, 0)
	if err := data.Find(&entities).Error; err != nil {
		return nil, 0, fmt.Errorf("repository list find: %w", err)
	}
	return entities, total, nil
}

// Count returns the number of rows matching the given conditions.
func (r *BaseRepository[T]) Count(ctx context.Context, conds ...any) (int64, error) {
	var zero T
	var total int64
	query := r.session(ctx).Model(&zero)
	if len(conds) > 0 {
		query = query.Where(conds[0], conds[1:]...)
	}
	if err := query.Count(&total).Error; err != nil {
		return 0, fmt.Errorf("repository count: %w", err)
	}
	return total, nil
}

// Exists is a shortcut for Count > 0.
func (r *BaseRepository[T]) Exists(ctx context.Context, conds ...any) (bool, error) {
	count, err := r.Count(ctx, conds...)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// ---- Write methods ----

// Create inserts a new row.
func (r *BaseRepository[T]) Create(ctx context.Context, entity *T) error {
	if entity == nil {
		return fmt.Errorf("%w: entity must not be nil", ErrInvalidArgument)
	}
	if err := r.session(ctx).Create(entity).Error; err != nil {
		return fmt.Errorf("repository create: %w", err)
	}
	return nil
}

// Update persists all fields of the entity (GORM Save semantics).
func (r *BaseRepository[T]) Update(ctx context.Context, entity *T) error {
	if entity == nil {
		return fmt.Errorf("%w: entity must not be nil", ErrInvalidArgument)
	}
	if err := r.session(ctx).Save(entity).Error; err != nil {
		return fmt.Errorf("repository update: %w", err)
	}
	return nil
}

// UpdateFields applies a partial column update to the row with the given ID.
func (r *BaseRepository[T]) UpdateFields(
	ctx context.Context, id uint, fields map[string]any,
) (int64, error) {
	if id == 0 {
		return 0, fmt.Errorf("%w: id must be non-zero", ErrInvalidArgument)
	}
	if len(fields) == 0 {
		return 0, fmt.Errorf("%w: fields must not be empty", ErrInvalidArgument)
	}
	var zero T
	result := r.session(ctx).Model(&zero).Where("id = ?", id).Updates(fields)
	if result.Error != nil {
		return 0, fmt.Errorf("repository update-fields: %w", result.Error)
	}
	return result.RowsAffected, nil
}

// Delete soft-deletes the row with the given ID.
func (r *BaseRepository[T]) Delete(ctx context.Context, id uint) error {
	if id == 0 {
		return fmt.Errorf("%w: id must be non-zero", ErrInvalidArgument)
	}
	var zero T
	if err := r.session(ctx).Delete(&zero, id).Error; err != nil {
		return fmt.Errorf("repository delete: %w", err)
	}
	return nil
}

// DeleteWhere soft-deletes all rows matching the given conditions.
func (r *BaseRepository[T]) DeleteWhere(ctx context.Context, conds ...any) (int64, error) {
	if len(conds) == 0 {
		return 0, fmt.Errorf("%w: delete-where requires at least one condition", ErrInvalidArgument)
	}
	var zero T
	result := r.session(ctx).Where(conds[0], conds[1:]...).Delete(&zero)
	if result.Error != nil {
		return 0, fmt.Errorf("repository delete-where: %w", result.Error)
	}
	return result.RowsAffected, nil
}

// ---- Unit of work ----

// WithTx returns a new Repository[T] bound to the given transaction.
func (r *BaseRepository[T]) WithTx(tx *gorm.DB) Repository[T] {
	return &BaseRepository[T]{db: tx}
}

// Transaction runs fn inside a GORM transaction. The callback receives the
// raw *gorm.DB so it can coordinate operations across multiple repositories.
func (r *BaseRepository[T]) Transaction(
	ctx context.Context, fn func(tx *gorm.DB) error,
) error {
	return r.session(ctx).Transaction(fn)
}

// DB returns a context-scoped *gorm.DB for queries that don't fit the
// generic API. Use sparingly — prefer domain-specific repo methods.
func (r *BaseRepository[T]) DB(ctx context.Context) *gorm.DB {
	return r.session(ctx)
}

// Compile-time check that BaseRepository[T] implements Repository[T].
var _ Repository[struct{}] = (*BaseRepository[struct{}])(nil)
