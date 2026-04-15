package services

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ---------------------------------------------------------------------------
// isSystemNamespace
// ---------------------------------------------------------------------------

func TestIsSystemNamespace(t *testing.T) {
	cases := []struct {
		ns   string
		want bool
	}{
		{"kube-system", true},
		{"kube-public", true},
		{"kube-node-lease", true},
		{"local-path-storage", true},
		{"default", false},
		{"production", false},
		{"monitoring", false},
	}

	for _, tc := range cases {
		got := isSystemNamespace(tc.ns)
		if got != tc.want {
			t.Errorf("isSystemNamespace(%q) = %v, want %v", tc.ns, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Debounce logic
// ---------------------------------------------------------------------------

func TestTrivyPodWatcher_Debounce(t *testing.T) {
	// Use a nil TrivyService — we test debounce logic only, not actual scan
	w := NewTrivyPodWatcher(nil, 1)
	w.SetDebounceWindow(1 * time.Second)

	// Manually insert a seen entry
	w.mu.Lock()
	w.seen["nginx:1.25"] = time.Now()
	w.mu.Unlock()

	if !w.IsSeen("nginx:1.25") {
		t.Error("expected nginx:1.25 to be seen")
	}
	if w.IsSeen("redis:7") {
		t.Error("expected redis:7 to not be seen")
	}
	if w.SeenCount() != 1 {
		t.Errorf("expected 1 seen image, got %d", w.SeenCount())
	}
}

func TestTrivyPodWatcher_CleanExpired(t *testing.T) {
	w := NewTrivyPodWatcher(nil, 1)
	w.SetDebounceWindow(50 * time.Millisecond)

	w.mu.Lock()
	w.seen["old-image:v1"] = time.Now().Add(-200 * time.Millisecond) // expired (> 2x debounce)
	w.seen["new-image:v2"] = time.Now()                              // still fresh
	w.mu.Unlock()

	w.cleanExpired()

	if w.IsSeen("old-image:v1") {
		t.Error("expected old-image to be cleaned")
	}
	if !w.IsSeen("new-image:v2") {
		t.Error("expected new-image to still be seen")
	}
}

// ---------------------------------------------------------------------------
// scanPodImages — skip system namespace
// ---------------------------------------------------------------------------

func TestTrivyPodWatcher_SkipsSystemNamespace(t *testing.T) {
	w := NewTrivyPodWatcher(nil, 1)
	w.SetDebounceWindow(1 * time.Second)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "coredns-abc",
			Namespace: "kube-system",
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{
				{Name: "coredns", Image: "coredns:1.11", ImageID: "sha256:abc123"},
			},
		},
	}

	w.scanPodImages(pod)

	if w.SeenCount() != 0 {
		t.Error("should skip system namespace pods")
	}
}

// ---------------------------------------------------------------------------
// scanPodImages — skip non-running pods
// ---------------------------------------------------------------------------

func TestTrivyPodWatcher_SkipsNonRunningPods(t *testing.T) {
	w := NewTrivyPodWatcher(nil, 1)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "completed-job",
			Namespace: "default",
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodSucceeded,
			ContainerStatuses: []corev1.ContainerStatus{
				{Name: "worker", Image: "worker:v1", ImageID: "sha256:def456"},
			},
		},
	}

	w.scanPodImages(pod)

	if w.SeenCount() != 0 {
		t.Error("should skip succeeded/failed pods")
	}
}

// ---------------------------------------------------------------------------
// scanPodImages — skip containers without imageID
// ---------------------------------------------------------------------------

func TestTrivyPodWatcher_SkipsNoImageID(t *testing.T) {
	w := NewTrivyPodWatcher(nil, 1)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pending-pod",
			Namespace: "default",
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{
				{Name: "app", Image: "app:v1", ImageID: ""}, // not yet pulled
			},
		},
	}

	w.scanPodImages(pod)

	if w.SeenCount() != 0 {
		t.Error("should skip containers without imageID")
	}
}

// ---------------------------------------------------------------------------
// TrivyPodWatcher constructor
// ---------------------------------------------------------------------------

func TestNewTrivyPodWatcher(t *testing.T) {
	w := NewTrivyPodWatcher(nil, 42)

	if w.clusterID != 42 {
		t.Errorf("expected clusterID=42, got %d", w.clusterID)
	}
	if w.debounceWindow != defaultDebounceWindow {
		t.Errorf("expected default debounce window, got %v", w.debounceWindow)
	}
	if w.SeenCount() != 0 {
		t.Error("expected empty seen map")
	}
}

// ---------------------------------------------------------------------------
// onPodAdd / onPodUpdate — type assertion
// ---------------------------------------------------------------------------

func TestTrivyPodWatcher_OnPodAdd_WrongType(t *testing.T) {
	w := NewTrivyPodWatcher(nil, 1)

	// Should not panic on wrong type
	w.onPodAdd("not-a-pod")
	if w.SeenCount() != 0 {
		t.Error("should be no-op for wrong type")
	}
}

func TestTrivyPodWatcher_OnPodUpdate_WrongType(t *testing.T) {
	w := NewTrivyPodWatcher(nil, 1)

	w.onPodUpdate("old", "new")
	if w.SeenCount() != 0 {
		t.Error("should be no-op for wrong type")
	}
}

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

func TestTrivyScanSourceInformer(t *testing.T) {
	if TrivyScanSourceInformer != "informer" {
		t.Errorf("expected 'informer', got %s", TrivyScanSourceInformer)
	}
}
