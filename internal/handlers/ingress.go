package handlers

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/shaia/Synapse/internal/config"
	"github.com/shaia/Synapse/internal/k8s"
	"github.com/shaia/Synapse/internal/middleware"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/pkg/logger"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/yaml"
)

// IngressHandler Ingress處理器
type IngressHandler struct {
	db             *gorm.DB
	cfg            *config.Config
	clusterService *services.ClusterService
	k8sMgr         *k8s.ClusterInformerManager
}

// NewIngressHandler 建立Ingress處理器
func NewIngressHandler(db *gorm.DB, cfg *config.Config, clusterService *services.ClusterService, k8sMgr *k8s.ClusterInformerManager) *IngressHandler {
	return &IngressHandler{
		db:             db,
		cfg:            cfg,
		clusterService: clusterService,
		k8sMgr:         k8sMgr,
	}
}

// IngressInfo Ingress資訊
type IngressInfo struct {
	Name             string               `json:"name"`
	Namespace        string               `json:"namespace"`
	IngressClassName *string              `json:"ingressClassName,omitempty"`
	Rules            []IngressRuleInfo    `json:"rules"`
	TLS              []IngressTLSInfo     `json:"tls,omitempty"`
	LoadBalancer     []LoadBalancerStatus `json:"loadBalancer,omitempty"`
	CreatedAt        time.Time            `json:"createdAt"`
	Labels           map[string]string    `json:"labels"`
	Annotations      map[string]string    `json:"annotations"`
}

// IngressRuleInfo Ingress規則資訊
type IngressRuleInfo struct {
	Host  string            `json:"host"`
	Paths []IngressPathInfo `json:"paths"`
}

// IngressPathInfo Ingress路徑資訊
type IngressPathInfo struct {
	Path        string `json:"path"`
	PathType    string `json:"pathType"`
	ServiceName string `json:"serviceName"`
	ServicePort string `json:"servicePort"`
}

// IngressTLSInfo Ingress TLS資訊
type IngressTLSInfo struct {
	Hosts      []string `json:"hosts"`
	SecretName string   `json:"secretName"`
}

// LoadBalancerStatus 負載均衡器狀態
type LoadBalancerStatus struct {
	IP       string `json:"ip,omitempty"`
	Hostname string `json:"hostname,omitempty"`
}

// ListIngresses 獲取Ingress列表
func (h *IngressHandler) ListIngresses(c *gin.Context) {
	clusterIDStr := c.Param("clusterID")
	clusterID, err := parseClusterID(clusterIDStr)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

	// 獲取查詢參數
	namespace := c.DefaultQuery("namespace", "")
	ingressClass := c.DefaultQuery("ingressClass", "")
	search := c.DefaultQuery("search", "")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))

	// 從叢集服務獲取叢集資訊
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		logger.Error("獲取叢集失敗", "error", err, "clusterId", clusterID)
		response.NotFound(c, "叢集不存在")
		return
	}

	// 獲取快取的 K8s 客戶端
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		logger.Error("獲取K8s客戶端失敗", "error", err, "clusterId", clusterID)
		response.InternalError(c, fmt.Sprintf("獲取K8s客戶端失敗: %v", err))
		return
	}

	clientset := k8sClient.GetClientset()

	// 檢查命名空間權限
	nsInfo, hasAccess := middleware.CheckNamespacePermission(c, namespace)
	if !hasAccess {
		middleware.ForbiddenNS(c, nsInfo)
		return
	}

	// 獲取Ingresses
	ingresses, err := h.getIngresses(clientset, namespace)
	if err != nil {
		logger.Error("獲取Ingresses失敗", "error", err, "clusterId", clusterID)
		response.InternalError(c, fmt.Sprintf("獲取Ingresses失敗: %v", err))
		return
	}

	// 根據命名空間權限過濾
	if !nsInfo.HasAllAccess && namespace == "" {
		ingresses = middleware.FilterResourcesByNamespace(c, ingresses, func(i IngressInfo) string {
			return i.Namespace
		})
	}

	// 過濾和搜尋
	filteredIngresses := h.filterIngresses(ingresses, ingressClass, search)

	// 排序
	sort.Slice(filteredIngresses, func(i, j int) bool {
		return filteredIngresses[i].CreatedAt.After(filteredIngresses[j].CreatedAt)
	})

	// 分頁
	total := len(filteredIngresses)
	start := (page - 1) * pageSize
	end := start + pageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}
	pagedIngresses := filteredIngresses[start:end]

	response.PagedList(c, pagedIngresses, int64(total), page, pageSize)
}

