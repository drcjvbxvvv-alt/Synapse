package services

import (
	"context"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// InferNetworkPolicies annotates static Service→Workload edges with NetworkPolicy status (Phase E).
// status values:
//   - "policy-allow"      : a NetworkPolicy applies AND has an open ingress rule (no From restriction)
//   - "policy-deny"       : a NetworkPolicy applies with empty ingress list (deny all)
//   - "policy-restricted" : a NetworkPolicy applies with source-specific From rules (uncertain)
func (t *ClusterNetworkTopology) InferNetworkPolicies(
	ctx context.Context,
	clientset kubernetes.Interface,
	namespaces []string,
) error {
	// Build node label index: nodeID → pod labels (Workload nodes only)
	nodeLabels := map[string]map[string]string{}
	for _, n := range t.Nodes {
		if n.Kind == "Workload" && len(n.Labels) > 0 {
			nodeLabels[n.ID] = n.Labels
		}
	}

	type netpolInfo struct {
		Name        string
		Namespace   string
		Selector    map[string]string // podSelector.matchLabels (empty = all pods in ns)
		AllSelector bool              // true when podSelector is completely empty
		HasIngress  bool              // spec.ingress is non-empty
		OpenIngress bool              // at least one rule with no From restriction
	}

	var policies []netpolInfo
	for _, ns := range namespaces {
		npList, err := clientset.NetworkingV1().NetworkPolicies(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			continue
		}
		for _, np := range npList.Items {
			pi := netpolInfo{
				Name:        np.Name,
				Namespace:   np.Namespace,
				Selector:    np.Spec.PodSelector.MatchLabels,
				AllSelector: len(np.Spec.PodSelector.MatchLabels) == 0 && len(np.Spec.PodSelector.MatchExpressions) == 0,
				HasIngress:  len(np.Spec.Ingress) > 0,
			}
			for _, rule := range np.Spec.Ingress {
				if len(rule.From) == 0 { // empty From = allow from anywhere
					pi.OpenIngress = true
					break
				}
			}
			policies = append(policies, pi)
		}
	}

	if len(policies) == 0 {
		return nil
	}

	for i, edge := range t.Edges {
		// Only annotate static Service→Workload edges
		if edge.Kind != "" {
			continue
		}
		targetLabels, ok := nodeLabels[edge.Target]
		if !ok {
			continue
		}

		// Determine target namespace from node ID: "workload/ns/kind/name"
		parts := strings.SplitN(edge.Target, "/", 4)
		if len(parts) < 2 {
			continue
		}
		targetNs := parts[1]

		// Find matching NetworkPolicies
		var matched []netpolInfo
		for _, p := range policies {
			if p.Namespace != targetNs {
				continue
			}
			if p.AllSelector || netpolMatchesLabels(p.Selector, targetLabels) {
				matched = append(matched, p)
			}
		}
		if len(matched) == 0 {
			continue
		}

		// Determine combined status across all matching policies
		anyOpen := false
		allDenyAll := true
		firstName := matched[0].Name
		for _, p := range matched {
			if p.OpenIngress {
				anyOpen = true
			}
			if p.HasIngress {
				allDenyAll = false
			}
		}

		switch {
		case anyOpen:
			t.Edges[i].PolicyStatus = "policy-allow"
			t.Edges[i].PolicyName = firstName
		case allDenyAll:
			t.Edges[i].PolicyStatus = "policy-deny"
			t.Edges[i].PolicyName = firstName
		default:
			// Has ingress rules but all have From restrictions — source-specific, uncertain
			t.Edges[i].PolicyStatus = "policy-restricted"
			t.Edges[i].PolicyName = firstName
		}
	}

	return nil
}

// netpolMatchesLabels checks if a NetworkPolicy podSelector matches pod labels.
// Unlike labelsContain, an empty selector matches all pods (NetworkPolicy semantics).
func netpolMatchesLabels(selector, podLabels map[string]string) bool {
	for k, v := range selector {
		if podLabels[k] != v {
			return false
		}
	}
	return true
}
