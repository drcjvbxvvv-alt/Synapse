package services

import (
	"context"
	"fmt"
	"strings"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/shaia/Synapse/pkg/logger"
)

// AnnotationCrossCluster is the service annotation key for declaring a
// cross-cluster dependency.
//
// Value format: "<targetClusterID>/<namespace>/<service-name>"
// Example:      "42/production/payment-service"
const AnnotationCrossCluster = "synapse.io/cross-cluster"

// ─── DTOs ──────────────────────────────────────────────────────────────────

// ClusterTopoInput is a work item for GetMultiClusterTopology.
type ClusterTopoInput struct {
	ID        uint
	Name      string
	Clientset kubernetes.Interface
}

// ClusterSection holds the per-cluster topology slice with all node/edge IDs
// already globalised (prefixed with the cluster ID).
type ClusterSection struct {
	ClusterID   uint          `json:"clusterId"`
	ClusterName string        `json:"clusterName"`
	Nodes       []NetworkNode `json:"nodes"`
	Edges       []NetworkEdge `json:"edges"`
}

// CrossEdge represents a detected cross-cluster connection.
type CrossEdge struct {
	SourceClusterID uint   `json:"sourceClusterId"`
	TargetClusterID uint   `json:"targetClusterId"`
	Source          string `json:"source"` // globalised node ID in source cluster
	Target          string `json:"target"` // globalised node ID in target cluster
	Kind            string `json:"kind"`   // "annotation"
	Label           string `json:"label,omitempty"`
}

// MultiClusterTopology is the aggregated federation view.
type MultiClusterTopology struct {
	Clusters   []ClusterSection `json:"clusters"`
	CrossEdges []CrossEdge      `json:"crossEdges"`
}

// ─── Internal types ────────────────────────────────────────────────────────

// mcResult is the output of a single parallel topology fetch.
type mcResult struct {
	idx   int
	id    uint
	name  string
	topo  *ClusterNetworkTopology
	err   error
}

// ─── Helpers ───────────────────────────────────────────────────────────────

// globalNodeID prefixes a local node ID with the cluster ID for global uniqueness.
// e.g. globalNodeID(42, "workload/default/Deployment/nginx") → "42:workload/default/Deployment/nginx"
func globalNodeID(clusterID uint, localID string) string {
	return fmt.Sprintf("%d:%s", clusterID, localID)
}

// ─── Service function ──────────────────────────────────────────────────────

