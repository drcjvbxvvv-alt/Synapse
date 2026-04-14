package services

import (
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// ---------------------------------------------------------------------------
// parseAPIVersion
// ---------------------------------------------------------------------------

func TestParseAPIVersion_Core(t *testing.T) {
	group, version := parseAPIVersion("v1")
	if group != "" {
		t.Errorf("expected empty group, got %s", group)
	}
	if version != "v1" {
		t.Errorf("expected v1, got %s", version)
	}
}

func TestParseAPIVersion_WithGroup(t *testing.T) {
	group, version := parseAPIVersion("apps/v1")
	if group != "apps" {
		t.Errorf("expected apps, got %s", group)
	}
	if version != "v1" {
		t.Errorf("expected v1, got %s", version)
	}
}

func TestParseAPIVersion_Nested(t *testing.T) {
	group, version := parseAPIVersion("networking.k8s.io/v1")
	if group != "networking.k8s.io" {
		t.Errorf("expected networking.k8s.io, got %s", group)
	}
	if version != "v1" {
		t.Errorf("expected v1, got %s", version)
	}
}

// ---------------------------------------------------------------------------
// resolveGVR
// ---------------------------------------------------------------------------

func TestResolveGVR_KnownResource(t *testing.T) {
	gvr, err := resolveGVR("apps/v1", "Deployment")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gvr.Group != "apps" {
		t.Errorf("expected apps, got %s", gvr.Group)
	}
	if gvr.Resource != "deployments" {
		t.Errorf("expected deployments, got %s", gvr.Resource)
	}
}

func TestResolveGVR_CoreResource(t *testing.T) {
	gvr, err := resolveGVR("v1", "ConfigMap")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gvr.Group != "" {
		t.Errorf("expected empty group, got %s", gvr.Group)
	}
	if gvr.Resource != "configmaps" {
		t.Errorf("expected configmaps, got %s", gvr.Resource)
	}
}

func TestResolveGVR_UnknownResource(t *testing.T) {
	gvr, err := resolveGVR("custom.io/v1beta1", "Widget")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gvr.Group != "custom.io" {
		t.Errorf("expected custom.io, got %s", gvr.Group)
	}
	if gvr.Version != "v1beta1" {
		t.Errorf("expected v1beta1, got %s", gvr.Version)
	}
	if gvr.Resource != "widgets" {
		t.Errorf("expected widgets (auto-pluralized), got %s", gvr.Resource)
	}
}

// ---------------------------------------------------------------------------
// isSystemAnnotation
// ---------------------------------------------------------------------------

func TestIsSystemAnnotation_System(t *testing.T) {
	systemKeys := []string{
		"kubectl.kubernetes.io/last-applied-configuration",
		"deployment.kubernetes.io/revision",
		"kubernetes.io/change-cause",
		"meta.helm.sh/release-name",
	}
	for _, k := range systemKeys {
		if !isSystemAnnotation(k) {
			t.Errorf("expected %s to be system annotation", k)
		}
	}
}

func TestIsSystemAnnotation_UserAnnotation(t *testing.T) {
	userKeys := []string{
		"app.example.com/version",
		"synapse.io/managed",
		"team",
	}
	for _, k := range userKeys {
		if isSystemAnnotation(k) {
			t.Errorf("expected %s to NOT be system annotation", k)
		}
	}
}

// ---------------------------------------------------------------------------
// compareResource
// ---------------------------------------------------------------------------

func TestCompareResource_NoChanges(t *testing.T) {
	desired := ResourceManifest{
		Kind: "Deployment",
		Name: "app",
		Labels: map[string]string{"app": "web"},
		Spec:   map[string]interface{}{"replicas": "3"},
	}
	actual := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"labels": map[string]interface{}{"app": "web"},
			},
			"spec": map[string]interface{}{"replicas": "3"},
		},
	}

	diffs := compareResource(desired, actual)
	if len(diffs) != 0 {
		t.Errorf("expected no diffs, got %d: %v", len(diffs), diffs)
	}
}

