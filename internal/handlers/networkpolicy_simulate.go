package handlers

import (
	"context"

	"github.com/shaia/Synapse/internal/response"

	"github.com/gin-gonic/gin"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SimulateRequest 策略模擬請求
type SimulateRequest struct {
	Namespace     string            `json:"namespace" binding:"required"`
	FromPodLabels map[string]string `json:"fromPodLabels"`
	ToPodLabels   map[string]string `json:"toPodLabels"`
	Port          int               `json:"port"`
	Protocol      string            `json:"protocol"`
}

// SimulateResult 策略模擬結果
type SimulateResult struct {
	Allowed         bool     `json:"allowed"`
	Reason          string   `json:"reason"`
	MatchedPolicies []string `json:"matchedPolicies"`
}

// SimulateNetworkPolicy POST /clusters/:clusterID/networkpolicies/simulate
func (h *NetworkPolicyHandler) SimulateNetworkPolicy(c *gin.Context) {
	clientset, _, ok := h.getClient(c)
	if !ok {
		return
	}

	var req SimulateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "無效的請求參數: "+err.Error())
		return
	}
	if req.Protocol == "" {
		req.Protocol = "TCP"
	}

	ctx := context.Background()
	nps, err := clientset.NetworkingV1().NetworkPolicies(req.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		response.InternalError(c, "取得 NetworkPolicy 失敗: "+err.Error())
		return
	}

	result := simulateTraffic(nps.Items, req)
	response.OK(c, result)
}

// labelsMatch checks if selector matches against podLabels (empty selector matches all)
func labelsMatch(selector map[string]string, podLabels map[string]string) bool {
	for k, v := range selector {
		if podLabels[k] != v {
			return false
		}
	}
	return true
}

// portMatches checks if port matches a NetworkPolicy port spec (0 port means any)
func portMatches(npPort networkingv1.NetworkPolicyPort, reqPort int, reqProtocol string) bool {
	if npPort.Protocol != nil && string(*npPort.Protocol) != reqProtocol {
		return false
	}
	if npPort.Port == nil {
		return true // no port restriction
	}
	portVal := npPort.Port.IntValue()
	if portVal == 0 {
		return true
	}
	if reqPort == 0 {
		return true
	}
	if portVal == reqPort {
		return true
	}
	if npPort.EndPort != nil && reqPort >= portVal && reqPort <= int(*npPort.EndPort) {
		return true
	}
	return false
}

// peerMatches checks if a NetworkPolicyPeer matches the given podLabels
func peerMatches(peer networkingv1.NetworkPolicyPeer, podLabels map[string]string) bool {
	if peer.IPBlock != nil {
		return false // IP block peers not matched against pod labels
	}
	if peer.PodSelector != nil {
		return labelsMatch(peer.PodSelector.MatchLabels, podLabels)
	}
	if peer.NamespaceSelector != nil {
		// namespace selector only - matches any pod in matching namespace; treat as allowing all pods
		return true
	}
	return true // empty peer = allow all
}

// simulateTraffic runs the NP simulation engine
func simulateTraffic(policies []networkingv1.NetworkPolicy, req SimulateRequest) SimulateResult {
	// find policies that select the target pod
	var matchedPolicies []string
	var ingressControlling []networkingv1.NetworkPolicy

	for _, np := range policies {
		if !labelsMatch(np.Spec.PodSelector.MatchLabels, req.ToPodLabels) {
			continue
		}
		// check if this policy has Ingress type
		for _, pt := range np.Spec.PolicyTypes {
			if pt == networkingv1.PolicyTypeIngress {
				ingressControlling = append(ingressControlling, np)
				matchedPolicies = append(matchedPolicies, np.Name)
				break
			}
		}
		// if no PolicyTypes specified but has ingress rules, it controls ingress
		if len(np.Spec.PolicyTypes) == 0 && len(np.Spec.Ingress) > 0 {
			ingressControlling = append(ingressControlling, np)
			matchedPolicies = append(matchedPolicies, np.Name)
		}
	}

	// No NP controls ingress to target → default allow
	if len(ingressControlling) == 0 {
		return SimulateResult{
			Allowed:         true,
			Reason:          "目標 Pod 無 Ingress NetworkPolicy 控管，預設允許",
			MatchedPolicies: []string{},
		}
	}

	// Check each controlling policy's ingress rules
	for _, np := range ingressControlling {
		if len(np.Spec.Ingress) == 0 {
			// policy with Ingress type but no rules = deny all ingress
			continue
		}
		for _, rule := range np.Spec.Ingress {
			// Check ports
			portOK := len(rule.Ports) == 0
			for _, p := range rule.Ports {
				if portMatches(p, req.Port, req.Protocol) {
					portOK = true
					break
				}
			}
			if !portOK {
				continue
			}
			// Check from peers
			fromOK := len(rule.From) == 0
			for _, peer := range rule.From {
				if peerMatches(peer, req.FromPodLabels) {
					fromOK = true
					break
				}
			}
			if fromOK {
				return SimulateResult{
					Allowed:         true,
					Reason:          "規則匹配: NetworkPolicy/" + np.Name,
					MatchedPolicies: matchedPolicies,
				}
			}
		}
	}

	return SimulateResult{
		Allowed:         false,
		Reason:          "存在 Ingress NetworkPolicy 但無規則匹配，預設拒絕",
		MatchedPolicies: matchedPolicies,
	}
}
