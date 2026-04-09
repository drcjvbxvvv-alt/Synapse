package repositories

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/shaia/Synapse/internal/models"
)

// ClusterRepository is the data-access interface for models.Cluster.
//
// It embeds the generic Repository[models.Cluster] surface and adds a handful
// of domain-specific finders that encode the queries scattered across
// ClusterService today. Handlers and services depend on this interface —
// never on *gorm.DB directly — so unit tests can substitute a fake.
type ClusterRepository interface {
	Repository[models.Cluster]

	// FindByName fetches a cluster by its unique name. Returns ErrNotFound
	// when no cluster matches.
	FindByName(ctx context.Context, name string) (*models.Cluster, error)

	// ListConnectable returns every cluster whose status is NOT "unhealthy".
	// Used by the informer pre-warm loop and any other caller that needs to
	// skip known-bad clusters.
	ListConnectable(ctx context.Context) ([]*models.Cluster, error)

	// FindByIDs returns clusters whose primary keys are in the given slice.
	// An empty input returns an empty slice without hitting the DB. Used by
	// the permission-filtered cluster list on ClusterHandler.GetClusters.
	FindByIDs(ctx context.Context, ids []uint) ([]*models.Cluster, error)

	// CountByStatus returns the number of clusters with the given status.
	// Used by GetClusterStats — pushed into the repo so the service does
	// not touch *gorm.DB directly.
	CountByStatus(ctx context.Context, status string) (int64, error)
}

// clusterRepository is the concrete GORM-backed implementation.
type clusterRepository struct {
	*BaseRepository[models.Cluster]
}

// NewClusterRepository constructs a ClusterRepository bound to the given DB.
func NewClusterRepository(db *gorm.DB) ClusterRepository {
	return &clusterRepository{
		BaseRepository: NewBaseRepository[models.Cluster](db),
	}
}

// FindByName implements ClusterRepository.
func (r *clusterRepository) FindByName(ctx context.Context, name string) (*models.Cluster, error) {
	if name == "" {
		return nil, fmt.Errorf("%w: name must not be empty", ErrInvalidArgument)
	}
	return r.FindOne(ctx, "name = ?", name)
}

// ListConnectable implements ClusterRepository.
func (r *clusterRepository) ListConnectable(ctx context.Context) ([]*models.Cluster, error) {
	return r.Find(ctx, "status != ?", "unhealthy")
}

// FindByIDs implements ClusterRepository.
func (r *clusterRepository) FindByIDs(ctx context.Context, ids []uint) ([]*models.Cluster, error) {
	if len(ids) == 0 {
		return []*models.Cluster{}, nil
	}
	return r.Find(ctx, "id IN ?", ids)
}

// CountByStatus implements ClusterRepository.
func (r *clusterRepository) CountByStatus(ctx context.Context, status string) (int64, error) {
	if status == "" {
		return r.Count(ctx)
	}
	return r.Count(ctx, "status = ?", status)
}
