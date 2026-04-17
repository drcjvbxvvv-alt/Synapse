package services

import (
	"gorm.io/gorm"

	"github.com/shaia/Synapse/internal/features"
	"github.com/shaia/Synapse/internal/repositories"
)

// PermissionService 權限服務
//
// P0-4b migration status: dual-path. When features.FlagRepositoryLayer is
// enabled the service routes reads/writes through the Repository layer;
// when disabled it falls back to the legacy *gorm.DB path so the flag can
// be flipped off if a production regression shows up.
//
// Note: all method signatures intentionally stay ctx-less. PermissionService
// is called from 30+ handler sites plus the auth middleware on every
// request; pushing ctx through every caller would explode P0-4b scope.
// Internally the repo calls run with a background context — request-scoped
// tracing is deferred to P0-4c together with ClusterService.GetCluster.
type PermissionService struct {
	db   *gorm.DB
	repo repositories.PermissionRepository
}

// NewPermissionService 建立權限服務
func NewPermissionService(db *gorm.DB, repo repositories.PermissionRepository) *PermissionService {
	return &PermissionService{db: db, repo: repo}
}

// useRepo reports whether the service should dispatch to the Repository layer.
func (s *PermissionService) useRepo() bool {
	return s.repo != nil && features.IsEnabled(features.FlagRepositoryLayer)
}