// GetIngress 獲取單個Ingress詳情
func (h *IngressHandler) GetIngress(c *gin.Context) {
	clusterIDStr := c.Param("clusterID")
	clusterID, err := parseClusterID(clusterIDStr)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

	namespace := c.Param("namespace")
	name := c.Param("name")

	// 從叢集服務獲取叢集資訊
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		logger.Error("獲取叢集失敗", "error", err, "clusterId", clusterID)
		response.NotFound(c, "叢集不存在")
		return
	}

	// 獲取快取的 K8s 客戶端
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		logger.Error("獲取K8s客戶端失敗", "error", err, "clusterId", clusterID)
		response.InternalError(c, fmt.Sprintf("獲取K8s客戶端失敗: %v", err))
		return
	}

	clientset := k8sClient.GetClientset()

	// 獲取Ingress
	ingress, err := clientset.NetworkingV1().Ingresses(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		logger.Error("獲取Ingress失敗", "error", err, "clusterId", clusterID, "namespace", namespace, "name", name)
		response.InternalError(c, fmt.Sprintf("獲取Ingress失敗: %v", err))
		return
	}

	ingressInfo := h.convertToIngressInfo(ingress)

	response.OK(c, ingressInfo)
}

// GetIngressYAML 獲取Ingress的YAML
func (h *IngressHandler) GetIngressYAML(c *gin.Context) {
	clusterIDStr := c.Param("clusterID")
	clusterID, err := parseClusterID(clusterIDStr)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

	namespace := c.Param("namespace")
	name := c.Param("name")

	// 從叢集服務獲取叢集資訊
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		logger.Error("獲取叢集失敗", "error", err, "clusterId", clusterID)
		response.NotFound(c, "叢集不存在")
		return
	}

	// 獲取快取的 K8s 客戶端
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		logger.Error("獲取K8s客戶端失敗", "error", err, "clusterId", clusterID)
		response.InternalError(c, fmt.Sprintf("獲取K8s客戶端失敗: %v", err))
		return
	}

	clientset := k8sClient.GetClientset()

	// 獲取Ingress
	ingress, err := clientset.NetworkingV1().Ingresses(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		logger.Error("獲取Ingress失敗", "error", err, "clusterId", clusterID, "namespace", namespace, "name", name)
		response.InternalError(c, fmt.Sprintf("獲取Ingress失敗: %v", err))
		return
	}

	// 設定 apiVersion 和 kind（API 返回的物件不包含這些欄位）
	cleanIng := ingress.DeepCopy()
	cleanIng.APIVersion = "networking.k8s.io/v1"
	cleanIng.Kind = "Ingress"
	cleanIng.ManagedFields = nil // 移除 managedFields 簡化 YAML

	// 轉換為YAML
	yamlData, err := yaml.Marshal(cleanIng)
	if err != nil {
		logger.Error("轉換YAML失敗", "error", err)
		response.InternalError(c, fmt.Sprintf("轉換YAML失敗: %v", err))
		return
	}

	response.OK(c, gin.H{"yaml": string(yamlData)})
}

// DeleteIngress 刪除Ingress
func (h *IngressHandler) DeleteIngress(c *gin.Context) {
	clusterIDStr := c.Param("clusterID")
	clusterID, err := parseClusterID(clusterIDStr)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

	namespace := c.Param("namespace")
	name := c.Param("name")

	// 從叢集服務獲取叢集資訊
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		logger.Error("獲取叢集失敗", "error", err, "clusterId", clusterID)
		response.NotFound(c, "叢集不存在")
		return
	}

	// 獲取快取的 K8s 客戶端
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		logger.Error("獲取K8s客戶端失敗", "error", err, "clusterId", clusterID)
		response.InternalError(c, fmt.Sprintf("獲取K8s客戶端失敗: %v", err))
		return
	}

	clientset := k8sClient.GetClientset()

	// 刪除Ingress
	err = clientset.NetworkingV1().Ingresses(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
	if err != nil {
		logger.Error("刪除Ingress失敗", "error", err, "clusterId", clusterID, "namespace", namespace, "name", name)
		response.InternalError(c, fmt.Sprintf("刪除Ingress失敗: %v", err))
		return
	}

	logger.Info("Ingress刪除成功", "clusterId", clusterID, "namespace", namespace, "name", name)
	response.NoContent(c)
}

