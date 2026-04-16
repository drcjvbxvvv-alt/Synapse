package argo

import (
	"context"
	"errors"
	"fmt"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

// newResolverArgoInstalled returns a resolver whose discovery advertises
// argoproj.io/v1alpha1 Workflow resources.
func newResolverArgoInstalled(t *testing.T, objects ...runtime.Object) *fakeResolver {
	t.Helper()
	fc := &clienttesting.Fake{Resources: []*metav1.APIResourceList{{
		GroupVersion: "argoproj.io/v1alpha1",
		APIResources: []metav1.APIResource{
			{Name: "workflows", Namespaced: true, Kind: "Workflow"},
		},
	}}}
	disc := &discoveryfake.FakeDiscovery{Fake: fc}

	scheme := runtime.NewScheme()
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme,
		map[schema.GroupVersionResource]string{gvrWorkflow: "WorkflowList"},
		objects...,
	)
	return &fakeResolver{dyn: dyn, disc: disc}
}

// newResolverArgoAbsent returns a resolver whose discovery returns nothing.
func newResolverArgoAbsent(t *testing.T) *fakeResolver {
	t.Helper()
	fc := &clienttesting.Fake{Resources: []*metav1.APIResourceList{}}
	disc := &discoveryfake.FakeDiscovery{Fake: fc}
	scheme := runtime.NewScheme()
	return &fakeResolver{
		dyn:  dynamicfake.NewSimpleDynamicClient(scheme),
		disc: disc,
	}
}

