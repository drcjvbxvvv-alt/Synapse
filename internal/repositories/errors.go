// Package repositories defines the data-access abstraction layer for Synapse.
//
// The Repository layer sits between services and GORM, enforcing two rules:
//
//  1. Every query MUST use context.WithContext, so request cancellation
//     propagates through to the DB driver.
//  2. Handlers never import GORM — they depend on typed Repository interfaces
//     and mock them in tests.
//
// Domain-specific repositories (ClusterRepository, UserRepository, …) embed
// BaseRepository[T] to get the common CRUD surface, and add their own methods
// for domain-specific queries.
//
// See docs/adr/0001-repository-layer.md for the architectural decision record.
package repositories

import "errors"

// Sentinel errors returned by the Repository layer. Domain code should compare
// using errors.Is and translate into apierrors.AppError at the service
// boundary (so handlers never see gorm.ErrRecordNotFound directly).
var (
	// ErrNotFound is returned when a single-record query (Get, FindOne)
	// matches no row. Wraps gorm.ErrRecordNotFound internally.
	ErrNotFound = errors.New("repository: record not found")

	// ErrAlreadyExists is returned when a Create violates a unique index.
	// Domain repositories may return this after inspecting the driver error;
	// BaseRepository does not detect this automatically because the error
	// shape is driver-specific (postgres).
	ErrAlreadyExists = errors.New("repository: record already exists")

	// ErrInvalidArgument is returned for programmer errors — e.g. passing a
	// zero ID to Get, or nil entity to Create.
	ErrInvalidArgument = errors.New("repository: invalid argument")
)