// 輔助函式

// getIngresses 獲取Ingresses
func (h *IngressHandler) getIngresses(clientset kubernetes.Interface, namespace string) ([]IngressInfo, error) {
	var ingressList *networkingv1.IngressList
	var err error

	if namespace == "" || namespace == "_all_" {
		ingressList, err = clientset.NetworkingV1().Ingresses("").List(context.Background(), metav1.ListOptions{})
	} else {
		ingressList, err = clientset.NetworkingV1().Ingresses(namespace).List(context.Background(), metav1.ListOptions{})
	}

	if err != nil {
		return nil, err
	}

	ingresses := make([]IngressInfo, 0, len(ingressList.Items))
	for _, ing := range ingressList.Items {
		ingresses = append(ingresses, h.convertToIngressInfo(&ing))
	}

	return ingresses, nil
}

// convertToIngressInfo 轉換為IngressInfo
func (h *IngressHandler) convertToIngressInfo(ing *networkingv1.Ingress) IngressInfo {
	// 轉換規則
	rules := make([]IngressRuleInfo, 0, len(ing.Spec.Rules))
	for _, rule := range ing.Spec.Rules {
		paths := make([]IngressPathInfo, 0)
		if rule.HTTP != nil {
			for _, path := range rule.HTTP.Paths {
				pathType := ""
				if path.PathType != nil {
					pathType = string(*path.PathType)
				}

				servicePort := ""
				if path.Backend.Service != nil {
					if path.Backend.Service.Port.Number > 0 {
						servicePort = strconv.Itoa(int(path.Backend.Service.Port.Number))
					} else {
						servicePort = path.Backend.Service.Port.Name
					}
				}

				paths = append(paths, IngressPathInfo{
					Path:     path.Path,
					PathType: pathType,
					ServiceName: func() string {
						if path.Backend.Service != nil {
							return path.Backend.Service.Name
						}
						return ""
					}(),
					ServicePort: servicePort,
				})
			}
		}

		rules = append(rules, IngressRuleInfo{
			Host:  rule.Host,
			Paths: paths,
		})
	}

	// 轉換TLS
	tls := make([]IngressTLSInfo, 0, len(ing.Spec.TLS))
	for _, t := range ing.Spec.TLS {
		tls = append(tls, IngressTLSInfo{
			Hosts:      t.Hosts,
			SecretName: t.SecretName,
		})
	}

	// 轉換LoadBalancer狀態
	lbStatus := make([]LoadBalancerStatus, 0, len(ing.Status.LoadBalancer.Ingress))
	for _, lb := range ing.Status.LoadBalancer.Ingress {
		lbStatus = append(lbStatus, LoadBalancerStatus{
			IP:       lb.IP,
			Hostname: lb.Hostname,
		})
	}

	return IngressInfo{
		Name:             ing.Name,
		Namespace:        ing.Namespace,
		IngressClassName: ing.Spec.IngressClassName,
		Rules:            rules,
		TLS:              tls,
		LoadBalancer:     lbStatus,
		CreatedAt:        ing.CreationTimestamp.Time,
		Labels:           ing.Labels,
		Annotations:      ing.Annotations,
	}
}

// filterIngresses 過濾Ingresses
func (h *IngressHandler) filterIngresses(ingresses []IngressInfo, ingressClass, search string) []IngressInfo {
	filtered := make([]IngressInfo, 0)
	for _, ing := range ingresses {
		// IngressClass過濾
		if ingressClass != "" {
			if ing.IngressClassName == nil || *ing.IngressClassName != ingressClass {
				continue
			}
		}

		// 搜尋過濾
		if search != "" {
			searchLower := strings.ToLower(search)
			matched := false

			// 匹配名稱和命名空間
			if strings.Contains(strings.ToLower(ing.Name), searchLower) ||
				strings.Contains(strings.ToLower(ing.Namespace), searchLower) {
				matched = true
			}

			// 匹配Host和路徑
			for _, rule := range ing.Rules {
				if strings.Contains(strings.ToLower(rule.Host), searchLower) {
					matched = true
					break
				}
				for _, path := range rule.Paths {
					if strings.Contains(strings.ToLower(path.ServiceName), searchLower) ||
						strings.Contains(strings.ToLower(path.Path), searchLower) {
						matched = true
						break
					}
				}
				if matched {
					break
				}
			}

			if !matched {
				continue
			}
		}

		filtered = append(filtered, ing)
	}
	return filtered
}

