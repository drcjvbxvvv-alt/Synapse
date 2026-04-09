package repositories

import (
	"context"

	"gorm.io/gorm"
)

// Repository is the generic data-access interface for a single model type T.
//
// T must be a plain struct type (not a pointer) — callers pass *T where an
// entity is needed, matching GORM's conventions. For example:
//
//	type ClusterRepository interface {
//	    Repository[models.Cluster]
//	    FindByName(ctx context.Context, name string) (*models.Cluster, error)
//	}
//
// Implementation notes:
//   - Every method MUST call .WithContext(ctx) on the underlying *gorm.DB.
//     BaseRepository enforces this; custom implementations must do the same.
//   - Get / FindOne return ErrNotFound when the row is absent. Find returns
//     an empty slice (no error) when nothing matches.
//   - Write methods (UpdateFields, DeleteWhere) return rows-affected so the
//     caller can decide whether "0 rows" is an error or not.
type Repository[T any] interface {
	// ---- Read ----

	// Get fetches a single row by primary key. Returns ErrNotFound on miss.
	Get(ctx context.Context, id uint) (*T, error)

	// FindOne fetches the first row matching the given conditions. The conds
	// follow GORM's variadic convention: ("name = ?", name) or a struct.
	// Returns ErrNotFound on miss.
	FindOne(ctx context.Context, conds ...any) (*T, error)

	// Find fetches all rows matching the given conditions. Returns an empty
	// slice (never nil) when nothing matches — callers should not check
	// ErrNotFound here.
	Find(ctx context.Context, conds ...any) ([]*T, error)

	// List fetches a paginated slice plus total count. opts.Page is 1-based;
	// pass an empty ListOptions{} to list everything without paging.
	List(ctx context.Context, opts ListOptions) ([]*T, int64, error)

	// Count returns the number of rows matching the given conditions.
	Count(ctx context.Context, conds ...any) (int64, error)

	// Exists is a shortcut for Count > 0.
	Exists(ctx context.Context, conds ...any) (bool, error)

	// ---- Write ----

	// Create inserts a new row. The entity's ID and timestamps are populated
	// on success.
	Create(ctx context.Context, entity *T) error

	// Update persists all fields of the entity (GORM Save semantics). Zero
	// values ARE written — if you want partial updates, use UpdateFields.
	Update(ctx context.Context, entity *T) error

	// UpdateFields applies a partial column update to the row with the given
	// ID. Returns rows-affected so callers can distinguish "nothing updated"
	// from a real error.
	UpdateFields(ctx context.Context, id uint, fields map[string]any) (int64, error)

	// Delete soft-deletes the row with the given ID (assuming the model has
	// gorm.DeletedAt). Returns no error if the row does not exist.
	Delete(ctx context.Context, id uint) error

	// DeleteWhere soft-deletes all rows matching the given conditions and
	// returns rows-affected.
	DeleteWhere(ctx context.Context, conds ...any) (int64, error)

	// ---- Unit of work ----

	// WithTx returns a new Repository[T] bound to the given transaction.
	// Used inside Transaction callbacks to reuse the tx across repos.
	WithTx(tx *gorm.DB) Repository[T]

	// Transaction runs fn inside a GORM transaction. The callback receives
	// the raw *gorm.DB so it can coordinate operations across multiple
	// repositories (e.g. delete Cluster + ClusterPermissions + TerminalSessions
	// in one atomic unit). Use WithTx(tx) inside fn to get typed repos.
	Transaction(ctx context.Context, fn func(tx *gorm.DB) error) error

	// ---- Escape hatch ----

	// DB returns a context-scoped *gorm.DB for queries that don't fit the
	// generic API (complex joins, group-by, UNION, etc). Use sparingly —
	// every such call-site is a candidate for a domain-specific Repository
	// method.
	DB(ctx context.Context) *gorm.DB
}

// Order describes a single ORDER BY clause.
type Order struct {
	Column string // column name
	Desc   bool   // true for DESC, false for ASC
}

// ListOptions bundles common list-query parameters.
//
// Zero value lists everything without pagination or ordering. Set Page and
// PageSize together for pagination (1-based page index).
type ListOptions struct {
	// Page is 1-based. 0 means "no pagination".
	Page int
	// PageSize is rows per page. 0 means "no pagination".
	PageSize int
	// OrderBy is applied in slice order. Empty slice = no explicit ORDER BY.
	OrderBy []Order
	// Where is an optional raw WHERE clause + args, same convention as
	// GORM's Where(query, args...). Pass an empty Where to match everything.
	Where WhereClause
	// Preloads lists association field names to preload (gorm.Preload).
	Preloads []string
}

// WhereClause holds a raw GORM where condition. Use zero value for "no
// condition". When Query is non-empty, it is applied via db.Where(Query, Args...).
type WhereClause struct {
	Query any
	Args  []any
}
