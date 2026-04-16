package tekton

import (
	"fmt"

	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"

	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

// ClusterResolver abstracts "given a Synapse-managed cluster id, give me
// enough K8s client machinery to talk to Tekton in it".
//
// Keeping this as an interface (rather than directly importing
// internal/k8s.ClusterInformerManager) avoids the dependency cycle:
//
//	internal/k8s → internal/services → …  cannot depend back on engine.
//
// The router provides a concrete implementation at startup by wrapping its
// k8sMgr (see routes_ci_engine.go).
type ClusterResolver interface {
	// Dynamic returns an unstructured client for the cluster identified by
	// clusterID. Callers use it to CRUD `PipelineRun` / `TaskRun` CRDs.
	Dynamic(clusterID uint) (dynamic.Interface, error)

	// Discovery returns a discovery client for probing API groups. Used
	// by IsAvailable() to detect whether Tekton CRDs are installed.
	Discovery(clusterID uint) (discovery.DiscoveryInterface, error)

	// Kubernetes returns a typed clientset for the cluster. Used by
	// StreamLogs to stream Pod container logs via CoreV1().Pods().GetLogs().
	Kubernetes(clusterID uint) (kubernetes.Interface, error)
}

// requireResolver is a tiny guard used at method entry: if the adapter was
// constructed without a resolver (edge case: unit tests exercising only
// metadata methods), execution paths fail gracefully with ErrUnavailable.
func requireResolver(r ClusterResolver) error {
	if r == nil {
		return fmt.Errorf("tekton: cluster resolver not configured: %w", engine.ErrUnavailable)
	}
	return nil
}
