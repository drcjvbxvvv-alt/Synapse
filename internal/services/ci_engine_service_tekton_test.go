package services

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
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
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/shaia/Synapse/internal/services/pipeline/engine"
	"github.com/shaia/Synapse/internal/services/pipeline/engine/tekton"
)

// fakeTektonResolver implements tekton.ClusterResolver for this integration
// test without pulling in the router layer.
type fakeTektonResolver struct {
	dyn  dynamic.Interface
	disc discovery.DiscoveryInterface
}

func (r *fakeTektonResolver) Dynamic(uint) (dynamic.Interface, error)               { return r.dyn, nil }
func (r *fakeTektonResolver) Discovery(uint) (discovery.DiscoveryInterface, error)  { return r.disc, nil }
func (r *fakeTektonResolver) Kubernetes(uint) (kubernetes.Interface, error)          { return k8sfake.NewSimpleClientset(), nil }

// TestCIEngineService_ListAvailableEngines_IncludesRegisteredTekton exercises
// the full integration: Register the Tekton builder with a fake resolver
// whose discovery advertises Tekton CRDs, seed ci_engine_configs, then
// verify the service reports Tekton as available.
func TestCIEngineService_ListAvailableEngines_IncludesRegisteredTekton(t *testing.T) {
	// Fake resolver with Tekton CRDs advertised.
	scheme := runtime.NewScheme()
	gvr := schema.GroupVersionResource{Group: "tekton.dev", Version: "v1", Resource: "pipelineruns"}
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		gvr: "PipelineRunList",
	})
	fc := &clienttesting.Fake{Resources: []*metav1.APIResourceList{{
		GroupVersion: "tekton.dev/v1",
		APIResources: []metav1.APIResource{
			{Name: "pipelineruns", Namespaced: true, Kind: "PipelineRun"},
		},
	}}}
	resolver := &fakeTektonResolver{dyn: dyn, disc: &discoveryfake.FakeDiscovery{Fake: fc}}

	f := engine.NewFactory()
	require.NoError(t, tekton.Register(f, resolver))

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn:                 db,
		PreferSimpleProtocol: true,
	}), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	require.NoError(t, err)
	t.Cleanup(func() {
		if sqlDB, _ := gormDB.DB(); sqlDB != nil {
			_ = sqlDB.Close()
		}
	})

	svc := NewCIEngineService(gormDB, f)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "ci_engine_configs"`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "engine_type", "enabled", "cluster_id", "extra_json",
		}).AddRow(
			1, "tekton-main", "tekton", true, 7,
			`{"pipeline_name":"build-app","namespace":"ci"}`,
		))

	got, err := svc.ListAvailableEngines(context.Background())
	require.NoError(t, err)
	require.Len(t, got, 2) // native + tekton

	tk := got[1]
	require.Equal(t, "tekton", tk.Type)
	require.True(t, tk.Available, "tekton should be available; error=%s", tk.Error)
	require.Equal(t, "tekton.dev/v1", tk.Version)
	require.NotNil(t, tk.Capabilities)
	require.True(t, tk.Capabilities.SupportsDAG)
	require.True(t, tk.Capabilities.SupportsLiveLog) // M19a: Pod log streaming implemented
}
