package services

import "strings"

// EnrichWithIstioMetrics enriches edges in-place using Istio flow metrics,
// and appends Workload→Service edges representing actual observed traffic (Phase D).
func (t *ClusterNetworkTopology) EnrichWithIstioMetrics(metrics map[string]*IstioEdgeMetrics) {
	if len(metrics) == 0 {
		return
	}

	// Index: "ns/workloadName" → node ID  (for Workload nodes)
	wlByName := map[string]string{}
	for _, n := range t.Nodes {
		if n.Kind == "Workload" {
			wlByName[n.Namespace+"/"+n.Name] = n.ID
		}
	}

	// Index: "ns/serviceName" → node ID  (for Service nodes)
	svcByName := map[string]string{}
	for _, n := range t.Nodes {
		if n.Kind == "Service" {
			svcByName[n.Namespace+"/"+n.Name] = n.ID
		}
	}

	// Enrich existing static Service→Workload edges with Istio metrics
	for i, edge := range t.Edges {
		if edge.Kind == "ingress" {
			continue // ingress edges are not Istio-managed
		}
		parts := splitWorkloadID(edge.Target)
		if parts == nil {
			continue
		}
		destKey := parts[0] + "/" + parts[1] // "ns/workloadName"
		for _, m := range metrics {
			if m.DestNamespace+"/"+m.DestWorkload == destKey {
				t.Edges[i].RequestRate = m.RequestRate
				t.Edges[i].ErrorRate = m.ErrorRate
				t.Edges[i].LatencyP99ms = m.LatencyP99ms
				if m.RequestRate > 0 {
					if m.ErrorRate > 0.2 {
						t.Edges[i].Health = "down"
					} else if m.ErrorRate > 0.05 {
						t.Edges[i].Health = "degraded"
					} else {
						t.Edges[i].Health = "healthy"
					}
				}
				break
			}
		}
	}

	// Phase D: append Workload→Service edges from actual Istio traffic
	existingEdges := map[string]bool{}
	for _, e := range t.Edges {
		existingEdges[e.Source+"->"+e.Target] = true
	}

	for _, m := range metrics {
		if m.RequestRate == 0 || m.DestService == "" {
			continue
		}
		srcID, srcOK := wlByName[m.SourceNamespace+"/"+m.SourceWorkload]
		// Use DestServiceNamespace first; fall back to DestNamespace
		svcNs := m.DestServiceNamespace
		if svcNs == "" {
			svcNs = m.DestNamespace
		}
		dstSvcID, dstOK := svcByName[svcNs+"/"+m.DestService]
		if !srcOK || !dstOK {
			continue
		}
		ek := srcID + "->" + dstSvcID
		if existingEdges[ek] {
			continue
		}
		existingEdges[ek] = true

		health := "healthy"
		if m.ErrorRate > 0.2 {
			health = "down"
		} else if m.ErrorRate > 0.05 {
			health = "degraded"
		}

		t.Edges = append(t.Edges, NetworkEdge{
			Source:       srcID,
			Target:       dstSvcID,
			Kind:         "istio-flow",
			Health:       health,
			RequestRate:  m.RequestRate,
			ErrorRate:    m.ErrorRate,
			LatencyP99ms: m.LatencyP99ms,
		})
	}
}

// extractNsFromNodeID extracts the namespace segment from node IDs:
//   "workload/ns/kind/name" → "ns"
//   "service/ns/name"       → "ns"
//   "ingress/ns/name"       → "ns"
func extractNsFromNodeID(id string) string {
	parts := strings.SplitN(id, "/", 3)
	if len(parts) >= 2 {
		return parts[1]
	}
	return ""
}

// EnrichWithHubbleMetrics annotates edges with Cilium Hubble flow data (Phase F).
// Matching is namespace-pair based: srcNs→dstNs.
func (t *ClusterNetworkTopology) EnrichWithHubbleMetrics(metrics map[string]*HubbleEdgeMetrics) {
	if len(metrics) == 0 {
		return
	}
	for i, edge := range t.Edges {
		srcNs := extractNsFromNodeID(edge.Source)
		dstNs := extractNsFromNodeID(edge.Target)
		if srcNs == "" || dstNs == "" {
			continue
		}
		m, ok := metrics[srcNs+"→"+dstNs]
		if !ok {
			continue
		}
		t.Edges[i].HubbleFlowRate = m.FlowRate
		t.Edges[i].HubbleDropRate = m.DropRate
		t.Edges[i].HubbleDropReason = m.TopDropReason
		// Override health based on drop rate (only when no Istio health already set)
		if edge.RequestRate == 0 && m.DropRate > 0 {
			if m.DropRate > 0.5 {
				t.Edges[i].Health = "down"
			} else if m.DropRate > 0.1 {
				t.Edges[i].Health = "degraded"
			}
		}
	}
}