func TestCompareResource_LabelDrift(t *testing.T) {
	desired := ResourceManifest{
		Kind:   "Deployment",
		Name:   "app",
		Labels: map[string]string{"app": "web", "version": "v2"},
	}
	actual := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"labels": map[string]interface{}{"app": "web", "version": "v1"},
			},
		},
	}

	diffs := compareResource(desired, actual)
	if len(diffs) != 1 {
		t.Fatalf("expected 1 diff, got %d: %v", len(diffs), diffs)
	}
	if !strings.Contains(diffs[0].Path, "version") {
		t.Errorf("diff should be about version label: %v", diffs[0])
	}
	if diffs[0].Expected != "v2" {
		t.Errorf("expected v2, got %s", diffs[0].Expected)
	}
}

func TestCompareResource_SpecDrift(t *testing.T) {
	desired := ResourceManifest{
		Kind: "Deployment",
		Name: "app",
		Spec: map[string]interface{}{"replicas": "3"},
	}
	actual := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"spec": map[string]interface{}{"replicas": "1"},
		},
	}

	diffs := compareResource(desired, actual)
	if len(diffs) != 1 {
		t.Fatalf("expected 1 diff, got %d", len(diffs))
	}
	if diffs[0].Path != "spec.replicas" {
		t.Errorf("expected spec.replicas, got %s", diffs[0].Path)
	}
}

func TestCompareResource_SkipsSystemAnnotations(t *testing.T) {
	desired := ResourceManifest{
		Kind: "Deployment",
		Name: "app",
		Annotations: map[string]string{
			"kubectl.kubernetes.io/last-applied-configuration": "old",
			"app.example.com/version":                          "v2",
		},
	}
	actual := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"annotations": map[string]interface{}{
					"kubectl.kubernetes.io/last-applied-configuration": "new",
					"app.example.com/version":                          "v1",
				},
			},
		},
	}

	diffs := compareResource(desired, actual)
	// Should only detect the user annotation diff, not the kubectl one
	if len(diffs) != 1 {
		t.Fatalf("expected 1 diff (skipping system annotation), got %d: %v", len(diffs), diffs)
	}
	if !strings.Contains(diffs[0].Path, "app.example.com/version") {
		t.Errorf("diff should be about user annotation: %v", diffs[0])
	}
}

// ---------------------------------------------------------------------------
// buildDiffSummary
// ---------------------------------------------------------------------------

func TestBuildDiffSummary_Mixed(t *testing.T) {
	diffs := []DiffItem{
		{DiffType: "added"},
		{DiffType: "modified"},
		{DiffType: "modified"},
		{DiffType: "unchanged"},
		{DiffType: "unchanged"},
		{DiffType: "unchanged"},
	}
	summary := buildDiffSummary(diffs)
	if !strings.Contains(summary, "1 to add") {
		t.Errorf("summary should mention added: %s", summary)
	}
	if !strings.Contains(summary, "2 drifted") {
		t.Errorf("summary should mention drifted: %s", summary)
	}
	if !strings.Contains(summary, "3 in sync") {
		t.Errorf("summary should mention in sync: %s", summary)
	}
}

func TestBuildDiffSummary_AllSynced(t *testing.T) {
	diffs := []DiffItem{
		{DiffType: "unchanged"},
		{DiffType: "unchanged"},
	}
	summary := buildDiffSummary(diffs)
	if !strings.Contains(summary, "2 in sync") {
		t.Errorf("expected '2 in sync', got %s", summary)
	}
}

func TestBuildDiffSummary_Empty(t *testing.T) {
	summary := buildDiffSummary(nil)
	if summary != "no resources" {
		t.Errorf("expected 'no resources', got %s", summary)
	}
}

// ---------------------------------------------------------------------------
// NewGitOpsDiffEngine
// ---------------------------------------------------------------------------

func TestNewGitOpsDiffEngine(t *testing.T) {
	gitopsSvc := NewGitOpsService(nil)
	engine := NewGitOpsDiffEngine(nil, gitopsSvc)
	if engine == nil {
		t.Fatal("expected non-nil engine")
	}
}

// ---------------------------------------------------------------------------
// commonGVRMap coverage
// ---------------------------------------------------------------------------

func TestCommonGVRMap_Coverage(t *testing.T) {
	expectedKeys := []string{
		"v1/ConfigMap",
		"v1/Secret",
		"v1/Service",
		"apps/v1/Deployment",
		"apps/v1/StatefulSet",
		"apps/v1/DaemonSet",
		"batch/v1/Job",
		"batch/v1/CronJob",
	}
	for _, key := range expectedKeys {
		if _, ok := commonGVRMap[key]; !ok {
			t.Errorf("commonGVRMap missing key %s", key)
		}
	}
}
