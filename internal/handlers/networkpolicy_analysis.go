package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/pkg/logger"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sintstr "k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/yaml"
)

// ---- Conflicts ----

// ConflictItem 衝突專案
type ConflictItem struct {
	PolicyA   string            `json:"policyA"`
	PolicyB   string            `json:"policyB"`
	Namespace string            `json:"namespace"`
	Reason    string            `json:"reason"`
	SelectorA map[string]string `json:"selectorA"`
	SelectorB map[string]string `json:"selectorB"`
}

// GetConflicts 偵測同命名空間中選擇器重疊的 NetworkPolicy 衝突
func (h *NetworkPolicyHandler) GetConflicts(c *gin.Context) {
	clientset, clusterID, ok := h.getClient(c)
	if !ok {
		return
	}

	namespace := c.DefaultQuery("namespace", "")
	ns := namespace
	if ns == "_all_" {
		ns = ""
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	policies, err := clientset.NetworkingV1().NetworkPolicies(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		logger.Error("取得 NetworkPolicy 列表失敗", "error", err, "clusterId", clusterID)
		response.InternalError(c, fmt.Sprintf("取得 NetworkPolicy 列表失敗: %v", err))
		return
	}

	// 依命名空間分組
	byNS := make(map[string][]networkingv1.NetworkPolicy)
	for _, p := range policies.Items {
		byNS[p.Namespace] = append(byNS[p.Namespace], p)
	}

	selectorsOverlap := func(a, b map[string]string) bool {
		// 空選擇器 = 選取所有 Pod，必然與任何選擇器重疊
		if len(a) == 0 || len(b) == 0 {
			return true
		}
		// 有共同的 key=value 時視為重疊
		for k, v := range a {
			if bv, ok := b[k]; ok && bv == v {
				return true
			}
		}
		return false
	}

	var conflicts []ConflictItem
	for nsName, pols := range byNS {
		for i := 0; i < len(pols); i++ {
			for j := i + 1; j < len(pols); j++ {
				a, b := pols[i], pols[j]
				selA := a.Spec.PodSelector.MatchLabels
				selB := b.Spec.PodSelector.MatchLabels
				if !selectorsOverlap(selA, selB) {
					continue
				}
				reason := "選擇器重疊：兩個 Policy 作用於相同 Pod，可能產生非預期的疊加規則"
				if len(selA) == 0 && len(selB) == 0 {
					reason = "兩個 Policy 均使用空選擇器（作用於命名空間所有 Pod），規則完全重疊"
				} else if len(selA) == 0 || len(selB) == 0 {
					reason = "其中一個 Policy 使用空選擇器（作用於所有 Pod），會覆蓋另一個 Policy 的目標"
				}
				conflicts = append(conflicts, ConflictItem{
					PolicyA:   a.Name,
					PolicyB:   b.Name,
					Namespace: nsName,
					Reason:    reason,
					SelectorA: selA,
					SelectorB: selB,
				})
			}
		}
	}

	if conflicts == nil {
		conflicts = []ConflictItem{}
	}
	response.OK(c, gin.H{"conflicts": conflicts, "total": len(conflicts)})
}

// ---- Wizard ----

// WizardIngressRule 精靈 Ingress 規則
type WizardIngressRule struct {
	Ports    []WizardPort      `json:"ports,omitempty"`
	FromType string            `json:"fromType"` // "pod" | "namespace" | "ipblock" | "all"
	Selector map[string]string `json:"selector,omitempty"`
	CIDR     string            `json:"cidr,omitempty"`
}

// WizardEgressRule 精靈 Egress 規則
type WizardEgressRule struct {
	Ports    []WizardPort      `json:"ports,omitempty"`
	ToType   string            `json:"toType"` // "pod" | "namespace" | "ipblock" | "all"
	Selector map[string]string `json:"selector,omitempty"`
	CIDR     string            `json:"cidr,omitempty"`
}

// WizardPort 精靈連線連接埠
type WizardPort struct {
	Protocol string `json:"protocol"`
	Port     string `json:"port"`
}

// WizardValidateRequest 精靈驗證請求
type WizardValidateRequest struct {
	Step        int                 `json:"step"`
	Namespace   string              `json:"namespace"`
	Name        string              `json:"name"`
	Selector    map[string]string   `json:"selector"`
	PolicyTypes []string            `json:"policyTypes"`
	Ingress     []WizardIngressRule `json:"ingress,omitempty"`
	Egress      []WizardEgressRule  `json:"egress,omitempty"`
}

// WizardValidateResponse 精靈驗證回應
type WizardValidateResponse struct {
	Valid   bool   `json:"valid"`
	Message string `json:"message,omitempty"`
	YAML    string `json:"yaml,omitempty"`
}

// WizardValidate 精靈步驟驗證，Step 3 時產生 YAML 預覽
func (h *NetworkPolicyHandler) WizardValidate(c *gin.Context) {
	var req WizardValidateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "參數錯誤: "+err.Error())
		return
	}

	switch req.Step {
	case 1:
		if req.Namespace == "" {
			response.OK(c, WizardValidateResponse{Valid: false, Message: "請選擇命名空間"})
			return
		}
		if req.Name == "" {
			response.OK(c, WizardValidateResponse{Valid: false, Message: "請輸入 Policy 名稱"})
			return
		}
		response.OK(c, WizardValidateResponse{Valid: true})

	case 2:
		if len(req.PolicyTypes) == 0 {
			response.OK(c, WizardValidateResponse{Valid: false, Message: "至少選擇一個 Policy 型別（Ingress 或 Egress）"})
			return
		}
		for _, pt := range req.PolicyTypes {
			if pt == "Ingress" {
				for i, r := range req.Ingress {
					if r.FromType == "ipblock" && r.CIDR == "" {
						response.OK(c, WizardValidateResponse{Valid: false, Message: fmt.Sprintf("Ingress 規則 #%d：IPBlock 需要填入 CIDR", i+1)})
						return
					}
				}
			}
			if pt == "Egress" {
				for i, r := range req.Egress {
					if r.ToType == "ipblock" && r.CIDR == "" {
						response.OK(c, WizardValidateResponse{Valid: false, Message: fmt.Sprintf("Egress 規則 #%d：IPBlock 需要填入 CIDR", i+1)})
						return
					}
				}
			}
		}
		response.OK(c, WizardValidateResponse{Valid: true})

	case 3:
		// 產生 YAML 預覽
		np := buildNetworkPolicyFromWizard(req)
		yamlBytes, err := yaml.Marshal(np)
		if err != nil {
			response.OK(c, WizardValidateResponse{Valid: false, Message: "YAML 生成失敗: " + err.Error()})
			return
		}
		response.OK(c, WizardValidateResponse{Valid: true, YAML: string(yamlBytes)})

	default:
		response.BadRequest(c, "無效的 step 值")
	}
}

