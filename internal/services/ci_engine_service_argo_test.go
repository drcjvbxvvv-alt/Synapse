package services

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
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

	"github.com/shaia/Synapse/internal/services/pipeline/engine"
	"github.com/shaia/Synapse/internal/services/pipeline/engine/argo"
)

type fakeArgoResolver struct {
	dyn  dynamic.Interface
	disc discovery.DiscoveryInterface
}

func (r *fakeArgoResolver) Dynamic(uint) (dynamic.Interface, error)               { return r.dyn, nil }
func (r *fakeArgoResolver) Discovery(uint) (discovery.DiscoveryInterface, error)  { return r.disc, nil }
func (r *fakeArgoResolver) Kubernetes(uint) (kubernetes.Interface, error)          { return k8sfake.NewSimpleClientset(), nil }

func TestCIEngineService_ListAvailableEngines_IncludesRegisteredArgo(t *testing.T) {
	scheme := runtime.NewScheme()
	gvr := schema.GroupVersionResource{Group: "argoproj.io", Version: "v1alpha1", Resource: "workflows"}
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		gvr: "WorkflowList",
	})
	fc := &clienttesting.Fake{Resources: []*metav1.APIResourceList{{
		GroupVersion: "argoproj.io/v1alpha1",
		APIResources: []metav1.APIResource{
			{Name: "workflows", Namespaced: true, Kind: "Workflow"},
		},
	}}}
	resolver := &fakeArgoResolver{dyn: dyn, disc: &discoveryfake.FakeDiscovery{Fake: fc}}

	f := engine.NewFactory()
	require.NoError(t, argo.Register(f, resolver))

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
			1, "argo-main", "argo", true, 7,
			`{"workflow_template_name":"build-app","namespace":"ci"}`,
		))

	got, err := svc.ListAvailableEngines(context.Background())
	require.NoError(t, err)
	require.Len(t, got, 2)

	ag := got[1]
	require.Equal(t, "argo", ag.Type)
	require.True(t, ag.Available, "argo should be reachable, error=%s", ag.Error)
	require.Equal(t, "argoproj.io/v1alpha1", ag.Version)
	require.NotNil(t, ag.Capabilities)
	require.True(t, ag.Capabilities.SupportsDAG)
	require.True(t, ag.Capabilities.SupportsLiveLog) // M19a: Pod log streaming implemented
}