// CreateIngressRequest 建立Ingress請求
type CreateIngressRequest struct {
	Namespace string           `json:"namespace" binding:"required"`
	YAML      string           `json:"yaml,omitempty"`     // YAML方式建立
	FormData  *IngressFormData `json:"formData,omitempty"` // 表單方式建立
}

// IngressFormData Ingress表單資料
type IngressFormData struct {
	Name             string                `json:"name" binding:"required"`
	IngressClassName *string               `json:"ingressClassName,omitempty"`
	Rules            []IngressRuleFormData `json:"rules" binding:"required"`
	TLS              []IngressTLSFormData  `json:"tls,omitempty"`
	Labels           map[string]string     `json:"labels,omitempty"`
	Annotations      map[string]string     `json:"annotations,omitempty"`
}

// IngressRuleFormData Ingress規則表單資料
type IngressRuleFormData struct {
	Host  string                `json:"host"`
	Paths []IngressPathFormData `json:"paths" binding:"required"`
}

// IngressPathFormData Ingress路徑表單資料
type IngressPathFormData struct {
	Path        string `json:"path" binding:"required"`
	PathType    string `json:"pathType" binding:"required"` // Prefix, Exact, ImplementationSpecific
	ServiceName string `json:"serviceName" binding:"required"`
	ServicePort int32  `json:"servicePort" binding:"required"`
}

// IngressTLSFormData Ingress TLS表單資料
type IngressTLSFormData struct {
	Hosts      []string `json:"hosts" binding:"required"`
	SecretName string   `json:"secretName" binding:"required"`
}

// CreateIngress 建立Ingress
func (h *IngressHandler) CreateIngress(c *gin.Context) {
	clusterIDStr := c.Param("clusterID")
	clusterID, err := parseClusterID(clusterIDStr)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

	var req CreateIngressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "參數錯誤: "+err.Error())
		return
	}

	// 從叢集服務獲取叢集資訊
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		logger.Error("獲取叢集失敗", "error", err, "clusterId", clusterID)
		response.NotFound(c, "叢集不存在")
		return
	}

	// 獲取快取的 K8s 客戶端
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		logger.Error("獲取K8s客戶端失敗", "error", err, "clusterId", clusterID)
		response.InternalError(c, fmt.Sprintf("獲取K8s客戶端失敗: %v", err))
		return
	}

	clientset := k8sClient.GetClientset()

	var ingress *networkingv1.Ingress

	// 根據建立方式選擇處理邏輯
	if req.YAML != "" {
		// YAML方式建立
		ingress, err = h.createIngressFromYAML(clientset, req.Namespace, req.YAML)
	} else if req.FormData != nil {
		// 表單方式建立
		ingress, err = h.createIngressFromForm(clientset, req.Namespace, req.FormData)
	} else {
		response.BadRequest(c, "必須提供YAML或表單資料")
		return
	}

	if err != nil {
		logger.Error("建立Ingress失敗", "error", err, "clusterId", clusterID)
		response.InternalError(c, fmt.Sprintf("建立Ingress失敗: %v", err))
		return
	}

	logger.Info("Ingress建立成功", "clusterId", clusterID, "namespace", ingress.Namespace, "name", ingress.Name)
	response.OK(c, h.convertToIngressInfo(ingress))
}

