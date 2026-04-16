package argo

import (
	"fmt"

	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"

	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

// ClusterResolver abstracts "given a Synapse-managed cluster id, give me
// enough K8s client machinery to talk to Argo Workflows in it".
//
// Defined locally (rather than importing a shared one from tekton) so that
// neither adapter depends on the other. Both interfaces happen to be
// structurally identical, which is fine — Go's interface satisfaction is
// structural, so a single router-layer implementation can satisfy both
// without any type gymnastics.
//
// Same rationale as the Tekton adapter: placing this interface inside the
// engine package would technically be DRYer, but the engine package is
// deliberately kept free of k8s client-go dependencies so CI tooling that
// stubs out external adapters (tests, lightweight builds) never pulls in
// ~20 MB of K8s libraries.
type ClusterResolver interface {
	Dynamic(clusterID uint) (dynamic.Interface, error)
	Discovery(clusterID uint) (discovery.DiscoveryInterface, error)
	// Kubernetes returns a typed clientset. Used by StreamLogs to stream
	// Pod container logs via CoreV1().Pods().GetLogs().
	Kubernetes(clusterID uint) (kubernetes.Interface, error)
}

// requireResolver is a tiny guard used at method entry.
func requireResolver(r ClusterResolver) error {
	if r == nil {
		return fmt.Errorf("argo: cluster resolver not configured: %w", engine.ErrUnavailable)
	}
	return nil
}
