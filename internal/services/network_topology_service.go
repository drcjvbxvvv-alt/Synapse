package services

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// ---- DTOs ----

type NetworkNode struct {
	ID           string            `json:"id"`
	Kind         string            `json:"kind"`                   // "Workload" | "Service" | "Ingress"
	Name         string            `json:"name"`
	Namespace    string            `json:"namespace"`
	WorkloadKind string            `json:"workloadKind,omitempty"` // Deployment|StatefulSet|DaemonSet|Job|Pod
	Labels       map[string]string `json:"labels,omitempty"`
	ReadyCount   int               `json:"readyCount"`
	TotalCount   int               `json:"totalCount"`
	ClusterIP    string            `json:"clusterIP,omitempty"`
	ServiceType  string            `json:"serviceType,omitempty"`
	IngressClass string            `json:"ingressClass,omitempty"` // Phase C: nginx | traefik | istio …
}

type NetworkEdge struct {
	Source       string  `json:"source"`
	Target       string  `json:"target"`
	Kind         string  `json:"kind,omitempty"`   // "" (static) | "ingress"
	Health       string  `json:"health"`           // "healthy"|"degraded"|"down"|"unknown"
	Ports        string  `json:"ports,omitempty"`
	// Phase B: Istio enrichment (omitted when not available)
	RequestRate  float64 `json:"requestRate,omitempty"`  // req/s
	ErrorRate    float64 `json:"errorRate,omitempty"`    // 0.0-1.0
	LatencyP99ms float64 `json:"latencyP99ms,omitempty"` // ms
}

