package tekton

import (
	"context"
	"errors"
	"fmt"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	discoveryfake "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// fakeResolver is a test implementation of ClusterResolver.
type fakeResolver struct {
	dyn     dynamic.Interface
	disc    discovery.DiscoveryInterface
	k8s     kubernetes.Interface
	dynErr  error
	discErr error
	k8sErr  error
}

func (f *fakeResolver) Dynamic(uint) (dynamic.Interface, error) {
	if f.dynErr != nil {
		return nil, f.dynErr
	}
	return f.dyn, nil
}

func (f *fakeResolver) Discovery(uint) (discovery.DiscoveryInterface, error) {
	if f.discErr != nil {
		return nil, f.discErr
	}
	return f.disc, nil
}

func (f *fakeResolver) Kubernetes(uint) (kubernetes.Interface, error) {
	if f.k8sErr != nil {
		return nil, f.k8sErr
	}
	if f.k8s != nil {
		return f.k8s, nil
	}
	return k8sfake.NewSimpleClientset(), nil
}

// newResolverWithTektonInstalled returns a resolver whose fake discovery
// advertises tekton.dev/v1 resources and whose fake dynamic has no objects.
func newResolverWithTektonInstalled(t *testing.T) *fakeResolver {
	t.Helper()
	fc := &clienttesting.Fake{}
	fc.Resources = []*metav1.APIResourceList{{
		GroupVersion: "tekton.dev/v1",
		APIResources: []metav1.APIResource{
			{Name: "pipelineruns", Namespaced: true, Kind: "PipelineRun"},
			{Name: "taskruns", Namespaced: true, Kind: "TaskRun"},
		},
	}}
	disc := &discoveryfake.FakeDiscovery{Fake: fc}

	scheme := runtime.NewScheme()
	dyn := dynamicfake.NewSimpleDynamicClient(scheme)
	return &fakeResolver{dyn: dyn, disc: disc}
}

// newResolverWithoutTekton returns a resolver whose discovery returns
// nothing on the Tekton group-version.
func newResolverWithoutTekton(t *testing.T) *fakeResolver {
	t.Helper()
	fc := &clienttesting.Fake{}
	fc.Resources = []*metav1.APIResourceList{} // empty
	disc := &discoveryfake.FakeDiscovery{Fake: fc}

	scheme := runtime.NewScheme()
	dyn := dynamicfake.NewSimpleDynamicClient(scheme)
	return &fakeResolver{dyn: dyn, disc: disc}
}

// newAdapter constructs an adapter for tests. clusterID defaults to 1.
func newAdapter(t *testing.T, resolver ClusterResolver, extraJSON string) *Adapter {
	t.Helper()
	id := uint(1)
	cfg := &models.CIEngineConfig{
		Name:       "tk",
		EngineType: "tekton",
		ClusterID:  &id,
		ExtraJSON:  extraJSON,
	}
	a, err := NewAdapter(cfg, resolver)
	if err != nil {
		t.Fatalf("NewAdapter: %v", err)
	}
	return a
}

// ---------------------------------------------------------------------------
// NewAdapter
// ---------------------------------------------------------------------------

func TestNewAdapter_NilConfig(t *testing.T) {
	if _, err := NewAdapter(nil, nil); !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestNewAdapter_MissingClusterID(t *testing.T) {
	_, err := NewAdapter(&models.CIEngineConfig{EngineType: "tekton"}, nil)
	if !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput when ClusterID missing, got %v", err)
	}
}

func TestNewAdapter_ZeroClusterID(t *testing.T) {
	zero := uint(0)
	_, err := NewAdapter(&models.CIEngineConfig{
		EngineType: "tekton",
		ClusterID:  &zero,
	}, nil)
	if !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestNewAdapter_InvalidExtraJSON(t *testing.T) {
	id := uint(1)
	_, err := NewAdapter(&models.CIEngineConfig{
		EngineType: "tekton",
		ClusterID:  &id,
		ExtraJSON:  `{bad`,
	}, nil)
	if !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestNewAdapter_NilResolver_OK(t *testing.T) {
	// Metadata-only paths must work even without a resolver; requireResolver
	// is checked lazily inside each execution method.
	id := uint(1)
	a, err := NewAdapter(&models.CIEngineConfig{
		EngineType: "tekton",
		ClusterID:  &id,
	}, nil)
	if err != nil {
		t.Fatalf("NewAdapter with nil resolver should succeed, got %v", err)
	}
	if a.resolver != nil {
		t.Fatal("resolver should be nil")
	}
}

// ---------------------------------------------------------------------------
// Type / Capabilities
// ---------------------------------------------------------------------------

func TestAdapter_Type(t *testing.T) {
	a := newAdapter(t, newResolverWithTektonInstalled(t), "")
	if a.Type() != engine.EngineTekton {
		t.Fatalf("Type = %q", a.Type())
	}
}

func TestAdapter_Capabilities_Contract(t *testing.T) {
	a := newAdapter(t, newResolverWithTektonInstalled(t), "")
	caps := a.Capabilities()
	want := engine.EngineCapabilities{
		SupportsDAG:          true,
		SupportsMatrix:       true,
		SupportsArtifacts:    false,
		SupportsSecrets:      true,
		SupportsCaching:      false,
		SupportsApprovals:    false,
		SupportsNotification: false,
		SupportsLiveLog:      true,
	}
	if caps != want {
		t.Fatalf("Capabilities mismatch\n got: %+v\nwant: %+v", caps, want)
	}
}

// ---------------------------------------------------------------------------
// IsAvailable / Version
// ---------------------------------------------------------------------------

func TestAdapter_IsAvailable_True_WhenTektonInstalled(t *testing.T) {
	a := newAdapter(t, newResolverWithTektonInstalled(t), "")
	if !a.IsAvailable(context.Background()) {
		t.Fatal("IsAvailable = false despite Tekton CRDs being advertised")
	}
}

func TestAdapter_IsAvailable_False_WhenTektonAbsent(t *testing.T) {
	a := newAdapter(t, newResolverWithoutTekton(t), "")
	if a.IsAvailable(context.Background()) {
		t.Fatal("IsAvailable = true despite empty discovery list")
	}
}

func TestAdapter_IsAvailable_NeverErrors_OnDiscoveryError(t *testing.T) {
	a := newAdapter(t, &fakeResolver{discErr: fmt.Errorf("connection refused")}, "")
	// Contract: must return false (not panic, not propagate error).
	if a.IsAvailable(context.Background()) {
		t.Fatal("IsAvailable should be false when discovery errors")
	}
}

func TestAdapter_IsAvailable_NeverErrors_OnNilResolver(t *testing.T) {
	id := uint(1)
	a, err := NewAdapter(&models.CIEngineConfig{EngineType: "tekton", ClusterID: &id}, nil)
	if err != nil {
		t.Fatalf("NewAdapter: %v", err)
	}
	if a.IsAvailable(context.Background()) {
		t.Fatal("IsAvailable should be false when resolver nil")
	}
}

func TestAdapter_Version_ReturnsGroupVersion(t *testing.T) {
	a := newAdapter(t, newResolverWithTektonInstalled(t), "")
	v, err := a.Version(context.Background())
	if err != nil {
		t.Fatalf("Version: %v", err)
	}
	if v != "tekton.dev/v1" {
		t.Fatalf("Version = %q, want tekton.dev/v1", v)
	}
}

func TestAdapter_Version_ErrorsOnNoResolver(t *testing.T) {
	id := uint(1)
	a, err := NewAdapter(&models.CIEngineConfig{EngineType: "tekton", ClusterID: &id}, nil)
	if err != nil {
		t.Fatalf("NewAdapter: %v", err)
	}
	_, err = a.Version(context.Background())
	if !errors.Is(err, engine.ErrUnavailable) {
		t.Fatalf("expected ErrUnavailable, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Tests for Cancel / StreamLogs / GetArtifacts live in cancel_test.go,
// logs_test.go, and artifacts_test.go. Adapter-level tests focus on
// construction + metadata methods.
// ---------------------------------------------------------------------------
