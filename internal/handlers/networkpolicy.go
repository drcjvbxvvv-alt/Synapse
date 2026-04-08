package handlers

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/shaia/Synapse/internal/k8s"
	"github.com/shaia/Synapse/internal/middleware"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/pkg/logger"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sintstr "k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/yaml"
)

// NetworkPolicyHandler NetworkPolicy 處理器
type NetworkPolicyHandler struct {
	clusterService *services.ClusterService
	k8sMgr         *k8s.ClusterInformerManager
}

// NewNetworkPolicyHandler 建立 NetworkPolicy 處理器
func NewNetworkPolicyHandler(clusterService *services.ClusterService, k8sMgr *k8s.ClusterInformerManager) *NetworkPolicyHandler {
	return &NetworkPolicyHandler{
		clusterService: clusterService,
		k8sMgr:         k8sMgr,
	}
}

// NetworkPolicyInfo NetworkPolicy 摘要資訊
type NetworkPolicyInfo struct {
	Name          string            `json:"name"`
	Namespace     string            `json:"namespace"`
	PodSelector   map[string]string `json:"podSelector"`
	PolicyTypes   []string          `json:"policyTypes"`
	IngressRules  int               `json:"ingressRules"`
	EgressRules   int               `json:"egressRules"`
	CreatedAt     time.Time         `json:"createdAt"`
	Labels        map[string]string `json:"labels"`
	Annotations   map[string]string `json:"annotations"`
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

// getClient 取得叢集客戶端
func (h *NetworkPolicyHandler) getClient(c *gin.Context) (kubernetes.Interface, uint, bool) {
	clusterIDStr := c.Param("clusterID")
	clusterID, err := parseClusterID(clusterIDStr)
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return nil, 0, false
	}

	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		logger.Error("取得叢集失敗", "error", err, "clusterId", clusterID)
		response.NotFound(c, "叢集不存在")
		return nil, 0, false
	}

	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		logger.Error("取得 K8s 客戶端失敗", "error", err, "clusterId", clusterID)
		response.InternalError(c, fmt.Sprintf("取得 K8s 客戶端失敗: %v", err))
		return nil, 0, false
	}

	return k8sClient.GetClientset(), clusterID, true
}

// ListNetworkPolicies 取得 NetworkPolicy 列表
func (h *NetworkPolicyHandler) ListNetworkPolicies(c *gin.Context) {
	clientset, clusterID, ok := h.getClient(c)
	if !ok {
		return
	}

	namespace := c.DefaultQuery("namespace", "")
	search := c.DefaultQuery("search", "")
	page := parsePage(c)
	pageSize := parsePageSize(c, 20)

	// 命名空間權限檢查
	nsInfo, hasAccess := middleware.CheckNamespacePermission(c, namespace)
	if !hasAccess {
		middleware.ForbiddenNS(c, nsInfo)
		return
	}

	policies, err := h.listPolicies(clientset, namespace)
	if err != nil {
		logger.Error("取得 NetworkPolicy 列表失敗", "error", err, "clusterId", clusterID)
		response.InternalError(c, fmt.Sprintf("取得 NetworkPolicy 列表失敗: %v", err))
		return
	}

	// 依命名空間權限過濾
	if !nsInfo.HasAllAccess && namespace == "" {
		policies = middleware.FilterResourcesByNamespace(c, policies, func(p NetworkPolicyInfo) string {
			return p.Namespace
		})
	}

	// 關鍵字搜尋
	if search != "" {
		sl := strings.ToLower(search)
		filtered := policies[:0]
		for _, p := range policies {
			if strings.Contains(strings.ToLower(p.Name), sl) ||
				strings.Contains(strings.ToLower(p.Namespace), sl) {
				filtered = append(filtered, p)
			}
		}
		policies = filtered
	}

	// 依建立時間倒序排列
	sort.Slice(policies, func(i, j int) bool {
		return policies[i].CreatedAt.After(policies[j].CreatedAt)
	})

	// 分頁
	total := len(policies)
	start := (page - 1) * pageSize
	end := start + pageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	response.PagedList(c, policies[start:end], int64(total), page, pageSize)
}

// GetNetworkPolicy 取得單一 NetworkPolicy 詳情
func (h *NetworkPolicyHandler) GetNetworkPolicy(c *gin.Context) {
	clientset, clusterID, ok := h.getClient(c)
	if !ok {
		return
	}

	namespace := c.Param("namespace")
	name := c.Param("name")

	np, err := clientset.NetworkingV1().NetworkPolicies(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		logger.Error("取得 NetworkPolicy 失敗", "error", err, "clusterId", clusterID)
		response.InternalError(c, fmt.Sprintf("取得 NetworkPolicy 失敗: %v", err))
		return
	}

	response.OK(c, h.convertToDetail(np))
}