func newAdapter(t *testing.T, resolver ClusterResolver, extraJSON string) *Adapter {
	t.Helper()
	id := uint(1)
	a, err := NewAdapter(&models.CIEngineConfig{
		Name:       "argo",
		EngineType: "argo",
		ClusterID:  &id,
		ExtraJSON:  extraJSON,
	}, resolver)
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
	_, err := NewAdapter(&models.CIEngineConfig{EngineType: "argo"}, nil)
	if !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestNewAdapter_ZeroClusterID(t *testing.T) {
	zero := uint(0)
	_, err := NewAdapter(&models.CIEngineConfig{
		EngineType: "argo", ClusterID: &zero,
	}, nil)
	if !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestNewAdapter_InvalidExtraJSON(t *testing.T) {
	id := uint(1)
	_, err := NewAdapter(&models.CIEngineConfig{
		EngineType: "argo", ClusterID: &id, ExtraJSON: `{bad`,
	}, nil)
	if !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Type / Capabilities
// ---------------------------------------------------------------------------

func TestAdapter_Type(t *testing.T) {
	a := newAdapter(t, newResolverArgoInstalled(t), "")
	if a.Type() != engine.EngineArgo {
		t.Fatalf("Type = %q", a.Type())
	}
}

func TestAdapter_Capabilities_Contract(t *testing.T) {
	a := newAdapter(t, newResolverArgoInstalled(t), "")
	caps := a.Capabilities()
	want := engine.EngineCapabilities{
		SupportsDAG:          true,
		SupportsMatrix:       false,
		SupportsArtifacts:    true,
		SupportsSecrets:      true,
		SupportsCaching:      false,
		SupportsApprovals:    true,
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

func TestAdapter_IsAvailable_True(t *testing.T) {
	a := newAdapter(t, newResolverArgoInstalled(t), "")
	if !a.IsAvailable(context.Background()) {
		t.Fatal("IsAvailable = false despite CRDs advertised")
	}
}

func TestAdapter_IsAvailable_False_WhenAbsent(t *testing.T) {
	a := newAdapter(t, newResolverArgoAbsent(t), "")
	if a.IsAvailable(context.Background()) {
		t.Fatal("IsAvailable = true for empty discovery")
	}
}

func TestAdapter_IsAvailable_NilResolver(t *testing.T) {
	id := uint(1)
	a, _ := NewAdapter(&models.CIEngineConfig{EngineType: "argo", ClusterID: &id}, nil)
	if a.IsAvailable(context.Background()) {
		t.Fatal("IsAvailable must be false with nil resolver")
	}
}

func TestAdapter_IsAvailable_DiscoveryError(t *testing.T) {
	a := newAdapter(t, &fakeResolver{discErr: fmt.Errorf("dial tcp: refused")}, "")
	if a.IsAvailable(context.Background()) {
		t.Fatal("IsAvailable must be false when discovery errors")
	}
}

func TestAdapter_Version_ReturnsGroupVersion(t *testing.T) {
	a := newAdapter(t, newResolverArgoInstalled(t), "")
	v, err := a.Version(context.Background())
	if err != nil {
		t.Fatalf("Version: %v", err)
	}
	if v != "argoproj.io/v1alpha1" {
		t.Fatalf("Version = %q", v)
	}
}

func TestAdapter_Version_NilResolver(t *testing.T) {
	id := uint(1)
	a, _ := NewAdapter(&models.CIEngineConfig{EngineType: "argo", ClusterID: &id}, nil)
	if _, err := a.Version(context.Background()); !errors.Is(err, engine.ErrUnavailable) {
		t.Fatalf("expected ErrUnavailable, got %v", err)
	}
}

// Tests for Cancel / StreamLogs / GetArtifacts live in their dedicated files
// (cancel_test.go, logs_test.go, artifacts_test.go).

// ---------------------------------------------------------------------------
// Config + errors helpers
// ---------------------------------------------------------------------------

func TestParseExtra_Empty(t *testing.T) {
	cfg, err := parseExtra("")
	if err != nil {
		t.Fatalf("parseExtra: %v", err)
	}
	if cfg == nil || cfg.WorkflowTemplateName != "" {
		t.Fatalf("expected zero cfg, got %+v", cfg)
	}
}

func TestParseExtra_Valid(t *testing.T) {
	cfg, err := parseExtra(`{"workflow_template_name":"wt","namespace":"ci","service_account_name":"sa"}`)
	if err != nil {
		t.Fatalf("parseExtra: %v", err)
	}
	if cfg.WorkflowTemplateName != "wt" || cfg.Namespace != "ci" || cfg.ServiceAccountName != "sa" {
		t.Fatalf("cfg = %+v", cfg)
	}
}

func TestParseExtra_Malformed(t *testing.T) {
	if _, err := parseExtra(`{bad`); !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestRequireTargets(t *testing.T) {
	if _, _, err := (*ExtraConfig)(nil).requireTargets(); !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("nil: %v", err)
	}
	if _, _, err := (&ExtraConfig{}).requireTargets(); !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("empty: %v", err)
	}
	if _, _, err := (&ExtraConfig{WorkflowTemplateName: "wt"}).requireTargets(); !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("missing ns: %v", err)
	}
	wt, ns, err := (&ExtraConfig{WorkflowTemplateName: " wt ", Namespace: " ci "}).requireTargets()
	if err != nil || wt != "wt" || ns != "ci" {
		t.Fatalf("trim failed: %q %q %v", wt, ns, err)
	}
}

func TestRequireNamespace(t *testing.T) {
	if _, err := (*ExtraConfig)(nil).requireNamespace(); !errors.Is(err, engine.ErrInvalidInput) {
		t.Fatalf("nil: %v", err)
	}
	if ns, err := (&ExtraConfig{Namespace: " ci "}).requireNamespace(); err != nil || ns != "ci" {
		t.Fatalf("got %q %v", ns, err)
	}
}

func TestMapK8sError_Nil(t *testing.T) {
	if mapK8sError(nil) != nil {
		t.Fatal("nil input should return nil")
	}
}

func TestRequireResolver_Nil(t *testing.T) {
	if err := requireResolver(nil); !errors.Is(err, engine.ErrUnavailable) {
		t.Fatalf("got %v", err)
	}
}