// UpdateIngress 更新Ingress
func (h *IngressHandler) UpdateIngress(c *gin.Context) {
	clusterIDStr := c.Param("clusterID")
	clusterID, err := parseClusterID(clusterIDStr)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}
	namespace := c.Param("namespace")
	name := c.Param("name")

	var req CreateIngressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "參數錯誤: "+err.Error())
		return
	}

	// 從叢集服務獲取叢集資訊
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		logger.Error("獲取叢集失敗", "error", err, "clusterId", clusterID)
		response.NotFound(c, "叢集不存在")
		return
	}

	// 獲取快取的 K8s 客戶端
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		logger.Error("獲取K8s客戶端失敗", "error", err, "clusterId", clusterID)
		response.InternalError(c, fmt.Sprintf("獲取K8s客戶端失敗: %v", err))
		return
	}

	clientset := k8sClient.GetClientset()

	var ingress *networkingv1.Ingress

	// 根據更新方式選擇處理邏輯
	if req.YAML != "" {
		// YAML方式更新
		ingress, err = h.updateIngressFromYAML(clientset, namespace, name, req.YAML)
	} else if req.FormData != nil {
		// 表單方式更新
		ingress, err = h.updateIngressFromForm(clientset, namespace, name, req.FormData)
	} else {
		response.BadRequest(c, "必須提供YAML或表單資料")
		return
	}

	if err != nil {
		logger.Error("更新Ingress失敗", "error", err, "clusterId", clusterID)
		response.InternalError(c, fmt.Sprintf("更新Ingress失敗: %v", err))
		return
	}

	logger.Info("Ingress更新成功", "clusterId", clusterID, "namespace", ingress.Namespace, "name", ingress.Name)
	response.OK(c, h.convertToIngressInfo(ingress))
}

// createIngressFromYAML 從YAML建立Ingress
func (h *IngressHandler) createIngressFromYAML(clientset kubernetes.Interface, namespace, yamlContent string) (*networkingv1.Ingress, error) {
	var ingress networkingv1.Ingress
	if err := yaml.Unmarshal([]byte(yamlContent), &ingress); err != nil {
		return nil, fmt.Errorf("解析YAML失敗: %w", err)
	}

	// 確保namespace正確
	if ingress.Namespace == "" {
		ingress.Namespace = namespace
	}

	createdIngress, err := clientset.NetworkingV1().Ingresses(ingress.Namespace).Create(context.Background(), &ingress, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	return createdIngress, nil
}

// createIngressFromForm 從表單建立Ingress
func (h *IngressHandler) createIngressFromForm(clientset kubernetes.Interface, namespace string, formData *IngressFormData) (*networkingv1.Ingress, error) {
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        formData.Name,
			Namespace:   namespace,
			Labels:      formData.Labels,
			Annotations: formData.Annotations,
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: formData.IngressClassName,
		},
	}

	// 新增規則
	rules := make([]networkingv1.IngressRule, 0, len(formData.Rules))
	for _, r := range formData.Rules {
		paths := make([]networkingv1.HTTPIngressPath, 0, len(r.Paths))
		for _, p := range r.Paths {
			pathType := networkingv1.PathType(p.PathType)
			paths = append(paths, networkingv1.HTTPIngressPath{
				Path:     p.Path,
				PathType: &pathType,
				Backend: networkingv1.IngressBackend{
					Service: &networkingv1.IngressServiceBackend{
						Name: p.ServiceName,
						Port: networkingv1.ServiceBackendPort{
							Number: p.ServicePort,
						},
					},
				},
			})
		}

		rules = append(rules, networkingv1.IngressRule{
			Host: r.Host,
			IngressRuleValue: networkingv1.IngressRuleValue{
				HTTP: &networkingv1.HTTPIngressRuleValue{
					Paths: paths,
				},
			},
		})
	}
	ingress.Spec.Rules = rules

	// 新增TLS
	if len(formData.TLS) > 0 {
		tls := make([]networkingv1.IngressTLS, 0, len(formData.TLS))
		for _, t := range formData.TLS {
			tls = append(tls, networkingv1.IngressTLS{
				Hosts:      t.Hosts,
				SecretName: t.SecretName,
			})
		}
		ingress.Spec.TLS = tls
	}

	createdIngress, err := clientset.NetworkingV1().Ingresses(namespace).Create(context.Background(), ingress, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	return createdIngress, nil
}

