package services

import (
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// ---------------------------------------------------------------------------
// parseRolloutInfo
// ---------------------------------------------------------------------------

func TestParseRolloutInfo_Canary(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"name":              "my-app",
				"namespace":         "default",
				"creationTimestamp": time.Now().Format(time.RFC3339),
			},
			"spec": map[string]interface{}{
				"strategy": map[string]interface{}{
					"canary": map[string]interface{}{
						"steps": []interface{}{},
					},
				},
				"template": map[string]interface{}{
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{
								"name":  "app",
								"image": "nginx:1.21",
							},
						},
					},
				},
			},
			"status": map[string]interface{}{
				"phase":           "Healthy",
				"replicas":        int64(3),
				"readyReplicas":   int64(3),
				"updatedReplicas": int64(3),
				"stableRS":        "abc123",
				"currentPodHash":  "def456",
				"canary": map[string]interface{}{
					"weight": int64(20),
				},
			},
		},
	}

	info := parseRolloutInfo(obj)

	if info.Name != "my-app" {
		t.Errorf("expected my-app, got %s", info.Name)
	}
	if info.Namespace != "default" {
		t.Errorf("expected default, got %s", info.Namespace)
	}
	if info.Strategy != "canary" {
		t.Errorf("expected canary, got %s", info.Strategy)
	}
	if info.Status != "Healthy" {
		t.Errorf("expected Healthy, got %s", info.Status)
	}
	if info.CurrentImage != "nginx:1.21" {
		t.Errorf("expected nginx:1.21, got %s", info.CurrentImage)
	}
	if info.DesiredReplicas != 3 {
		t.Errorf("expected 3 desired replicas, got %d", info.DesiredReplicas)
	}
	if info.ReadyReplicas != 3 {
		t.Errorf("expected 3 ready replicas, got %d", info.ReadyReplicas)
	}
	if info.CanaryWeight != 20 {
		t.Errorf("expected canary weight 20, got %d", info.CanaryWeight)
	}
	if info.StableRevision != "abc123" {
		t.Errorf("expected abc123, got %s", info.StableRevision)
	}
}

func TestParseRolloutInfo_BlueGreen(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"name":              "bg-app",
				"namespace":         "production",
				"creationTimestamp": time.Now().Format(time.RFC3339),
			},
			"spec": map[string]interface{}{
				"strategy": map[string]interface{}{
					"blueGreen": map[string]interface{}{
						"activeService":  "bg-app-active",
						"previewService": "bg-app-preview",
					},
				},
			},
			"status": map[string]interface{}{
				"phase": "Paused",
			},
		},
	}

	info := parseRolloutInfo(obj)

	if info.Strategy != "blueGreen" {
		t.Errorf("expected blueGreen, got %s", info.Strategy)
	}
	if info.Status != "Paused" {
		t.Errorf("expected Paused, got %s", info.Status)
	}
}

func TestParseRolloutInfo_NoStatus(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"name":              "new-app",
				"namespace":         "default",
				"creationTimestamp": time.Now().Format(time.RFC3339),
			},
			"spec": map[string]interface{}{},
		},
	}

	info := parseRolloutInfo(obj)

	if info.Status != "Unknown" {
		t.Errorf("expected Unknown, got %s", info.Status)
	}
	if info.Strategy != "unknown" {
		t.Errorf("expected unknown strategy, got %s", info.Strategy)
	}
}

func TestParseRolloutInfo_WithConditions(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"name":              "cond-app",
				"namespace":         "default",
				"creationTimestamp": time.Now().Format(time.RFC3339),
			},
			"spec": map[string]interface{}{},
			"status": map[string]interface{}{
				"phase": "Degraded",
				"conditions": []interface{}{
					map[string]interface{}{
						"type":    "Available",
						"status":  "False",
						"reason":  "ProgressDeadlineExceeded",
						"message": "deadline exceeded",
					},
				},
			},
		},
	}

	info := parseRolloutInfo(obj)

	if len(info.Conditions) != 1 {
		t.Fatalf("expected 1 condition, got %d", len(info.Conditions))
	}
	if info.Conditions[0].Type != "Available" {
		t.Errorf("expected Available, got %s", info.Conditions[0].Type)
	}
	if info.Conditions[0].Reason != "ProgressDeadlineExceeded" {
		t.Errorf("expected ProgressDeadlineExceeded, got %s", info.Conditions[0].Reason)
	}
}

