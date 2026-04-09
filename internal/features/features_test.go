package features_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/shaia/Synapse/internal/features"
)

// mockStore is a deterministic in-memory Store used by tests.
type mockStore struct {
	enabled map[features.Flag]bool
}

func (m mockStore) IsEnabled(flag features.Flag, _ features.EvalContext) bool {
	return m.enabled[flag]
}

func TestIsEnabled_EnvVar_Truthy(t *testing.T) {
	// Replace the default store so this test doesn't rely on process env.
	prev := features.GetStore()
	defer features.SetStore(prev)

	features.SetStore(mockStore{enabled: map[features.Flag]bool{
		features.FlagRepositoryLayer: true,
	}})

	assert.True(t, features.IsEnabled(features.FlagRepositoryLayer))
	assert.False(t, features.IsEnabled(features.FlagOTEL))
}

func TestIsEnabled_EnvVar_RealBackend(t *testing.T) {
	// Test the actual env-backed store with t.Setenv.
	prev := features.GetStore()
	defer features.SetStore(prev)
	features.SetStore(nil) // reset to envStore default

	t.Setenv("SYNAPSE_FLAG_USE_REPO_LAYER", "true")
	assert.True(t, features.IsEnabled(features.FlagRepositoryLayer))

	t.Setenv("SYNAPSE_FLAG_USE_REPO_LAYER", "0")
	assert.False(t, features.IsEnabled(features.FlagRepositoryLayer))

	t.Setenv("SYNAPSE_FLAG_USE_REPO_LAYER", "yes")
	assert.True(t, features.IsEnabled(features.FlagRepositoryLayer))
}

func TestSetStore_NilResetsToEnv(t *testing.T) {
	prev := features.GetStore()
	defer features.SetStore(prev)

	features.SetStore(mockStore{enabled: map[features.Flag]bool{
		features.FlagOTEL: true,
	}})
	assert.True(t, features.IsEnabled(features.FlagOTEL))

	features.SetStore(nil)
	// After reset, the env-backed store returns false (no env var set for OTEL).
	assert.False(t, features.IsEnabled(features.FlagOTEL))
}
