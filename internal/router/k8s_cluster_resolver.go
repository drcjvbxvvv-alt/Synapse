package router

import (
	"fmt"

	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"

	"github.com/shaia/Synapse/internal/k8s"
	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

// k8sClusterResolver bridges the ClusterResolver interfaces used by every
// K8s-native CI engine adapter (Tekton M18d, Argo Workflows M18e, future
// adapters).
//
// The router layer is the natural home for this bridge because:
//   - It has *direct* access to *k8s.ClusterInformerManager.
//   - Placing the struct inside an adapter package would force an
//     internal/k8s ↔ internal/services/pipeline/engine/<adapter> import
//     cycle that ADR-015 explicitly avoided.
//
// Go's structural interface typing means this single struct satisfies
// both tekton.ClusterResolver and argo.ClusterResolver (and any future
// CRD-based adapter with the same 2-method shape), so we don't need one
// wrapper per adapter.
//
// The resolver relies on the cluster having been previously initialised
// via ClusterInformerManager.EnsureForCluster() — that's already how
// every other routes_*.go file reaches K8s clients.
type k8sClusterResolver struct {
	mgr *k8s.ClusterInformerManager
}

// newK8sClusterResolver constructs the shared resolver.
func newK8sClusterResolver(mgr *k8s.ClusterInformerManager) *k8sClusterResolver {
	return &k8sClusterResolver{mgr: mgr}
}

// Dynamic returns an unstructured client for the target cluster.
func (r *k8sClusterResolver) Dynamic(clusterID uint) (dynamic.Interface, error) {
	c := r.mgr.GetK8sClientByID(clusterID)
	if c == nil {
		return nil, fmt.Errorf("router: no cached K8sClient for cluster %d: %w",
			clusterID, engine.ErrUnavailable)
	}
	cfg := c.GetRestConfig()
	if cfg == nil {
		return nil, fmt.Errorf("router: cluster %d has nil rest.Config: %w",
			clusterID, engine.ErrUnavailable)
	}
	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("router: dynamic client for cluster %d: %w: %w",
			clusterID, err, engine.ErrUnavailable)
	}
	return dyn, nil
}

// Discovery returns a DiscoveryInterface — reusing the clientset's
// built-in discovery is cheaper than building one from rest.Config.
func (r *k8sClusterResolver) Discovery(clusterID uint) (discovery.DiscoveryInterface, error) {
	c := r.mgr.GetK8sClientByID(clusterID)
	if c == nil {
		return nil, fmt.Errorf("router: no cached K8sClient for cluster %d: %w",
			clusterID, engine.ErrUnavailable)
	}
	cs := c.GetClientset()
	if cs == nil {
		return nil, fmt.Errorf("router: cluster %d has nil clientset: %w",
			clusterID, engine.ErrUnavailable)
	}
	return cs.Discovery(), nil
}

// Kubernetes returns the typed clientset for streaming Pod logs and other
// CoreV1 operations that the dynamic client does not support.
func (r *k8sClusterResolver) Kubernetes(clusterID uint) (kubernetes.Interface, error) {
	c := r.mgr.GetK8sClientByID(clusterID)
	if c == nil {
		return nil, fmt.Errorf("router: no cached K8sClient for cluster %d: %w",
			clusterID, engine.ErrUnavailable)
	}
	cs := c.GetClientset()
	if cs == nil {
		return nil, fmt.Errorf("router: cluster %d has nil clientset: %w",
			clusterID, engine.ErrUnavailable)
	}
	return cs, nil
}
