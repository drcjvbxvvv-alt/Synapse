package features

import (
	"context"
	"sync"
	"time"

	"github.com/shaia/Synapse/internal/models"
	"gorm.io/gorm"
)

// DBStore is a DB-backed implementation of Store.
// It caches all flag values in memory and refreshes from the database at most
// once per ttl interval, keeping hot-path latency negligible.
//
// On cache miss (flag not in DB), the value defaults to false (not envStore —
// callers who want an env fallback should wrap with a MultiStore).
//
// Thread-safe.
type DBStore struct {
	db          *gorm.DB
	ttl         time.Duration
	mu          sync.RWMutex
	cache       map[Flag]bool
	refreshedAt time.Time
}

// NewDBStore creates a DBStore that refreshes its cache every ttl duration.
// Recommended ttl: 30s for production, 1s for tests.
func NewDBStore(db *gorm.DB, ttl time.Duration) *DBStore {
	return &DBStore{
		db:    db,
		ttl:   ttl,
		cache: make(map[Flag]bool),
	}
}

// IsEnabled implements Store. It returns the DB value for the flag, falling
// back to false if the flag is not yet seeded in the database.
// EvalContext is accepted for interface compatibility but not yet used (planned
// for percentage-rollout extension).
func (s *DBStore) IsEnabled(flag Flag, _ EvalContext) bool {
	s.mu.RLock()
	fresh := time.Since(s.refreshedAt) < s.ttl
	val, hit := s.cache[flag]
	s.mu.RUnlock()

	if fresh && hit {
		return val
	}

	// Cache stale or flag not yet loaded — reload all flags from DB.
	if err := s.refresh(); err != nil {
		// On DB error, return false (safe default).
		return false
	}

	s.mu.RLock()
	val = s.cache[flag]
	s.mu.RUnlock()
	return val
}

// refresh loads all rows from feature_flags and replaces the in-memory cache.
func (s *DBStore) refresh() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var rows []models.FeatureFlag
	if err := s.db.WithContext(ctx).Find(&rows).Error; err != nil {
		return err
	}

	next := make(map[Flag]bool, len(rows))
	for _, r := range rows {
		next[Flag(r.Key)] = r.Enabled
	}

	s.mu.Lock()
	s.cache = next
	s.refreshedAt = time.Now()
	s.mu.Unlock()
	return nil
}

// Invalidate clears the cache so the next call to IsEnabled forces a DB reload.
// Call after any Set/update so flag changes take effect immediately.
func (s *DBStore) Invalidate() {
	s.mu.Lock()
	s.refreshedAt = time.Time{}
	s.mu.Unlock()
}
