package handlers

import (
	"time"

	networkingv1 "k8s.io/api/networking/v1"
)

// NetworkPolicyInfo NetworkPolicy 摘要資訊
type NetworkPolicyInfo struct {
	Name         string            `json:"name"`
	Namespace    string            `json:"namespace"`
	PodSelector  map[string]string `json:"podSelector"`
	PolicyTypes  []string          `json:"policyTypes"`
	IngressRules int               `json:"ingressRules"`
	EgressRules  int               `json:"egressRules"`
	CreatedAt    time.Time         `json:"createdAt"`
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
}

// NetworkPolicyDetail NetworkPolicy 詳細資訊
type NetworkPolicyDetail struct {
	NetworkPolicyInfo
	Ingress []NetworkPolicyIngressRule `json:"ingress,omitempty"`
	Egress  []NetworkPolicyEgressRule  `json:"egress,omitempty"`
}

// NetworkPolicyIngressRule Ingress 規則
type NetworkPolicyIngressRule struct {
	Ports []NetworkPolicyPort `json:"ports,omitempty"`
	From  []NetworkPolicyPeer `json:"from,omitempty"`
}

// NetworkPolicyEgressRule Egress 規則
type NetworkPolicyEgressRule struct {
	Ports []NetworkPolicyPort `json:"ports,omitempty"`
	To    []NetworkPolicyPeer `json:"to,omitempty"`
}

// NetworkPolicyPort 連線連接埠規則
type NetworkPolicyPort struct {
	Protocol string `json:"protocol,omitempty"`
	Port     string `json:"port,omitempty"`
	EndPort  *int32 `json:"endPort,omitempty"`
}

// NetworkPolicyPeer 對等方規則
type NetworkPolicyPeer struct {
	PodSelector       *LabelSelectorInfo `json:"podSelector,omitempty"`
	NamespaceSelector *LabelSelectorInfo `json:"namespaceSelector,omitempty"`
	IPBlock           *IPBlockInfo       `json:"ipBlock,omitempty"`
}

// LabelSelectorInfo 標籤選擇器資訊
type LabelSelectorInfo struct {
	MatchLabels map[string]string `json:"matchLabels,omitempty"`
}

// IPBlockInfo IP 區塊資訊
type IPBlockInfo struct {
	CIDR   string   `json:"cidr"`
	Except []string `json:"except,omitempty"`
}

// convertPeer 轉換 NetworkPolicyPeer
func convertPeer(peer networkingv1.NetworkPolicyPeer) NetworkPolicyPeer {
	result := NetworkPolicyPeer{}
	if peer.PodSelector != nil {
		result.PodSelector = &LabelSelectorInfo{
			MatchLabels: peer.PodSelector.MatchLabels,
		}
	}
	if peer.NamespaceSelector != nil {
		result.NamespaceSelector = &LabelSelectorInfo{
			MatchLabels: peer.NamespaceSelector.MatchLabels,
		}
	}
	if peer.IPBlock != nil {
		result.IPBlock = &IPBlockInfo{
			CIDR:   peer.IPBlock.CIDR,
			Except: peer.IPBlock.Except,
		}
	}
	return result
}
