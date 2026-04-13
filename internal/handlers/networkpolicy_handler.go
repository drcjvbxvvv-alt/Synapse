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
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	policies, err := h.listPolicies(ctx, clientset, namespace)
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

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	np, err := clientset.NetworkingV1().NetworkPolicies(namespace).Get(ctx, name, metav1.GetOptions{})
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

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	np, err := clientset.NetworkingV1().NetworkPolicies(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		logger.Error("取得 NetworkPolicy 失敗", "error", err, "clusterId", clusterID)
		response.InternalError(c, fmt.Sprintf("取得 NetworkPolicy 失敗: %v", err))
		return
	}

	clean := np.DeepCopy()
	clean.APIVersion = "networking.k8s.io/v1"
	clean.Kind = "NetworkPolicy"
	clean.ManagedFields = nil
	clean.Annotations = filterAnnotations(clean.Annotations)

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

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	created, err := clientset.NetworkingV1().NetworkPolicies(np.Namespace).Create(ctx, &np, metav1.CreateOptions{DryRun: dryRunOpt})
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

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	existing, err := clientset.NetworkingV1().NetworkPolicies(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		response.InternalError(c, fmt.Sprintf("取得現有 NetworkPolicy 失敗: %v", err))
		return
	}

	np.ResourceVersion = existing.ResourceVersion
	np.Namespace = namespace
	np.Name = name

	updated, err := clientset.NetworkingV1().NetworkPolicies(namespace).Update(ctx, &np, metav1.UpdateOptions{})
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

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	err := clientset.NetworkingV1().NetworkPolicies(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		logger.Error("刪除 NetworkPolicy 失敗", "error", err, "clusterId", clusterID)
		response.InternalError(c, fmt.Sprintf("刪除 NetworkPolicy 失敗: %v", err))
		return
	}

	logger.Info("NetworkPolicy 刪除成功", "clusterId", clusterID, "namespace", namespace, "name", name)
	response.NoContent(c)
}

// ---- helpers ----

func (h *NetworkPolicyHandler) listPolicies(ctx context.Context, clientset kubernetes.Interface, namespace string) ([]NetworkPolicyInfo, error) {
	ns := namespace
	if ns == "_all_" {
		ns = ""
	}
	list, err := clientset.NetworkingV1().NetworkPolicies(ns).List(ctx, metav1.ListOptions{})
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