// updateIngressFromYAML 從YAML更新Ingress
func (h *IngressHandler) updateIngressFromYAML(clientset kubernetes.Interface, namespace, name, yamlContent string) (*networkingv1.Ingress, error) {
	var ingress networkingv1.Ingress
	if err := yaml.Unmarshal([]byte(yamlContent), &ingress); err != nil {
		return nil, fmt.Errorf("解析YAML失敗: %w", err)
	}

	// 獲取現有Ingress
	existingIngress, err := clientset.NetworkingV1().Ingresses(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	// 保留ResourceVersion
	ingress.ResourceVersion = existingIngress.ResourceVersion
	ingress.Namespace = namespace
	ingress.Name = name

	updatedIngress, err := clientset.NetworkingV1().Ingresses(namespace).Update(context.Background(), &ingress, metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}

	return updatedIngress, nil
}

// updateIngressFromForm 從表單更新Ingress
func (h *IngressHandler) updateIngressFromForm(clientset kubernetes.Interface, namespace, name string, formData *IngressFormData) (*networkingv1.Ingress, error) {
	// 獲取現有Ingress
	existingIngress, err := clientset.NetworkingV1().Ingresses(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	// 更新Spec
	existingIngress.Spec.IngressClassName = formData.IngressClassName

	// 更新規則
	rules := make([]networkingv1.IngressRule, 0, len(formData.Rules))
	for _, r := range formData.Rules {
		paths := make([]networkingv1.HTTPIngressPath, 0, len(r.Paths))
		for _, p := range r.Paths {
			pathType := networkingv1.PathType(p.PathType)
			paths = append(paths, networkingv1.HTTPIngressPath{
				Path:     p.Path,
				PathType: &pathType,
				Backend: networkingv1.IngressBackend{
					Service: &networkingv1.IngressServiceBackend{
						Name: p.ServiceName,
						Port: networkingv1.ServiceBackendPort{
							Number: p.ServicePort,
						},
					},
				},
			})
		}

		rules = append(rules, networkingv1.IngressRule{
			Host: r.Host,
			IngressRuleValue: networkingv1.IngressRuleValue{
				HTTP: &networkingv1.HTTPIngressRuleValue{
					Paths: paths,
				},
			},
		})
	}
	existingIngress.Spec.Rules = rules

	// 更新TLS
	if len(formData.TLS) > 0 {
		tls := make([]networkingv1.IngressTLS, 0, len(formData.TLS))
		for _, t := range formData.TLS {
			tls = append(tls, networkingv1.IngressTLS{
				Hosts:      t.Hosts,
				SecretName: t.SecretName,
			})
		}
		existingIngress.Spec.TLS = tls
	} else {
		existingIngress.Spec.TLS = nil
	}

	// 更新Labels和Annotations
	if formData.Labels != nil {
		existingIngress.Labels = formData.Labels
	}
	if formData.Annotations != nil {
		existingIngress.Annotations = formData.Annotations
	}

	updatedIngress, err := clientset.NetworkingV1().Ingresses(namespace).Update(context.Background(), existingIngress, metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}

	return updatedIngress, nil
}

// GetIngressNamespaces 獲取Ingress所在的命名空間列表
func (h *IngressHandler) GetIngressNamespaces(c *gin.Context) {
	clusterID := c.Param("clusterID")

	// 獲取叢集
	id, err := strconv.ParseUint(clusterID, 10, 32)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

	cluster, err := h.clusterService.GetCluster(uint(id))
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}

	// 獲取快取的 K8s 客戶端
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, fmt.Sprintf("獲取K8s客戶端失敗: %v", err))
		return
	}
	clientset := k8sClient.GetClientset()

	// 獲取所有Ingresses
	ingressList, err := clientset.NetworkingV1().Ingresses("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		logger.Error("獲取Ingress列表失敗", "cluster", cluster.Name, "error", err)
		response.InternalError(c, fmt.Sprintf("獲取Ingress列表失敗: %v", err))
		return
	}

	// 統計每個命名空間的Ingress數量
	nsMap := make(map[string]int)
	for _, ing := range ingressList.Items {
		nsMap[ing.Namespace]++
	}

	type NamespaceItem struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}

	var namespaces []NamespaceItem
	for ns, count := range nsMap {
		namespaces = append(namespaces, NamespaceItem{
			Name:  ns,
			Count: count,
		})
	}

	// 按名稱排序
	sort.Slice(namespaces, func(i, j int) bool {
		return namespaces[i].Name < namespaces[j].Name
	})

	response.OK(c, namespaces)
}
