package k8s

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestHealthCheck_Empty(t *testing.T) {
	m := NewClusterInformerManager()
	health := m.HealthCheck()
	assert.Empty(t, health, "no clusters registered → empty map")
}

func TestHealthCheck_ReflectsRuntimeState(t *testing.T) {
	m := NewClusterInformerManager()

	// Inject a fake ClusterRuntime directly (avoids needing a real K8s cluster).
	rt := &ClusterRuntime{
		clusterID:    42,
		started:      true,
		synced:       true,
		lastAccessAt: time.Now(),
		stopCh:       make(chan struct{}),
	}
	m.mu.Lock()
	m.clusters[42] = rt
	m.mu.Unlock()

	health := m.HealthCheck()
	assert.Len(t, health, 1)

	h, ok := health[42]
	assert.True(t, ok, "cluster 42 must appear in health map")
	assert.Equal(t, uint(42), h.ClusterID)
	assert.True(t, h.Started)
	assert.True(t, h.Synced)
	assert.False(t, h.LastAccess.IsZero())
}

func TestHealthCheck_NotSynced(t *testing.T) {
	m := NewClusterInformerManager()

	rt := &ClusterRuntime{
		clusterID:    7,
		started:      true,
		synced:       false, // not yet synced
		lastAccessAt: time.Now(),
		stopCh:       make(chan struct{}),
	}
	m.mu.Lock()
	m.clusters[7] = rt
	m.mu.Unlock()

	health := m.HealthCheck()
	h := health[7]
	assert.True(t, h.Started)
	assert.False(t, h.Synced)
}

func TestHealthCheck_ExposesStartedAt(t *testing.T) {
	m := NewClusterInformerManager()
	started := time.Now().Add(-2 * time.Minute)
	rt := &ClusterRuntime{
		clusterID:    5,
		started:      true,
		synced:       false,
		startedAt:    started,
		lastAccessAt: time.Now(),
		stopCh:       make(chan struct{}),
	}
	m.mu.Lock()
	m.clusters[5] = rt
	m.mu.Unlock()

	health := m.HealthCheck()
	h := health[5]
	assert.False(t, h.StartedAt.IsZero(), "StartedAt must be exposed")
	assert.WithinDuration(t, started, h.StartedAt, time.Second)
}

func TestRestartStuckInformers_DoesNotRestartSynced(t *testing.T) {
	m := NewClusterInformerManager()

	// Synced cluster — should never be touched
	rt := &ClusterRuntime{
		clusterID:    10,
		started:      true,
		synced:       true, // already synced
		startedAt:    time.Now().Add(-10 * time.Minute),
		lastAccessAt: time.Now(),
		stopCh:       make(chan struct{}),
	}
	m.mu.Lock()
	m.clusters[10] = rt
	m.mu.Unlock()

	m.restartStuckInformers(5 * time.Minute)

	m.mu.RLock()
	_, stillPresent := m.clusters[10]
	m.mu.RUnlock()

	assert.True(t, stillPresent, "synced cluster must not be restarted")
}

func TestRestartStuckInformers_RemovesStuckCluster(t *testing.T) {
	m := NewClusterInformerManager()

	// Stuck: started but never synced, started >5 min ago, cluster=nil (no real K8s)
	rt := &ClusterRuntime{
		clusterID:    20,
		cluster:      nil, // nil → watcher skips re-register but still removes
		started:      true,
		synced:       false,
		startedAt:    time.Now().Add(-6 * time.Minute),
		lastAccessAt: time.Now(),
		stopCh:       make(chan struct{}),
	}
	m.mu.Lock()
	m.clusters[20] = rt
	m.mu.Unlock()

	m.restartStuckInformers(5 * time.Minute)

	m.mu.RLock()
	_, stillPresent := m.clusters[20]
	m.mu.RUnlock()

	assert.False(t, stillPresent, "stuck cluster with nil model must be removed")
}

func TestRestartStuckInformers_DoesNotRestartFreshUnsyncedCluster(t *testing.T) {
	m := NewClusterInformerManager()

	// Not yet stuck: started recently
	rt := &ClusterRuntime{
		clusterID:    30,
		started:      true,
		synced:       false,
		startedAt:    time.Now().Add(-1 * time.Minute), // only 1 min, threshold is 5
		lastAccessAt: time.Now(),
		stopCh:       make(chan struct{}),
	}
	m.mu.Lock()
	m.clusters[30] = rt
	m.mu.Unlock()

	m.restartStuckInformers(5 * time.Minute)

	m.mu.RLock()
	_, stillPresent := m.clusters[30]
	m.mu.RUnlock()

	assert.True(t, stillPresent, "recently-started cluster must not be restarted yet")
}

func TestHealthCheck_MultipleClusterCounts(t *testing.T) {
	m := NewClusterInformerManager()

	for _, id := range []uint{1, 2, 3} {
		rt := &ClusterRuntime{
			clusterID:    id,
			started:      true,
			synced:       id%2 == 1, // odd = synced
			lastAccessAt: time.Now(),
			stopCh:       make(chan struct{}),
		}
		m.mu.Lock()
		m.clusters[id] = rt
		m.mu.Unlock()
	}

	health := m.HealthCheck()
	assert.Len(t, health, 3)

	synced := 0
	for _, h := range health {
		if h.Synced {
			synced++
		}
	}
	assert.Equal(t, 2, synced, "clusters 1 and 3 should be synced")
}
