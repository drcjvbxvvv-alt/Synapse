package services

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/shaia/Synapse/internal/testutil"
)

// newTestScheduler builds a minimal PipelineScheduler with a mock DB.
func newTestScheduler(t *testing.T) *PipelineScheduler {
	t.Helper()
	db, _, err := testutil.SetupMockDB()
	require.NoError(t, err)
	return NewPipelineScheduler(
		db, nil, nil, nil, nil, nil, DefaultSchedulerConfig(),
	)
}

// ── IsAlive ──────────────────────────────────────────────────────────────────

func TestPipelineScheduler_IsAlive_FalseBeforeStart(t *testing.T) {
	s := newTestScheduler(t)
	assert.False(t, s.IsAlive(), "must be false before Start()")
}

func TestPipelineScheduler_IsAlive_TrueAfterStart(t *testing.T) {
	s := NewPipelineScheduler(
		nil, nil, nil, nil, nil, nil, DefaultSchedulerConfig(),
	)
	s.Start()
	defer s.Stop()

	// Give the goroutine a moment to set the flag.
	assert.Eventually(t, func() bool {
		return s.IsAlive()
	}, 200*time.Millisecond, 10*time.Millisecond, "IsAlive must be true after Start()")
}

func TestPipelineScheduler_IsAlive_FalseAfterStop(t *testing.T) {
	s := NewPipelineScheduler(
		nil, nil, nil, nil, nil, nil, DefaultSchedulerConfig(),
	)
	s.Start()
	// Wait for it to come alive.
	require.Eventually(t, s.IsAlive, 200*time.Millisecond, 10*time.Millisecond)

	s.Stop()

	// loop() returns → defer sets flag to false.
	assert.Eventually(t, func() bool {
		return !s.IsAlive()
	}, 2*time.Second, 20*time.Millisecond, "IsAlive must be false after Stop()")
}

// ── QueueDepth ───────────────────────────────────────────────────────────────

func TestPipelineScheduler_QueueDepth_NilDB_ReturnsZero(t *testing.T) {
	s := NewPipelineScheduler(
		nil, nil, nil, nil, nil, nil, DefaultSchedulerConfig(),
	)
	depth, err := s.QueueDepth(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, int64(0), depth)
}

func TestPipelineScheduler_QueueDepth_ReturnsCount(t *testing.T) {
	db, mock, err := testutil.SetupMockDB()
	require.NoError(t, err)

	mock.ExpectQuery(`SELECT count\(\*\) FROM "pipeline_runs"`).
		WillReturnRows(mock.NewRows([]string{"count"}).AddRow(7))

	s := NewPipelineScheduler(
		db, nil, nil, nil, nil, nil, DefaultSchedulerConfig(),
	)

	depth, err := s.QueueDepth(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(7), depth)
	assert.NoError(t, mock.ExpectationsWereMet())
}