// GetNetworkPolicyYAML 取得 NetworkPolicy YAML
func (h *NetworkPolicyHandler) GetNetworkPolicyYAML(c *gin.Context) {
	clientset, clusterID, ok := h.getClient(c)
	if !ok {
		return
	}

	namespace := c.Param("namespace")
	name := c.Param("name")

	np, err := clientset.NetworkingV1().NetworkPolicies(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		logger.Error("取得 NetworkPolicy 失敗", "error", err, "clusterId", clusterID)
		response.InternalError(c, fmt.Sprintf("取得 NetworkPolicy 失敗: %v", err))
		return
	}

	clean := np.DeepCopy()
	clean.APIVersion = "networking.k8s.io/v1"
	clean.Kind = "NetworkPolicy"
	clean.ManagedFields = nil

	yamlData, err := yaml.Marshal(clean)
	if err != nil {
		response.InternalError(c, fmt.Sprintf("轉換 YAML 失敗: %v", err))
		return
	}

	response.OK(c, gin.H{"yaml": string(yamlData)})
}

// CreateNetworkPolicy 建立 NetworkPolicy（僅支援 YAML 模式）
func (h *NetworkPolicyHandler) CreateNetworkPolicy(c *gin.Context) {
	clientset, clusterID, ok := h.getClient(c)
	if !ok {
		return
	}

	var req struct {
		Namespace string `json:"namespace" binding:"required"`
		YAML      string `json:"yaml" binding:"required"`
		DryRun    bool   `json:"dryRun"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "參數錯誤: "+err.Error())
		return
	}

	var np networkingv1.NetworkPolicy
	if err := yaml.Unmarshal([]byte(req.YAML), &np); err != nil {
		response.BadRequest(c, "YAML 解析失敗: "+err.Error())
		return
	}
	if np.Namespace == "" {
		np.Namespace = req.Namespace
	}

	var dryRunOpt []string
	if req.DryRun {
		dryRunOpt = []string{metav1.DryRunAll}
	}

	created, err := clientset.NetworkingV1().NetworkPolicies(np.Namespace).Create(context.Background(), &np, metav1.CreateOptions{DryRun: dryRunOpt})
	if err != nil {
		logger.Error("建立 NetworkPolicy 失敗", "error", err, "clusterId", clusterID)
		response.InternalError(c, fmt.Sprintf("建立 NetworkPolicy 失敗: %v", err))
		return
	}

	if !req.DryRun {
		logger.Info("NetworkPolicy 建立成功", "clusterId", clusterID, "namespace", created.Namespace, "name", created.Name)
	}
	response.OK(c, h.convertToInfo(created))
}

// UpdateNetworkPolicy 更新 NetworkPolicy（YAML 模式）
func (h *NetworkPolicyHandler) UpdateNetworkPolicy(c *gin.Context) {
	clientset, clusterID, ok := h.getClient(c)
	if !ok {
		return
	}

	namespace := c.Param("namespace")
	name := c.Param("name")

	var req struct {
		YAML string `json:"yaml" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "參數錯誤: "+err.Error())
		return
	}

	var np networkingv1.NetworkPolicy
	if err := yaml.Unmarshal([]byte(req.YAML), &np); err != nil {
		response.BadRequest(c, "YAML 解析失敗: "+err.Error())
		return
	}

	existing, err := clientset.NetworkingV1().NetworkPolicies(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		response.InternalError(c, fmt.Sprintf("取得現有 NetworkPolicy 失敗: %v", err))
		return
	}

	np.ResourceVersion = existing.ResourceVersion
	np.Namespace = namespace
	np.Name = name

	updated, err := clientset.NetworkingV1().NetworkPolicies(namespace).Update(context.Background(), &np, metav1.UpdateOptions{})
	if err != nil {
		logger.Error("更新 NetworkPolicy 失敗", "error", err, "clusterId", clusterID)
		response.InternalError(c, fmt.Sprintf("更新 NetworkPolicy 失敗: %v", err))
		return
	}

	logger.Info("NetworkPolicy 更新成功", "clusterId", clusterID, "namespace", updated.Namespace, "name", updated.Name)
	response.OK(c, h.convertToInfo(updated))
}

