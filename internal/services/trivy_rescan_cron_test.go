package services

import (
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

func TestDefaultRescanInterval(t *testing.T) {
	if DefaultRescanInterval != 24*time.Hour {
		t.Errorf("expected 24h, got %v", DefaultRescanInterval)
	}
}

func TestDefaultRescanAge(t *testing.T) {
	if DefaultRescanAge != 24*time.Hour {
		t.Errorf("expected 24h, got %v", DefaultRescanAge)
	}
}

func TestDefaultMaxPerRound(t *testing.T) {
	if DefaultMaxPerRound != 100 {
		t.Errorf("expected 100, got %d", DefaultMaxPerRound)
	}
}

// ---------------------------------------------------------------------------
// Constructor
// ---------------------------------------------------------------------------

func TestNewTrivyRescanCron(t *testing.T) {
	cron := NewTrivyRescanCron(nil, nil)

	if cron.interval != DefaultRescanInterval {
		t.Errorf("expected default interval, got %v", cron.interval)
	}
	if cron.age != DefaultRescanAge {
		t.Errorf("expected default age, got %v", cron.age)
	}
	if cron.maxPer != DefaultMaxPerRound {
		t.Errorf("expected default maxPer, got %d", cron.maxPer)
	}
}

// ---------------------------------------------------------------------------
// Setters
// ---------------------------------------------------------------------------

func TestTrivyRescanCron_SetInterval(t *testing.T) {
	cron := NewTrivyRescanCron(nil, nil)
	cron.SetInterval(12 * time.Hour)

	if cron.interval != 12*time.Hour {
		t.Errorf("expected 12h, got %v", cron.interval)
	}
}

func TestTrivyRescanCron_SetRescanAge(t *testing.T) {
	cron := NewTrivyRescanCron(nil, nil)
	cron.SetRescanAge(6 * time.Hour)

	if cron.age != 6*time.Hour {
		t.Errorf("expected 6h, got %v", cron.age)
	}
}

func TestTrivyRescanCron_SetMaxPerRound(t *testing.T) {
	cron := NewTrivyRescanCron(nil, nil)
	cron.SetMaxPerRound(50)

	if cron.maxPer != 50 {
		t.Errorf("expected 50, got %d", cron.maxPer)
	}
}

// ---------------------------------------------------------------------------
// staleImage struct
// ---------------------------------------------------------------------------

func TestStaleImage_Fields(t *testing.T) {
	img := staleImage{
		ClusterID:     5,
		Namespace:     "production",
		PodName:       "api-pod-abc",
		ContainerName: "api",
		Image:         "myrepo/api:v2.1",
	}

	if img.ClusterID != 5 {
		t.Error("cluster_id mismatch")
	}
	if img.Image != "myrepo/api:v2.1" {
		t.Error("image mismatch")
	}
	if img.Namespace != "production" {
		t.Error("namespace mismatch")
	}
}

// ---------------------------------------------------------------------------
// Stop
// ---------------------------------------------------------------------------

func TestTrivyRescanCron_StopIdempotent(t *testing.T) {
	cron := NewTrivyRescanCron(nil, nil)

	// Stop should not panic even without Start
	cron.Stop()

	// Verify stopCh is closed
	select {
	case <-cron.stopCh:
		// ok
	default:
		t.Error("expected stopCh to be closed")
	}
}
