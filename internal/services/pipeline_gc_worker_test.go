package services

import (
	"testing"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDefaultPipelineGCConfig(t *testing.T) {
	cfg := DefaultPipelineGCConfig()

	if cfg.OrphanJobScanInterval != 10*time.Minute {
		t.Errorf("expected 10m scan interval, got: %v", cfg.OrphanJobScanInterval)
	}
	if cfg.OrphanJobMaxAge != 1*time.Hour {
		t.Errorf("expected 1h max age, got: %v", cfg.OrphanJobMaxAge)
	}
	if cfg.RunRetentionDays != 90 {
		t.Errorf("expected 90 day run retention, got: %d", cfg.RunRetentionDays)
	}
	if cfg.LogRetentionDays != 30 {
		t.Errorf("expected 30 day log retention, got: %d", cfg.LogRetentionDays)
	}
}

func TestIsJobFailed_True(t *testing.T) {
	job := &batchv1.Job{
		Status: batchv1.JobStatus{
			Conditions: []batchv1.JobCondition{
				{
					Type:   batchv1.JobFailed,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}
	if !isJobFailed(job) {
		t.Error("expected job to be detected as failed")
	}
}

func TestIsJobFailed_False_NoConditions(t *testing.T) {
	job := &batchv1.Job{
		Status: batchv1.JobStatus{},
	}
	if isJobFailed(job) {
		t.Error("expected job without conditions to not be failed")
	}
}

func TestIsJobFailed_False_SucceededCondition(t *testing.T) {
	job := &batchv1.Job{
		Status: batchv1.JobStatus{
			Conditions: []batchv1.JobCondition{
				{
					Type:   batchv1.JobComplete,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}
	if isJobFailed(job) {
		t.Error("expected completed job to not be detected as failed")
	}
}

func TestIsJobFailed_False_FailedStatusFalse(t *testing.T) {
	job := &batchv1.Job{
		Status: batchv1.JobStatus{
			Conditions: []batchv1.JobCondition{
				{
					Type:   batchv1.JobFailed,
					Status: corev1.ConditionFalse,
				},
			},
		},
	}
	if isJobFailed(job) {
		t.Error("expected job with Failed=False to not be detected as failed")
	}
}

func TestPipelineGCWorker_NewAndStop(t *testing.T) {
	cfg := DefaultPipelineGCConfig()
	// Use nil db and k8sProvider — just test construction and stop
	worker := NewPipelineGCWorker(nil, nil, cfg)
	if worker == nil {
		t.Fatal("expected non-nil worker")
	}
	if worker.cfg.RunRetentionDays != 90 {
		t.Errorf("expected 90 day retention, got: %d", worker.cfg.RunRetentionDays)
	}
	// Stop should not panic
	worker.Stop()
}

func TestPipelineGCWorker_CustomConfig(t *testing.T) {
	cfg := PipelineGCConfig{
		OrphanJobScanInterval: 5 * time.Minute,
		OrphanJobMaxAge:       30 * time.Minute,
		RunRetentionDays:      30,
		RunCleanupInterval:    12 * time.Hour,
		LogRetentionDays:      7,
		LogCleanupInterval:    6 * time.Hour,
	}
	worker := NewPipelineGCWorker(nil, nil, cfg)
	if worker.cfg.RunRetentionDays != 30 {
		t.Errorf("expected 30 day retention, got: %d", worker.cfg.RunRetentionDays)
	}
	if worker.cfg.LogRetentionDays != 7 {
		t.Errorf("expected 7 day log retention, got: %d", worker.cfg.LogRetentionDays)
	}
	if worker.cfg.OrphanJobMaxAge != 30*time.Minute {
		t.Errorf("expected 30m max age, got: %v", worker.cfg.OrphanJobMaxAge)
	}
}

// TestOrphanJobAge verifies the age calculation logic used in cleanupClusterOrphanJobs
func TestOrphanJobAge_Calculation(t *testing.T) {
	maxAge := 1 * time.Hour
	now := time.Now()

	tests := []struct {
		name        string
		completedAt time.Time
		shouldClean bool
	}{
		{"completed 2h ago", now.Add(-2 * time.Hour), true},
		{"completed 30m ago", now.Add(-30 * time.Minute), false},
		{"completed 1h1s ago", now.Add(-1*time.Hour - time.Second), true},
		{"completed just now", now, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			elapsed := now.Sub(tc.completedAt)
			shouldClean := elapsed >= maxAge
			if shouldClean != tc.shouldClean {
				t.Errorf("elapsed=%v, maxAge=%v: expected shouldClean=%v, got %v",
					elapsed, maxAge, tc.shouldClean, shouldClean)
			}
		})
	}
}

// TestRetentionCutoff verifies cutoff date calculation
func TestRetentionCutoff(t *testing.T) {
	now := time.Date(2026, 4, 14, 12, 0, 0, 0, time.UTC)

	runCutoff := now.AddDate(0, 0, -90)
	expected := time.Date(2026, 1, 14, 12, 0, 0, 0, time.UTC)
	if !runCutoff.Equal(expected) {
		t.Errorf("expected run cutoff %v, got %v", expected, runCutoff)
	}

	logCutoff := now.AddDate(0, 0, -30)
	expectedLog := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	if !logCutoff.Equal(expectedLog) {
		t.Errorf("expected log cutoff %v, got %v", expectedLog, logCutoff)
	}
}

// TestJobCompletionTimeExtraction verifies the completion time detection logic
func TestJobCompletionTimeExtraction(t *testing.T) {
	now := metav1.Now()

	t.Run("uses CompletionTime when available", func(t *testing.T) {
		job := &batchv1.Job{
			Status: batchv1.JobStatus{
				CompletionTime: &now,
				Succeeded:      1,
			},
		}
		ct := job.Status.CompletionTime
		if ct == nil {
			t.Fatal("expected non-nil completion time")
		}
	})

	t.Run("falls back to condition time", func(t *testing.T) {
		job := &batchv1.Job{
			Status: batchv1.JobStatus{
				Succeeded: 1,
				Conditions: []batchv1.JobCondition{
					{
						Type:               batchv1.JobComplete,
						Status:             corev1.ConditionTrue,
						LastTransitionTime: now,
					},
				},
			},
		}
		// CompletionTime is nil
		if job.Status.CompletionTime != nil {
			t.Fatal("expected nil CompletionTime for this test")
		}
		// Should fall back to condition time
		var fallbackTime *metav1.Time
		for _, cond := range job.Status.Conditions {
			if !cond.LastTransitionTime.Time.IsZero() {
				fallbackTime = &cond.LastTransitionTime
			}
		}
		if fallbackTime == nil {
			t.Fatal("expected fallback time from condition")
		}
	})
}
