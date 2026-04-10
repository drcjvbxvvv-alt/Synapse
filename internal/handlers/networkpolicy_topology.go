package handlers

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/pkg/logger"

	"github.com/gin-gonic/gin"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TopologyNode 拓撲節點
type TopologyNode struct {
	ID          string            `json:"id"`
	Type        string            `json:"type"` // "podgroup" | "namespace" | "ipblock" | "external"
	Label       string            `json:"label"`
	Namespace   string            `json:"namespace,omitempty"`
	Selector    map[string]string `json:"selector,omitempty"`
	PolicyCount int               `json:"policyCount,omitempty"`
}

// TopologyEdge 拓撲邊
type TopologyEdge struct {
	ID        string `json:"id"`
	Source    string `json:"source"`
	Target    string `json:"target"`
	Label     string `json:"label,omitempty"`
	Direction string `json:"direction"` // "ingress" | "egress"
	Policy    string `json:"policy"`
	Namespace string `json:"namespace"`
}

// TopologyResponse 拓撲回應
type TopologyResponse struct {
	Nodes []TopologyNode `json:"nodes"`
	Edges []TopologyEdge `json:"edges"`
}

// GetTopology 取得 NetworkPolicy 拓撲圖資料
func (h *NetworkPolicyHandler) GetTopology(c *gin.Context) {
	clientset, clusterID, ok := h.getClient(c)
	if !ok {
		return
	}

	namespace := c.DefaultQuery("namespace", "")
	ns := namespace
	if ns == "_all_" {
		ns = ""
	}

	policies, err := clientset.NetworkingV1().NetworkPolicies(ns).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		logger.Error("取得 NetworkPolicy 列表失敗", "error", err, "clusterId", clusterID)
		response.InternalError(c, fmt.Sprintf("取得 NetworkPolicy 列表失敗: %v", err))
		return
	}

	nodeMap := make(map[string]*TopologyNode)
	var edges []TopologyEdge
	edgeCount := 0

	addNode := func(id, ntype, label, nodeNS string, sel map[string]string) {
		if _, ok := nodeMap[id]; !ok {
			nodeMap[id] = &TopologyNode{ID: id, Type: ntype, Label: label, Namespace: nodeNS, Selector: sel}
		}
		nodeMap[id].PolicyCount++
	}

	selectorStr := func(labels map[string]string) string {
		if len(labels) == 0 {
			return "(all pods)"
		}
		parts := make([]string, 0, len(labels))
		for k, v := range labels {
			parts = append(parts, k+"="+v)
		}
		sort.Strings(parts)
		return strings.Join(parts, ",")
	}

	portLabel := func(ports []networkingv1.NetworkPolicyPort) string {
		if len(ports) == 0 {
			return "all ports"
		}
		ps := make([]string, 0, len(ports))
		for _, p := range ports {
			proto := "TCP"
			if p.Protocol != nil {
				proto = string(*p.Protocol)
			}
			port := "*"
			if p.Port != nil {
				port = p.Port.String()
			}
			ps = append(ps, proto+":"+port)
		}
		return strings.Join(ps, ", ")
	}

	for i := range policies.Items {
		np := &policies.Items[i]
		targetSel := np.Spec.PodSelector.MatchLabels
		targetID := "pod:" + np.Namespace + ":" + selectorStr(targetSel)
		targetLabel := selectorStr(targetSel)
		if len(targetSel) == 0 {
			targetLabel = "All Pods"
		}
		addNode(targetID, "podgroup", targetLabel, np.Namespace, targetSel)

		// Ingress rules → peers → target
		for _, rule := range np.Spec.Ingress {
			pLabel := portLabel(rule.Ports)
			if len(rule.From) == 0 {
				srcID := "external:any"
				addNode(srcID, "external", "Any", "", nil)
				edgeCount++
				edges = append(edges, TopologyEdge{
					ID: fmt.Sprintf("e%d", edgeCount), Source: srcID, Target: targetID,
					Label: pLabel, Direction: "ingress", Policy: np.Name, Namespace: np.Namespace,
				})
			}
			for _, peer := range rule.From {
				var srcID, srcLabel, srcNS string
				var srcType string
				var srcSel map[string]string
				switch {
				case peer.IPBlock != nil:
					srcID = "ip:" + peer.IPBlock.CIDR
					srcLabel = peer.IPBlock.CIDR
					srcType = "ipblock"
				case peer.NamespaceSelector != nil:
					sel := peer.NamespaceSelector.MatchLabels
					srcID = "ns:" + selectorStr(sel)
					srcLabel = selectorStr(sel)
					if len(sel) == 0 {
						srcLabel = "All Namespaces"
					}
					srcType = "namespace"
					srcSel = sel
				default:
					sel := map[string]string{}
					if peer.PodSelector != nil {
						sel = peer.PodSelector.MatchLabels
					}
					srcNS = np.Namespace
					srcID = "pod:" + srcNS + ":" + selectorStr(sel)
					srcLabel = selectorStr(sel)
					if len(sel) == 0 {
						srcLabel = "All Pods"
					}
					srcType = "podgroup"
					srcSel = sel
				}
				addNode(srcID, srcType, srcLabel, srcNS, srcSel)
				edgeCount++
				edges = append(edges, TopologyEdge{
					ID: fmt.Sprintf("e%d", edgeCount), Source: srcID, Target: targetID,
					Label: pLabel, Direction: "ingress", Policy: np.Name, Namespace: np.Namespace,
				})
			}
		}

		// Egress rules → target → peers
		for _, rule := range np.Spec.Egress {
			pLabel := portLabel(rule.Ports)
			if len(rule.To) == 0 {
				dstID := "external:any"
				addNode(dstID, "external", "Any", "", nil)
				edgeCount++
				edges = append(edges, TopologyEdge{
					ID: fmt.Sprintf("e%d", edgeCount), Source: targetID, Target: dstID,
					Label: pLabel, Direction: "egress", Policy: np.Name, Namespace: np.Namespace,
				})
			}
			for _, peer := range rule.To {
				var dstID, dstLabel, dstNS string
				var dstType string
				var dstSel map[string]string
				switch {
				case peer.IPBlock != nil:
					dstID = "ip:" + peer.IPBlock.CIDR
					dstLabel = peer.IPBlock.CIDR
					dstType = "ipblock"
				case peer.NamespaceSelector != nil:
					sel := peer.NamespaceSelector.MatchLabels
					dstID = "ns:" + selectorStr(sel)
					dstLabel = selectorStr(sel)
					if len(sel) == 0 {
						dstLabel = "All Namespaces"
					}
					dstType = "namespace"
					dstSel = sel
				default:
					sel := map[string]string{}
					if peer.PodSelector != nil {
						sel = peer.PodSelector.MatchLabels
					}
					dstNS = np.Namespace
					dstID = "pod:" + dstNS + ":" + selectorStr(sel)
					dstLabel = selectorStr(sel)
					if len(sel) == 0 {
						dstLabel = "All Pods"
					}
					dstType = "podgroup"
					dstSel = sel
				}
				addNode(dstID, dstType, dstLabel, dstNS, dstSel)
				edgeCount++
				edges = append(edges, TopologyEdge{
					ID: fmt.Sprintf("e%d", edgeCount), Source: targetID, Target: dstID,
					Label: pLabel, Direction: "egress", Policy: np.Name, Namespace: np.Namespace,
				})
			}
		}
	}

	nodes := make([]TopologyNode, 0, len(nodeMap))
	for _, n := range nodeMap {
		nodes = append(nodes, *n)
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })

	response.OK(c, TopologyResponse{Nodes: nodes, Edges: edges})
}