// EnrichWithIstioMetrics enriches edges in-place using Istio flow metrics.
// Matching is done by destination workload name + namespace.
func (t *ClusterNetworkTopology) EnrichWithIstioMetrics(metrics map[string]*IstioEdgeMetrics) {
	if len(metrics) == 0 {
		return
	}
	// Build workload name map: "ns/name" → node ID
	wlByName := map[string]string{}
	for _, n := range t.Nodes {
		if n.Kind == "Workload" {
			wlByName[n.Namespace+"/"+n.Name] = n.ID
		}
	}

	for i, edge := range t.Edges {
		// edge.Target is "workload/ns/kind/name"
		parts := splitWorkloadID(edge.Target)
		if parts == nil {
			continue
		}
		destKey := parts[0] + "/" + parts[1] // "ns/name"
		for _, m := range metrics {
			if m.DestNamespace+"/"+m.DestWorkload == destKey {
				t.Edges[i].RequestRate = m.RequestRate
				t.Edges[i].ErrorRate = m.ErrorRate
				t.Edges[i].LatencyP99ms = m.LatencyP99ms
				// Override health with Istio error rate if more precise
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
}

// splitWorkloadID extracts [namespace, name] from "workload/ns/kind/name"
func splitWorkloadID(id string) []string {
	// format: workload/{ns}/{kind}/{name}
	parts := strings.SplitN(id, "/", 4)
	if len(parts) != 4 || parts[0] != "workload" {
		return nil
	}
	return []string{parts[1], parts[3]} // [ns, name]
}

type ClusterNetworkTopology struct {
	Nodes []NetworkNode `json:"nodes"`
	Edges []NetworkEdge `json:"edges"`
}

// workloadRef identifies a workload node
type workloadRef struct {
	Namespace string
	Kind      string
	Name      string
}

func networkWorkloadNodeID(ns, kind, name string) string {
	return fmt.Sprintf("workload/%s/%s/%s", ns, kind, name)
}

func networkServiceNodeID(ns, name string) string {
	return fmt.Sprintf("service/%s/%s", ns, name)
}

func networkIngressNodeID(ns, name string) string {
	return fmt.Sprintf("ingress/%s/%s", ns, name)
}

// labelsContain returns true if all selector key-values exist in labels.
// Empty selectors are treated as non-matching (headless/no-selector services).
func labelsContain(selector, labels map[string]string) bool {
	if len(selector) == 0 {
		return false
	}
	for k, v := range selector {
		if labels[k] != v {
			return false
		}
	}
	return true
}

func isPodReadyForTopo(pod corev1.Pod) bool {
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodReady {
			return cond.Status == corev1.ConditionTrue
		}
	}
	return false
}

func networkEdgeHealth(ep *epReadiness) string {
	if ep == nil || ep.Total == 0 {
		return "unknown"
	}
	if ep.Ready == 0 {
		return "down"
	}
	if ep.Ready < ep.Total {
		return "degraded"
	}
	return "healthy"
}

type epReadiness struct {
	Ready int
	Total int
}

// rsOwnerRef records the top-level owner of a ReplicaSet
type rsOwnerRef struct {
	kind string
	name string
}

// workloadState accumulates pod count and labels for a rolled-up workload
type workloadState struct {
	Kind       string
	Name       string
	Namespace  string
	PodLabels  map[string]string
	ReadyCount int
	TotalCount int
}

// GetClusterNetworkTopology builds static topology from K8s resources.
// namespaces: filter to specific namespaces; empty = all namespaces.
func GetClusterNetworkTopology(
	ctx context.Context,
	clientset kubernetes.Interface,
	namespaces []string,
) (*ClusterNetworkTopology, error) {
	// 1. Determine namespace list
	if len(namespaces) == 0 {
		nsList, err := clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("list namespaces: %w", err)
		}
		for _, ns := range nsList.Items {
			namespaces = append(namespaces, ns.Name)
		}
	}

	workloads := map[workloadRef]*workloadState{}
	standalonePods := []corev1.Pod{}

	// 2. Pre-list ReplicaSets per namespace (avoids per-pod API calls)
	rsOwners := map[string]map[string]rsOwnerRef{} // ns -> rsName -> ownerRef

	for _, ns := range namespaces {
		rsList, err := clientset.AppsV1().ReplicaSets(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			continue
		}
		rsOwners[ns] = make(map[string]rsOwnerRef, len(rsList.Items))
		for _, rs := range rsList.Items {
			for _, ref := range rs.OwnerReferences {
				if ref.Kind == "Deployment" {
					rsOwners[ns][rs.Name] = rsOwnerRef{kind: "Deployment", name: ref.Name}
					break
				}
			}
		}
	}

	// 3. List pods and roll up into workloads
	for _, ns := range namespaces {
		pods, err := clientset.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			continue
		}
		for _, pod := range pods.Items {
			kind, name := resolveWorkloadOwner(pod.OwnerReferences, rsOwners[ns])
			if kind == "" {
				standalonePods = append(standalonePods, pod)
				continue
			}
			key := workloadRef{Namespace: ns, Kind: kind, Name: name}
			ws, ok := workloads[key]
			if !ok {
				ws = &workloadState{Kind: kind, Name: name, Namespace: ns, PodLabels: pod.Labels}
				workloads[key] = ws
			}
			ws.TotalCount++
			if isPodReadyForTopo(pod) {
				ws.ReadyCount++
			}
		}
	}

	// 4. List Services
	type svcInfo struct {
		Name      string
		Namespace string
		ClusterIP string
		Type      string
		Selector  map[string]string
		Ports     []string
	}
	var services []*svcInfo
	for _, ns := range namespaces {
		svcList, err := clientset.CoreV1().Services(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			continue
		}
		for _, svc := range svcList.Items {
			if svc.Spec.ClusterIP == "None" && len(svc.Spec.Selector) == 0 {
				continue // skip headless no-selector services
			}
			var ports []string
			for _, p := range svc.Spec.Ports {
				ports = append(ports, fmt.Sprintf("%d/%s", p.Port, p.Protocol))
			}
			services = append(services, &svcInfo{
				Name:      svc.Name,
				Namespace: svc.Namespace,
				ClusterIP: svc.Spec.ClusterIP,
				Type:      string(svc.Spec.Type),
				Selector:  svc.Spec.Selector,
				Ports:     ports,
			})
		}
	}

	// 5. Endpoint readiness (for edge health)
	epHealth := map[string]*epReadiness{} // key = "ns/name"
	for _, ns := range namespaces {
		epList, err := clientset.CoreV1().Endpoints(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			continue
		}
		for _, ep := range epList.Items {
			key := ep.Namespace + "/" + ep.Name
			h := &epReadiness{}
			for _, sub := range ep.Subsets {
				h.Ready += len(sub.Addresses)
				h.Total += len(sub.Addresses) + len(sub.NotReadyAddresses)
			}
			epHealth[key] = h
		}
	}

	// 6. Build nodes
	var nodes []NetworkNode
	nodeSet := map[string]bool{}

	for key, ws := range workloads {
		id := networkWorkloadNodeID(key.Namespace, key.Kind, key.Name)
		if nodeSet[id] {
			continue
		}
		nodeSet[id] = true
		nodes = append(nodes, NetworkNode{
			ID:           id,
			Kind:         "Workload",
			Name:         ws.Name,
			Namespace:    ws.Namespace,
			WorkloadKind: ws.Kind,
			Labels:       ws.PodLabels,
			ReadyCount:   ws.ReadyCount,
			TotalCount:   ws.TotalCount,
		})
	}
	for _, pod := range standalonePods {
		id := networkWorkloadNodeID(pod.Namespace, "Pod", pod.Name)
		if nodeSet[id] {
			continue
		}
		nodeSet[id] = true
		ready := 0
		if isPodReadyForTopo(pod) {
			ready = 1
		}
		nodes = append(nodes, NetworkNode{
			ID:           id,
			Kind:         "Workload",
			Name:         pod.Name,
			Namespace:    pod.Namespace,
			WorkloadKind: "Pod",
			Labels:       pod.Labels,
			ReadyCount:   ready,
			TotalCount:   1,
		})
	}
	for _, svc := range services {
		id := networkServiceNodeID(svc.Namespace, svc.Name)
		if nodeSet[id] {
			continue
		}
		nodeSet[id] = true
		nodes = append(nodes, NetworkNode{
			ID:          id,
			Kind:        "Service",
			Name:        svc.Name,
			Namespace:   svc.Namespace,
			ClusterIP:   svc.ClusterIP,
			ServiceType: svc.Type,
		})
	}

	// 7. Build edges: Service → Workload via label selector match
	var edges []NetworkEdge
	edgeSet := map[string]bool{}

	for _, svc := range services {
		if len(svc.Selector) == 0 {
			continue
		}
		svcID := networkServiceNodeID(svc.Namespace, svc.Name)
		health := networkEdgeHealth(epHealth[svc.Namespace+"/"+svc.Name])
		ports := strings.Join(svc.Ports, ", ")

		for key, ws := range workloads {
			if key.Namespace != svc.Namespace {
				continue
			}
			if !labelsContain(svc.Selector, ws.PodLabels) {
				continue
			}
			wlID := networkWorkloadNodeID(key.Namespace, key.Kind, key.Name)
			ek := svcID + "->" + wlID
			if edgeSet[ek] {
				continue
			}
			edgeSet[ek] = true
			edges = append(edges, NetworkEdge{Source: svcID, Target: wlID, Health: health, Ports: ports})
		}
		for _, pod := range standalonePods {
			if pod.Namespace != svc.Namespace {
				continue
			}
			if !labelsContain(svc.Selector, pod.Labels) {
				continue
			}
			podID := networkWorkloadNodeID(pod.Namespace, "Pod", pod.Name)
			ek := svcID + "->" + podID
			if edgeSet[ek] {
				continue
			}
			edgeSet[ek] = true
			edges = append(edges, NetworkEdge{Source: svcID, Target: podID, Health: health, Ports: ports})
		}
	}

	// 8. List Ingresses and build Ingress → Service edges (Phase C)
	for _, ns := range namespaces {
		ingList, err := clientset.NetworkingV1().Ingresses(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			continue
		}
		for _, ing := range ingList.Items {
			ingID := networkIngressNodeID(ing.Namespace, ing.Name)
			if !nodeSet[ingID] {
				nodeSet[ingID] = true
				ingressClass := ""
				if ing.Spec.IngressClassName != nil {
					ingressClass = *ing.Spec.IngressClassName
				}
				nodes = append(nodes, NetworkNode{
					ID:           ingID,
					Kind:         "Ingress",
					Name:         ing.Name,
					Namespace:    ing.Namespace,
					IngressClass: ingressClass,
				})
			}

			addIngressEdge := func(svcName, port string) {
				svcID := networkServiceNodeID(ing.Namespace, svcName)
				if !nodeSet[svcID] {
					return // skip if target service not in selected namespaces
				}
				ek := ingID + "->" + svcID
				if edgeSet[ek] {
					return
				}
				edgeSet[ek] = true
				edges = append(edges, NetworkEdge{
					Source: ingID,
					Target: svcID,
					Kind:   "ingress",
					Health: "healthy",
					Ports:  port,
				})
			}

			// Rules-based backends
			for _, rule := range ing.Spec.Rules {
				if rule.HTTP == nil {
					continue
				}
				for _, path := range rule.HTTP.Paths {
					if path.Backend.Service == nil {
						continue
					}
					port := ""
					if path.Backend.Service.Port.Number > 0 {
						port = fmt.Sprintf("%d", path.Backend.Service.Port.Number)
					} else if path.Backend.Service.Port.Name != "" {
						port = path.Backend.Service.Port.Name
					}
					addIngressEdge(path.Backend.Service.Name, port)
				}
			}

			// Default backend
			if ing.Spec.DefaultBackend != nil && ing.Spec.DefaultBackend.Service != nil {
				addIngressEdge(ing.Spec.DefaultBackend.Service.Name, "")
			}
		}
	}

	return &ClusterNetworkTopology{Nodes: nodes, Edges: edges}, nil
}

// resolveWorkloadOwner returns the top-level workload kind+name for a pod's owner references.
func resolveWorkloadOwner(refs []metav1.OwnerReference, rsMap map[string]rsOwnerRef) (string, string) {
	for _, ref := range refs {
		switch ref.Kind {
		case "ReplicaSet":
			if rsMap != nil {
				if owner, ok := rsMap[ref.Name]; ok {
					return owner.kind, owner.name
				}
			}
			return "ReplicaSet", ref.Name
		case "Deployment":
			return "Deployment", ref.Name
		case "StatefulSet":
			return "StatefulSet", ref.Name
		case "DaemonSet":
			return "DaemonSet", ref.Name
		case "Job":
			return "Job", ref.Name
		}
	}
	return "", ""
}