// GetMultiClusterTopology fetches topology for multiple clusters in parallel,
// globalises all node/edge IDs, and detects cross-cluster edges via the
// synapse.io/cross-cluster service annotation.
//
// Clusters that fail to respond are included as empty sections (graceful
// degradation), so the frontend can still render the other clusters.
func GetMultiClusterTopology(
	ctx context.Context,
	clusters []ClusterTopoInput,
) (*MultiClusterTopology, error) {
	if len(clusters) == 0 {
		return &MultiClusterTopology{
			Clusters:   []ClusterSection{},
			CrossEdges: []CrossEdge{},
		}, nil
	}

	// ── 1. Fetch per-cluster topology in parallel ──────────────────────────
	results := make([]mcResult, len(clusters))
	var wg sync.WaitGroup
	for i, c := range clusters {
		wg.Add(1)
		go func(idx int, ci ClusterTopoInput) {
			defer wg.Done()
			topo, err := GetClusterNetworkTopology(ctx, ci.Clientset, nil)
			results[idx] = mcResult{idx: idx, id: ci.ID, name: ci.Name, topo: topo, err: err}
		}(i, c)
	}
	wg.Wait()

	// ── 2. Globalise IDs and build sections ───────────────────────────────
	sections := make([]ClusterSection, 0, len(clusters))
	// nodeExists tracks globalised IDs for cross-edge validation.
	nodeExists := map[string]bool{}

	for _, r := range results {
		if r.err != nil {
			logger.Warn("multi-cluster topology: fetch failed",
				"cluster_id", r.id, "cluster_name", r.name, "error", r.err)
			sections = append(sections, ClusterSection{
				ClusterID:   r.id,
				ClusterName: r.name,
				Nodes:       []NetworkNode{},
				Edges:       []NetworkEdge{},
			})
			continue
		}

		sec := ClusterSection{
			ClusterID:   r.id,
			ClusterName: r.name,
			Nodes:       make([]NetworkNode, 0, len(r.topo.Nodes)),
			Edges:       make([]NetworkEdge, 0, len(r.topo.Edges)),
		}
		for _, n := range r.topo.Nodes {
			gn := n
			gn.ID = globalNodeID(r.id, n.ID)
			sec.Nodes = append(sec.Nodes, gn)
			nodeExists[gn.ID] = true
		}
		for _, e := range r.topo.Edges {
			ge := e
			ge.Source = globalNodeID(r.id, e.Source)
			ge.Target = globalNodeID(r.id, e.Target)
			sec.Edges = append(sec.Edges, ge)
		}
		sections = append(sections, sec)
	}

	// ── 3. Build service index for cross-edge lookup ───────────────────────
	// svcIndex: clusterID → "namespace/name" → globalNodeID
	svcIndex := map[uint]map[string]string{}
	for _, sec := range sections {
		m := map[string]string{}
		for _, n := range sec.Nodes {
			if n.Kind == "Service" {
				m[n.Namespace+"/"+n.Name] = n.ID
			}
		}
		svcIndex[sec.ClusterID] = m
	}

	// ── 4. Detect cross-cluster edges (annotation-based) ──────────────────
	var crossEdges []CrossEdge
	for i, c := range clusters {
		if results[i].err != nil {
			continue
		}
		edges := detectAnnotationCrossEdges(ctx, c, svcIndex, nodeExists)
		crossEdges = append(crossEdges, edges...)
	}
	if crossEdges == nil {
		crossEdges = []CrossEdge{}
	}

	return &MultiClusterTopology{
		Clusters:   sections,
		CrossEdges: crossEdges,
	}, nil
}

// detectAnnotationCrossEdges queries a cluster's services for the
// synapse.io/cross-cluster annotation and returns CrossEdge entries where
// the annotation value can be resolved to an actual node in another section.
func detectAnnotationCrossEdges(
	ctx context.Context,
	src ClusterTopoInput,
	svcIndex map[uint]map[string]string,
	nodeExists map[string]bool,
) []CrossEdge {
	svcList, err := src.Clientset.CoreV1().Services("").List(ctx, metav1.ListOptions{})
	if err != nil {
		logger.Warn("cross-cluster detection: list services failed",
			"cluster_id", src.ID, "error", err)
		return nil
	}

	var edges []CrossEdge
	for _, svc := range svcList.Items {
		ann, ok := svc.Annotations[AnnotationCrossCluster]
		if !ok {
			continue
		}
		// Parse "targetClusterID/namespace/svc-name"
		parts := strings.SplitN(ann, "/", 3)
		if len(parts) != 3 {
			logger.Warn("cross-cluster annotation: invalid format",
				"cluster_id", src.ID,
				"svc", svc.Namespace+"/"+svc.Name,
				"annotation", ann)
			continue
		}
		var targetClusterID uint
		if _, err := fmt.Sscanf(parts[0], "%d", &targetClusterID); err != nil {
			logger.Warn("cross-cluster annotation: non-numeric cluster ID",
				"cluster_id", src.ID, "value", parts[0])
			continue
		}
		targetNs := parts[1]
		targetSvcName := parts[2]

		targetGlobalID, found := svcIndex[targetClusterID][targetNs+"/"+targetSvcName]
		if !found {
			// Target cluster not in this query or service doesn't exist — skip silently
			continue
		}

		sourceGlobalID := globalNodeID(src.ID, networkServiceNodeID(svc.Namespace, svc.Name))
		if !nodeExists[sourceGlobalID] {
			continue
		}

		edges = append(edges, CrossEdge{
			SourceClusterID: src.ID,
			TargetClusterID: targetClusterID,
			Source:          sourceGlobalID,
			Target:          targetGlobalID,
			Kind:            "annotation",
			Label:           fmt.Sprintf("%s → cluster %d", svc.Name, targetClusterID),
		})
	}
	return edges
}
