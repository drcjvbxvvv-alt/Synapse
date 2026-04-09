package repositories

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/shaia/Synapse/internal/models"
)

// UserRepository is the data-access interface for models.User.
//
// It embeds the generic Repository[models.User] surface and adds the domain
// finders used by UserService and the auth layer. Services depend on this
// interface so they can be unit-tested with a fake.
type UserRepository interface {
	Repository[models.User]

	// FindByUsername fetches a user by their unique username. Returns
	// ErrNotFound when no user matches.
	FindByUsername(ctx context.Context, username string) (*models.User, error)

	// ListPaged returns a slice of users + total count, filtered by the
	// given ListUsersFilter. The returned slice contains pointers to match
	// the generic Repository convention; service code converts to values
	// before returning to handlers.
	ListPaged(ctx context.Context, filter ListUsersFilter) ([]*models.User, int64, error)
}

// ListUsersFilter bundles the optional filters accepted by ListPaged.
//
// Zero value lists every user on page 1 with no filters. Page and PageSize
// are 1-based; pass 0 to either to disable pagination.
type ListUsersFilter struct {
	Page     int
	PageSize int
	// Search is a case-insensitive LIKE match applied to username,
	// display_name and email.
	Search string
	// Status filters on users.status (active / inactive / locked). Empty
	// string = no filter.
	Status string
	// AuthType filters on users.auth_type (local / ldap). Empty string =
	// no filter.
	AuthType string
}

// userRepository is the concrete GORM-backed implementation.
type userRepository struct {
	*BaseRepository[models.User]
}

// NewUserRepository constructs a UserRepository bound to the given DB.
func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{
		BaseRepository: NewBaseRepository[models.User](db),
	}
}

// FindByUsername implements UserRepository.
func (r *userRepository) FindByUsername(ctx context.Context, username string) (*models.User, error) {
	if username == "" {
		return nil, fmt.Errorf("%w: username must not be empty", ErrInvalidArgument)
	}
	return r.FindOne(ctx, "username = ?", username)
}

// ListPaged implements UserRepository.
//
// Translates the domain-shaped filter into a ListOptions call on the base
// repository, so pagination and counting stay consistent with the rest of
// the codebase.
func (r *userRepository) ListPaged(
	ctx context.Context, filter ListUsersFilter,
) ([]*models.User, int64, error) {
	opts := ListOptions{
		Page:     filter.Page,
		PageSize: filter.PageSize,
		OrderBy:  []Order{{Column: "id", Desc: false}},
	}

	var (
		whereParts []string
		args       []any
	)
	if filter.Search != "" {
		like := "%" + filter.Search + "%"
		whereParts = append(whereParts,
			"(username LIKE ? OR display_name LIKE ? OR email LIKE ?)")
		args = append(args, like, like, like)
	}
	if filter.Status != "" {
		whereParts = append(whereParts, "status = ?")
		args = append(args, filter.Status)
	}
	if filter.AuthType != "" {
		whereParts = append(whereParts, "auth_type = ?")
		args = append(args, filter.AuthType)
	}
	if len(whereParts) > 0 {
		opts.Where = WhereClause{
			Query: joinAnd(whereParts),
			Args:  args,
		}
	}

	return r.List(ctx, opts)
}

// joinAnd joins SQL WHERE fragments with " AND ". Extracted for clarity; the
// stdlib strings.Join also works but this keeps the call-site obvious.
func joinAnd(parts []string) string {
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return parts[0]
	}
	out := parts[0]
	for _, p := range parts[1:] {
		out += " AND " + p
	}
	return out
}