// buildNetworkPolicyFromWizard 從精靈請求建構 NetworkPolicy 物件
func buildNetworkPolicyFromWizard(req WizardValidateRequest) networkingv1.NetworkPolicy {
	np := networkingv1.NetworkPolicy{}
	np.APIVersion = "networking.k8s.io/v1"
	np.Kind = "NetworkPolicy"
	np.Name = req.Name
	np.Namespace = req.Namespace
	np.Spec.PodSelector.MatchLabels = req.Selector

	for _, pt := range req.PolicyTypes {
		np.Spec.PolicyTypes = append(np.Spec.PolicyTypes, networkingv1.PolicyType(pt))
	}

	for _, r := range req.Ingress {
		rule := networkingv1.NetworkPolicyIngressRule{}
		for _, p := range r.Ports {
			proto := corev1.ProtocolTCP
			if p.Protocol == "UDP" {
				proto = corev1.ProtocolUDP
			} else if p.Protocol == "SCTP" {
				proto = corev1.ProtocolSCTP
			}
			port := networkingv1.NetworkPolicyPort{Protocol: &proto}
			if p.Port != "" {
				intOrStr := k8sintstr.Parse(p.Port)
				port.Port = &intOrStr
			}
			rule.Ports = append(rule.Ports, port)
		}
		switch r.FromType {
		case "all":
			// 空 from = 允許所有
		case "ipblock":
			rule.From = append(rule.From, networkingv1.NetworkPolicyPeer{
				IPBlock: &networkingv1.IPBlock{CIDR: r.CIDR},
			})
		case "namespace":
			rule.From = append(rule.From, networkingv1.NetworkPolicyPeer{
				NamespaceSelector: &metav1.LabelSelector{MatchLabels: r.Selector},
			})
		case "pod":
			rule.From = append(rule.From, networkingv1.NetworkPolicyPeer{
				PodSelector: &metav1.LabelSelector{MatchLabels: r.Selector},
			})
		}
		np.Spec.Ingress = append(np.Spec.Ingress, rule)
	}

	for _, r := range req.Egress {
		rule := networkingv1.NetworkPolicyEgressRule{}
		for _, p := range r.Ports {
			proto := corev1.ProtocolTCP
			if p.Protocol == "UDP" {
				proto = corev1.ProtocolUDP
			} else if p.Protocol == "SCTP" {
				proto = corev1.ProtocolSCTP
			}
			port := networkingv1.NetworkPolicyPort{Protocol: &proto}
			if p.Port != "" {
				intOrStr := k8sintstr.Parse(p.Port)
				port.Port = &intOrStr
			}
			rule.Ports = append(rule.Ports, port)
		}
		switch r.ToType {
		case "all":
			// 空 to = 允許所有
		case "ipblock":
			rule.To = append(rule.To, networkingv1.NetworkPolicyPeer{
				IPBlock: &networkingv1.IPBlock{CIDR: r.CIDR},
			})
		case "namespace":
			rule.To = append(rule.To, networkingv1.NetworkPolicyPeer{
				NamespaceSelector: &metav1.LabelSelector{MatchLabels: r.Selector},
			})
		case "pod":
			rule.To = append(rule.To, networkingv1.NetworkPolicyPeer{
				PodSelector: &metav1.LabelSelector{MatchLabels: r.Selector},
			})
		}
		np.Spec.Egress = append(np.Spec.Egress, rule)
	}

	return np
}
