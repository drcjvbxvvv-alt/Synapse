// Package features provides a minimal feature-flag facility for the
// Synapse codebase.
//
// Goals:
//   - Allow refactors (Repository layer, router split, OTEL, …) to be merged
//     behind a flag and rolled out gradually.
//   - Require zero new runtime dependencies — the default store only reads
//     environment variables.
//   - Keep the public API small enough that a future DB-backed store can
//     replace envStore without touching call-sites.
//
// See docs/ARCHITECTURE_REVIEW.md §11.2 for the full feature-flag strategy.
//
// Usage:
//
//	if features.IsEnabled(features.FlagRepositoryLayer) {
//	    // new code path
//	} else {
//	    // legacy code path
//	}
//
// Env-var convention: SYNAPSE_FLAG_<UPPER_SNAKE_FLAG_NAME>.
// Example: FlagRepositoryLayer ("use_repo_layer") → SYNAPSE_FLAG_USE_REPO_LAYER.
// Values "1", "true", "yes", "on" (case-insensitive) enable the flag.
package features

import (
	"os"
	"strings"
	"sync"
)

// Flag is the canonical name of a feature flag. Register new flags as
// constants below so IDEs autocomplete and typos become compile errors.
type Flag string

const (
	// FlagRepositoryLayer gates P0-4 Repository layer usage.
	// When enabled, services route data access through internal/repositories
	// instead of holding *gorm.DB directly.
	// Env var: SYNAPSE_FLAG_USE_REPO_LAYER
	FlagRepositoryLayer Flag = "use_repo_layer"

	// FlagRouteSplit gates P1-2 router module split.
	// Env var: SYNAPSE_FLAG_USE_SPLIT_ROUTER
	FlagRouteSplit Flag = "use_split_router"

	// FlagOTEL gates P1-10 OpenTelemetry tracing.
	// Env var: SYNAPSE_FLAG_ENABLE_OTEL_TRACING
	FlagOTEL Flag = "enable_otel_tracing"

	// FlagRedisRateLimit gates P1-8 Redis-backed rate limiter.
	// Env var: SYNAPSE_FLAG_USE_REDIS_RATELIMIT
	FlagRedisRateLimit Flag = "use_redis_ratelimit"

	// FlagZustand gates P2-5 Zustand frontend store migration.
	// Env var: SYNAPSE_FLAG_USE_ZUSTAND_STORE
	FlagZustand Flag = "use_zustand_store"

	// FlagHashChainAudit gates P2-2 audit hash-chain verification.
	// Env var: SYNAPSE_FLAG_ENABLE_AUDIT_HASHCHAIN
	FlagHashChainAudit Flag = "enable_audit_hashchain"
)

// EvalContext carries signals a Store may use to decide flag enablement
// (e.g. percentage rollout, user allowlist). The env-backed default store
// ignores this, but future DB-backed stores will read it.
type EvalContext struct {
	UserID     uint
	ClusterID  uint
	Percentage int // 0~100 for percentage rollout
}

// Store is the contract a feature-flag backend must satisfy. Alternative
// implementations may read from a DB table, remote service, or in-memory
// override map for tests.
type Store interface {
	IsEnabled(flag Flag, ctx EvalContext) bool
}

// ---------------------------------------------------------------------------
// envStore: default, dependency-free implementation.
// ---------------------------------------------------------------------------

// envStore resolves flags purely from environment variables.
// It is safe for concurrent use.
type envStore struct{}

// IsEnabled returns true iff the corresponding env var is set to a truthy
// value. EvalContext is ignored in this implementation.
func (envStore) IsEnabled(flag Flag, _ EvalContext) bool {
	v := os.Getenv(envVarName(flag))
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// envVarName converts a Flag to its SYNAPSE_FLAG_* env variable name.
// Example: "use_repo_layer" → "SYNAPSE_FLAG_USE_REPO_LAYER".
func envVarName(flag Flag) string {
	return "SYNAPSE_FLAG_" + strings.ToUpper(string(flag))
}

// ---------------------------------------------------------------------------
// Package-level default store + convenience wrappers.
// ---------------------------------------------------------------------------

var (
	mu           sync.RWMutex
	defaultStore Store = envStore{}
)

// SetStore replaces the package-level default store. Intended for tests and
// application bootstrap; production code should prefer env-var configuration.
func SetStore(s Store) {
	mu.Lock()
	defer mu.Unlock()
	if s == nil {
		defaultStore = envStore{}
		return
	}
	defaultStore = s
}

// GetStore returns the current package-level store. Useful when a component
// wants to pass its own EvalContext.
func GetStore() Store {
	mu.RLock()
	defer mu.RUnlock()
	return defaultStore
}

// IsEnabled is the top-level convenience helper: check whether flag is
// enabled in the default store, without EvalContext.
func IsEnabled(flag Flag) bool {
	return GetStore().IsEnabled(flag, EvalContext{})
}

// IsEnabledFor is like IsEnabled but carries an EvalContext (used by DB or
// percentage-rollout stores).
func IsEnabledFor(flag Flag, ctx EvalContext) bool {
	return GetStore().IsEnabled(flag, ctx)
}