// ---------------------------------------------------------------------------
// detectRolloutStrategy
// ---------------------------------------------------------------------------

func TestDetectRolloutStrategy_Canary(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"spec": map[string]interface{}{
				"strategy": map[string]interface{}{
					"canary": map[string]interface{}{},
				},
			},
		},
	}
	if s := detectRolloutStrategy(obj); s != "canary" {
		t.Errorf("expected canary, got %s", s)
	}
}

func TestDetectRolloutStrategy_BlueGreen(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"spec": map[string]interface{}{
				"strategy": map[string]interface{}{
					"blueGreen": map[string]interface{}{},
				},
			},
		},
	}
	if s := detectRolloutStrategy(obj); s != "blueGreen" {
		t.Errorf("expected blueGreen, got %s", s)
	}
}

func TestDetectRolloutStrategy_Unknown(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"spec": map[string]interface{}{},
		},
	}
	if s := detectRolloutStrategy(obj); s != "unknown" {
		t.Errorf("expected unknown, got %s", s)
	}
}

// ---------------------------------------------------------------------------
// ValidateRolloutStatus
// ---------------------------------------------------------------------------

func TestValidateRolloutStatus_Valid(t *testing.T) {
	for _, s := range []string{"healthy", "Healthy", "progressing", "degraded", "paused", "PAUSED"} {
		if err := ValidateRolloutStatus(s); err != nil {
			t.Errorf("expected valid for %q: %v", s, err)
		}
	}
}

func TestValidateRolloutStatus_Invalid(t *testing.T) {
	if err := ValidateRolloutStatus("running"); err == nil {
		t.Error("expected error for invalid status")
	}
}

// ---------------------------------------------------------------------------
// IsRolloutStatusMatch
// ---------------------------------------------------------------------------

func TestIsRolloutStatusMatch(t *testing.T) {
	if !IsRolloutStatusMatch("Healthy", "healthy") {
		t.Error("expected match for Healthy/healthy")
	}
	if !IsRolloutStatusMatch("PAUSED", "Paused") {
		t.Error("expected match for PAUSED/Paused")
	}
	if IsRolloutStatusMatch("Healthy", "Degraded") {
		t.Error("expected no match for Healthy/Degraded")
	}
}

// ---------------------------------------------------------------------------
// NewRolloutService
// ---------------------------------------------------------------------------

func TestNewRolloutService(t *testing.T) {
	svc := NewRolloutService()
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
}

// ---------------------------------------------------------------------------
// nestedString / stringFromMap helpers
// ---------------------------------------------------------------------------

func TestNestedString_Found(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"status": map[string]interface{}{
				"phase": "Healthy",
			},
		},
	}
	if v := nestedString(obj, "status", "phase"); v != "Healthy" {
		t.Errorf("expected Healthy, got %s", v)
	}
}

func TestNestedString_NotFound(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{},
	}
	if v := nestedString(obj, "status", "phase"); v != "" {
		t.Errorf("expected empty, got %s", v)
	}
}

func TestStringFromMap(t *testing.T) {
	m := map[string]interface{}{
		"key": "value",
		"num": 42,
	}
	if v := stringFromMap(m, "key"); v != "value" {
		t.Errorf("expected value, got %s", v)
	}
	if v := stringFromMap(m, "num"); v != "" {
		t.Errorf("expected empty for non-string, got %s", v)
	}
	if v := stringFromMap(m, "missing"); v != "" {
		t.Errorf("expected empty for missing, got %s", v)
	}
}

// ---------------------------------------------------------------------------
// rolloutGVR
// ---------------------------------------------------------------------------

func TestRolloutGVR(t *testing.T) {
	if rolloutGVR.Group != "argoproj.io" {
		t.Errorf("expected argoproj.io, got %s", rolloutGVR.Group)
	}
	if rolloutGVR.Version != "v1alpha1" {
		t.Errorf("expected v1alpha1, got %s", rolloutGVR.Version)
	}
	if rolloutGVR.Resource != "rollouts" {
		t.Errorf("expected rollouts, got %s", rolloutGVR.Resource)
	}
}