// DeleteNetworkPolicy 刪除 NetworkPolicy
func (h *NetworkPolicyHandler) DeleteNetworkPolicy(c *gin.Context) {
	clientset, clusterID, ok := h.getClient(c)
	if !ok {
		return
	}

	namespace := c.Param("namespace")
	name := c.Param("name")

	err := clientset.NetworkingV1().NetworkPolicies(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
	if err != nil {
		logger.Error("刪除 NetworkPolicy 失敗", "error", err, "clusterId", clusterID)
		response.InternalError(c, fmt.Sprintf("刪除 NetworkPolicy 失敗: %v", err))
		return
	}

	logger.Info("NetworkPolicy 刪除成功", "clusterId", clusterID, "namespace", namespace, "name", name)
	response.NoContent(c)
}

// ---- Topology ----

// TopologyNode 拓撲節點
type TopologyNode struct {
	ID          string            `json:"id"`
	Type        string            `json:"type"`      // "podgroup" | "namespace" | "ipblock" | "external"
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

	policies, err := clientset.NetworkingV1().NetworkPolicies(ns).List(context.Background(), metav1.ListOptions{})
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
	Ports       []WizardPort `json:"ports,omitempty"`
	FromType    string       `json:"fromType"`    // "pod" | "namespace" | "ipblock" | "all"
	Selector    map[string]string `json:"selector,omitempty"`
	CIDR        string       `json:"cidr,omitempty"`
}

// WizardEgressRule 精靈 Egress 規則
type WizardEgressRule struct {
	Ports    []WizardPort `json:"ports,omitempty"`
	ToType   string       `json:"toType"`    // "pod" | "namespace" | "ipblock" | "all"
	Selector map[string]string `json:"selector,omitempty"`
	CIDR     string       `json:"cidr,omitempty"`
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
				iv := networkingv1.NetworkPolicyPort{}
				_ = iv
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

// ---- helpers ----

func (h *NetworkPolicyHandler) listPolicies(clientset kubernetes.Interface, namespace string) ([]NetworkPolicyInfo, error) {
	ns := namespace
	if ns == "_all_" {
		ns = ""
	}
	list, err := clientset.NetworkingV1().NetworkPolicies(ns).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	result := make([]NetworkPolicyInfo, 0, len(list.Items))
	for i := range list.Items {
		result = append(result, h.convertToInfo(&list.Items[i]))
	}
	return result, nil
}

func (h *NetworkPolicyHandler) convertToInfo(np *networkingv1.NetworkPolicy) NetworkPolicyInfo {
	types := make([]string, 0, len(np.Spec.PolicyTypes))
	for _, t := range np.Spec.PolicyTypes {
		types = append(types, string(t))
	}
	return NetworkPolicyInfo{
		Name:         np.Name,
		Namespace:    np.Namespace,
		PodSelector:  np.Spec.PodSelector.MatchLabels,
		PolicyTypes:  types,
		IngressRules: len(np.Spec.Ingress),
		EgressRules:  len(np.Spec.Egress),
		CreatedAt:    np.CreationTimestamp.Time,
		Labels:       np.Labels,
		Annotations:  np.Annotations,
	}
}

func (h *NetworkPolicyHandler) convertToDetail(np *networkingv1.NetworkPolicy) NetworkPolicyDetail {
	info := h.convertToInfo(np)
	detail := NetworkPolicyDetail{NetworkPolicyInfo: info}

	for _, rule := range np.Spec.Ingress {
		r := NetworkPolicyIngressRule{}
		for _, p := range rule.Ports {
			np_ := NetworkPolicyPort{}
			if p.Protocol != nil {
				np_.Protocol = string(*p.Protocol)
			}
			if p.Port != nil {
				np_.Port = p.Port.String()
			}
			np_.EndPort = p.EndPort
			r.Ports = append(r.Ports, np_)
		}
		for _, peer := range rule.From {
			r.From = append(r.From, convertPeer(peer))
		}
		detail.Ingress = append(detail.Ingress, r)
	}

	for _, rule := range np.Spec.Egress {
		r := NetworkPolicyEgressRule{}
		for _, p := range rule.Ports {
			np_ := NetworkPolicyPort{}
			if p.Protocol != nil {
				np_.Protocol = string(*p.Protocol)
			}
			if p.Port != nil {
				np_.Port = p.Port.String()
			}
			np_.EndPort = p.EndPort
			r.Ports = append(r.Ports, np_)
		}
		for _, peer := range rule.To {
			r.To = append(r.To, convertPeer(peer))
		}
		detail.Egress = append(detail.Egress, r)
	}

	return detail
}

func convertPeer(peer networkingv1.NetworkPolicyPeer) NetworkPolicyPeer {
	p := NetworkPolicyPeer{}
	if peer.PodSelector != nil {
		p.PodSelector = &LabelSelectorInfo{MatchLabels: peer.PodSelector.MatchLabels}
	}
	if peer.NamespaceSelector != nil {
		p.NamespaceSelector = &LabelSelectorInfo{MatchLabels: peer.NamespaceSelector.MatchLabels}
	}
	if peer.IPBlock != nil {
		p.IPBlock = &IPBlockInfo{
			CIDR:   peer.IPBlock.CIDR,
			Except: peer.IPBlock.Except,
		}
	}
	return p
}

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
