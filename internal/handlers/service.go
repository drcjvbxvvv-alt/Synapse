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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/yaml"
)

// ServiceHandler Service處理器
type ServiceHandler struct {
	db             *gorm.DB
	cfg            *config.Config
	clusterService *services.ClusterService
	k8sMgr         *k8s.ClusterInformerManager
}

// NewServiceHandler 建立Service處理器
func NewServiceHandler(db *gorm.DB, cfg *config.Config, clusterService *services.ClusterService, k8sMgr *k8s.ClusterInformerManager) *ServiceHandler {
	return &ServiceHandler{
		db:             db,
		cfg:            cfg,
		clusterService: clusterService,
		k8sMgr:         k8sMgr,
	}
}

// ServiceInfo Service資訊
type ServiceInfo struct {
	Name                string                `json:"name"`
	Namespace           string                `json:"namespace"`
	Type                string                `json:"type"`
	ClusterIP           string                `json:"clusterIP"`
	ExternalIPs         []string              `json:"externalIPs,omitempty"`
	Ports               []ServicePort         `json:"ports"`
	Selector            map[string]string     `json:"selector"`
	SessionAffinity     string                `json:"sessionAffinity"`
	LoadBalancerIP      string                `json:"loadBalancerIP,omitempty"`
	LoadBalancerIngress []LoadBalancerIngress `json:"loadBalancerIngress,omitempty"`
	ExternalName        string                `json:"externalName,omitempty"`
	CreatedAt           time.Time             `json:"createdAt"`
	Labels              map[string]string     `json:"labels"`
	Annotations         map[string]string     `json:"annotations"`
}

// ServicePort Service連接埠資訊
type ServicePort struct {
	Name       string `json:"name"`
	Protocol   string `json:"protocol"`
	Port       int32  `json:"port"`
	TargetPort string `json:"targetPort"`
	NodePort   int32  `json:"nodePort,omitempty"`
}

// LoadBalancerIngress 負載均衡器入口資訊
type LoadBalancerIngress struct {
	IP       string `json:"ip,omitempty"`
	Hostname string `json:"hostname,omitempty"`
}

// ListServices 獲取Service列表
func (h *ServiceHandler) ListServices(c *gin.Context) {
	clusterIDStr := c.Param("clusterID")
	clusterID, err := parseClusterID(clusterIDStr)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

	// 獲取查詢參數
	namespace := c.DefaultQuery("namespace", "")
	serviceType := c.DefaultQuery("type", "")
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

	// 獲取Services
	services, err := h.getServices(clientset, namespace)
	if err != nil {
		logger.Error("獲取Services失敗", "error", err, "clusterId", clusterID)
		response.InternalError(c, fmt.Sprintf("獲取Services失敗: %v", err))
		return
	}

	// 根據命名空間權限過濾
	if !nsInfo.HasAllAccess && namespace == "" {
		services = middleware.FilterResourcesByNamespace(c, services, func(s ServiceInfo) string {
			return s.Namespace
		})
	}

	// 過濾和搜尋
	filteredServices := h.filterServices(services, serviceType, search)

	// 排序
	sort.Slice(filteredServices, func(i, j int) bool {
		return filteredServices[i].CreatedAt.After(filteredServices[j].CreatedAt)
	})

	// 分頁
	total := len(filteredServices)
	start := (page - 1) * pageSize
	end := start + pageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}
	pagedServices := filteredServices[start:end]

	response.PagedList(c, pagedServices, int64(total), page, pageSize)
}

// GetService 獲取單個Service詳情
func (h *ServiceHandler) GetService(c *gin.Context) {
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

	// 獲取Service
	service, err := clientset.CoreV1().Services(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		logger.Error("獲取Service失敗", "error", err, "clusterId", clusterID, "namespace", namespace, "name", name)
		response.InternalError(c, fmt.Sprintf("獲取Service失敗: %v", err))
		return
	}

	serviceInfo := h.convertToServiceInfo(service)

	response.OK(c, serviceInfo)
}

// GetServiceYAML 獲取Service的YAML
func (h *ServiceHandler) GetServiceYAML(c *gin.Context) {
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

	// 獲取Service
	service, err := clientset.CoreV1().Services(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		logger.Error("獲取Service失敗", "error", err, "clusterId", clusterID, "namespace", namespace, "name", name)
		response.InternalError(c, fmt.Sprintf("獲取Service失敗: %v", err))
		return
	}

	// 設定 apiVersion 和 kind（API 返回的物件不包含這些欄位）
	cleanSvc := service.DeepCopy()
	cleanSvc.APIVersion = "v1"
	cleanSvc.Kind = "Service"
	cleanSvc.ManagedFields = nil // 移除 managedFields 簡化 YAML

	// 轉換為YAML
	yamlData, err := yaml.Marshal(cleanSvc)
	if err != nil {
		logger.Error("轉換YAML失敗", "error", err)
		response.InternalError(c, fmt.Sprintf("轉換YAML失敗: %v", err))
		return
	}

	response.OK(c, gin.H{"yaml": string(yamlData)})
}

// DeleteService 刪除Service
func (h *ServiceHandler) DeleteService(c *gin.Context) {
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

	// 刪除Service
	err = clientset.CoreV1().Services(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
	if err != nil {
		logger.Error("刪除Service失敗", "error", err, "clusterId", clusterID, "namespace", namespace, "name", name)
		response.InternalError(c, fmt.Sprintf("刪除Service失敗: %v", err))
		return
	}

	logger.Info("Service刪除成功", "clusterId", clusterID, "namespace", namespace, "name", name)
	response.NoContent(c)
}

// GetServiceEndpoints 獲取Service的Endpoints
func (h *ServiceHandler) GetServiceEndpoints(c *gin.Context) {
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

	// 獲取Endpoints
	endpoints, err := clientset.CoreV1().Endpoints(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		logger.Error("獲取Endpoints失敗", "error", err, "clusterId", clusterID, "namespace", namespace, "name", name)
		response.InternalError(c, fmt.Sprintf("獲取Endpoints失敗: %v", err))
		return
	}

	// 轉換Endpoints資訊
	endpointInfo := h.convertEndpointsInfo(endpoints)

	response.OK(c, endpointInfo)
}

// 輔助函式

// getServices 獲取Services
func (h *ServiceHandler) getServices(clientset kubernetes.Interface, namespace string) ([]ServiceInfo, error) {
	var serviceList *corev1.ServiceList
	var err error

	if namespace == "" || namespace == "_all_" {
		serviceList, err = clientset.CoreV1().Services("").List(context.Background(), metav1.ListOptions{})
	} else {
		serviceList, err = clientset.CoreV1().Services(namespace).List(context.Background(), metav1.ListOptions{})
	}

	if err != nil {
		return nil, err
	}

	services := make([]ServiceInfo, 0, len(serviceList.Items))
	for _, svc := range serviceList.Items {
		services = append(services, h.convertToServiceInfo(&svc))
	}

	return services, nil
}

// convertToServiceInfo 轉換為ServiceInfo
func (h *ServiceHandler) convertToServiceInfo(svc *corev1.Service) ServiceInfo {
	ports := make([]ServicePort, 0, len(svc.Spec.Ports))
	for _, p := range svc.Spec.Ports {
		ports = append(ports, ServicePort{
			Name:       p.Name,
			Protocol:   string(p.Protocol),
			Port:       p.Port,
			TargetPort: h.getTargetPortString(p.TargetPort),
			NodePort:   p.NodePort,
		})
	}

	lbIngress := make([]LoadBalancerIngress, 0, len(svc.Status.LoadBalancer.Ingress))
	for _, ing := range svc.Status.LoadBalancer.Ingress {
		lbIngress = append(lbIngress, LoadBalancerIngress{
			IP:       ing.IP,
			Hostname: ing.Hostname,
		})
	}

	return ServiceInfo{
		Name:                svc.Name,
		Namespace:           svc.Namespace,
		Type:                string(svc.Spec.Type),
		ClusterIP:           svc.Spec.ClusterIP,
		ExternalIPs:         svc.Spec.ExternalIPs,
		Ports:               ports,
		Selector:            svc.Spec.Selector,
		SessionAffinity:     string(svc.Spec.SessionAffinity),
		LoadBalancerIP:      svc.Spec.LoadBalancerIP,
		LoadBalancerIngress: lbIngress,
		ExternalName:        svc.Spec.ExternalName,
		CreatedAt:           svc.CreationTimestamp.Time,
		Labels:              svc.Labels,
		Annotations:         svc.Annotations,
	}
}

// getTargetPortString 獲取目標連接埠字串
func (h *ServiceHandler) getTargetPortString(targetPort intstr.IntOrString) string {
	if targetPort.Type == intstr.Int {
		return strconv.Itoa(int(targetPort.IntVal))
	}
	return targetPort.StrVal
}

// filterServices 過濾Services
func (h *ServiceHandler) filterServices(services []ServiceInfo, serviceType, search string) []ServiceInfo {
	filtered := make([]ServiceInfo, 0)
	for _, svc := range services {
		// 型別過濾
		if serviceType != "" && svc.Type != serviceType {
			continue
		}

		// 搜尋過濾
		if search != "" {
			searchLower := strings.ToLower(search)
			if !strings.Contains(strings.ToLower(svc.Name), searchLower) &&
				!strings.Contains(strings.ToLower(svc.Namespace), searchLower) &&
				!strings.Contains(strings.ToLower(svc.ClusterIP), searchLower) {
				continue
			}
		}

		filtered = append(filtered, svc)
	}
	return filtered
}

// convertEndpointsInfo 轉換Endpoints資訊
func (h *ServiceHandler) convertEndpointsInfo(endpoints *corev1.Endpoints) gin.H {
	subsets := make([]gin.H, 0, len(endpoints.Subsets))
	for _, subset := range endpoints.Subsets {
		addresses := make([]gin.H, 0, len(subset.Addresses))
		for _, addr := range subset.Addresses {
			addresses = append(addresses, gin.H{
				"ip":       addr.IP,
				"nodeName": addr.NodeName,
				"targetRef": func() gin.H {
					if addr.TargetRef != nil {
						return gin.H{
							"kind":      addr.TargetRef.Kind,
							"name":      addr.TargetRef.Name,
							"namespace": addr.TargetRef.Namespace,
						}
					}
					return nil
				}(),
			})
		}

		ports := make([]gin.H, 0, len(subset.Ports))
		for _, port := range subset.Ports {
			ports = append(ports, gin.H{
				"name":     port.Name,
				"port":     port.Port,
				"protocol": string(port.Protocol),
			})
		}

		subsets = append(subsets, gin.H{
			"addresses": addresses,
			"ports":     ports,
		})
	}

	return gin.H{
		"name":      endpoints.Name,
		"namespace": endpoints.Namespace,
		"subsets":   subsets,
	}
}

// CreateServiceRequest 建立Service請求
type CreateServiceRequest struct {
	Namespace string           `json:"namespace" binding:"required"`
	YAML      string           `json:"yaml,omitempty"`     // YAML方式建立
	FormData  *ServiceFormData `json:"formData,omitempty"` // 表單方式建立
}

// ServiceFormData Service表單資料
type ServiceFormData struct {
	Name            string            `json:"name" binding:"required"`
	Type            string            `json:"type" binding:"required"` // ClusterIP, NodePort, LoadBalancer
	Selector        map[string]string `json:"selector"`
	Ports           []ServicePortForm `json:"ports" binding:"required"`
	SessionAffinity string            `json:"sessionAffinity"`
	ExternalIPs     []string          `json:"externalIPs,omitempty"`
	LoadBalancerIP  string            `json:"loadBalancerIP,omitempty"`
	ExternalName    string            `json:"externalName,omitempty"`
	Labels          map[string]string `json:"labels,omitempty"`
	Annotations     map[string]string `json:"annotations,omitempty"`
}

// ServicePortForm Service連接埠表單
type ServicePortForm struct {
	Name       string `json:"name"`
	Protocol   string `json:"protocol"` // TCP, UDP, SCTP
	Port       int32  `json:"port" binding:"required"`
	TargetPort string `json:"targetPort"` // 可以是數字或字串
	NodePort   int32  `json:"nodePort,omitempty"`
}

// CreateService 建立Service
func (h *ServiceHandler) CreateService(c *gin.Context) {
	clusterIDStr := c.Param("clusterID")
	clusterID, err := parseClusterID(clusterIDStr)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

	var req CreateServiceRequest
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

	var service *corev1.Service

	// 根據建立方式選擇處理邏輯
	if req.YAML != "" {
		// YAML方式建立
		service, err = h.createServiceFromYAML(clientset, req.Namespace, req.YAML)
	} else if req.FormData != nil {
		// 表單方式建立
		service, err = h.createServiceFromForm(clientset, req.Namespace, req.FormData)
	} else {
		response.BadRequest(c, "必須提供YAML或表單資料")
		return
	}

	if err != nil {
		logger.Error("建立Service失敗", "error", err, "clusterId", clusterID)
		response.InternalError(c, fmt.Sprintf("建立Service失敗: %v", err))
		return
	}

	logger.Info("Service建立成功", "clusterId", clusterID, "namespace", service.Namespace, "name", service.Name)
	response.OK(c, h.convertToServiceInfo(service))
}

// UpdateService 更新Service
func (h *ServiceHandler) UpdateService(c *gin.Context) {
	clusterIDStr := c.Param("clusterID")
	clusterID, err := parseClusterID(clusterIDStr)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}
	namespace := c.Param("namespace")
	name := c.Param("name")

	var req CreateServiceRequest
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

	var service *corev1.Service

	// 根據更新方式選擇處理邏輯
	if req.YAML != "" {
		// YAML方式更新
		service, err = h.updateServiceFromYAML(clientset, namespace, name, req.YAML)
	} else if req.FormData != nil {
		// 表單方式更新
		service, err = h.updateServiceFromForm(clientset, namespace, name, req.FormData)
	} else {
		response.BadRequest(c, "必須提供YAML或表單資料")
		return
	}

	if err != nil {
		logger.Error("更新Service失敗", "error", err, "clusterId", clusterID)
		response.InternalError(c, fmt.Sprintf("更新Service失敗: %v", err))
		return
	}

	logger.Info("Service更新成功", "clusterId", clusterID, "namespace", service.Namespace, "name", service.Name)
	response.OK(c, h.convertToServiceInfo(service))
}

// createServiceFromYAML 從YAML建立Service
func (h *ServiceHandler) createServiceFromYAML(clientset kubernetes.Interface, namespace, yamlContent string) (*corev1.Service, error) {
	var service corev1.Service
	if err := yaml.Unmarshal([]byte(yamlContent), &service); err != nil {
		return nil, fmt.Errorf("解析YAML失敗: %w", err)
	}

	// 確保namespace正確
	if service.Namespace == "" {
		service.Namespace = namespace
	}

	createdService, err := clientset.CoreV1().Services(service.Namespace).Create(context.Background(), &service, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	return createdService, nil
}

// createServiceFromForm 從表單建立Service
func (h *ServiceHandler) createServiceFromForm(clientset kubernetes.Interface, namespace string, formData *ServiceFormData) (*corev1.Service, error) {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        formData.Name,
			Namespace:   namespace,
			Labels:      formData.Labels,
			Annotations: formData.Annotations,
		},
		Spec: corev1.ServiceSpec{
			Type:            corev1.ServiceType(formData.Type),
			Selector:        formData.Selector,
			SessionAffinity: corev1.ServiceAffinity(formData.SessionAffinity),
		},
	}

	// 新增連接埠
	ports := make([]corev1.ServicePort, 0, len(formData.Ports))
	for _, p := range formData.Ports {
		port := corev1.ServicePort{
			Name:     p.Name,
			Protocol: corev1.Protocol(p.Protocol),
			Port:     p.Port,
		}

		// 處理TargetPort
		if portNum, err := strconv.Atoi(p.TargetPort); err == nil {
			port.TargetPort = intstr.FromInt(portNum)
		} else {
			port.TargetPort = intstr.FromString(p.TargetPort)
		}

		// NodePort型別時設定NodePort
		if formData.Type == "NodePort" || formData.Type == "LoadBalancer" {
			port.NodePort = p.NodePort
		}

		ports = append(ports, port)
	}
	service.Spec.Ports = ports

	// 其他可選配置
	if len(formData.ExternalIPs) > 0 {
		service.Spec.ExternalIPs = formData.ExternalIPs
	}
	if formData.LoadBalancerIP != "" {
		service.Spec.LoadBalancerIP = formData.LoadBalancerIP
	}
	if formData.ExternalName != "" {
		service.Spec.ExternalName = formData.ExternalName
	}

	createdService, err := clientset.CoreV1().Services(namespace).Create(context.Background(), service, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	return createdService, nil
}

// updateServiceFromYAML 從YAML更新Service
func (h *ServiceHandler) updateServiceFromYAML(clientset kubernetes.Interface, namespace, name, yamlContent string) (*corev1.Service, error) {
	var service corev1.Service
	if err := yaml.Unmarshal([]byte(yamlContent), &service); err != nil {
		return nil, fmt.Errorf("解析YAML失敗: %w", err)
	}

	// 獲取現有Service
	existingService, err := clientset.CoreV1().Services(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	// 保留ResourceVersion
	service.ResourceVersion = existingService.ResourceVersion
	service.Namespace = namespace
	service.Name = name

	updatedService, err := clientset.CoreV1().Services(namespace).Update(context.Background(), &service, metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}

	return updatedService, nil
}

// updateServiceFromForm 從表單更新Service
func (h *ServiceHandler) updateServiceFromForm(clientset kubernetes.Interface, namespace, name string, formData *ServiceFormData) (*corev1.Service, error) {
	// 獲取現有Service
	existingService, err := clientset.CoreV1().Services(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	// 更新Spec
	existingService.Spec.Type = corev1.ServiceType(formData.Type)
	existingService.Spec.Selector = formData.Selector
	existingService.Spec.SessionAffinity = corev1.ServiceAffinity(formData.SessionAffinity)

	// 更新連接埠
	ports := make([]corev1.ServicePort, 0, len(formData.Ports))
	for _, p := range formData.Ports {
		port := corev1.ServicePort{
			Name:     p.Name,
			Protocol: corev1.Protocol(p.Protocol),
			Port:     p.Port,
		}

		// 處理TargetPort
		if portNum, err := strconv.Atoi(p.TargetPort); err == nil {
			port.TargetPort = intstr.FromInt(portNum)
		} else {
			port.TargetPort = intstr.FromString(p.TargetPort)
		}

		// NodePort型別時設定NodePort
		if formData.Type == "NodePort" || formData.Type == "LoadBalancer" {
			port.NodePort = p.NodePort
		}

		ports = append(ports, port)
	}
	existingService.Spec.Ports = ports

	// 更新其他可選配置
	existingService.Spec.ExternalIPs = formData.ExternalIPs
	existingService.Spec.LoadBalancerIP = formData.LoadBalancerIP
	existingService.Spec.ExternalName = formData.ExternalName

	// 更新Labels和Annotations
	if formData.Labels != nil {
		existingService.Labels = formData.Labels
	}
	if formData.Annotations != nil {
		existingService.Annotations = formData.Annotations
	}

	updatedService, err := clientset.CoreV1().Services(namespace).Update(context.Background(), existingService, metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}

	return updatedService, nil
}

// GetServiceNamespaces 獲取Service所在的命名空間列表
func (h *ServiceHandler) GetServiceNamespaces(c *gin.Context) {
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

	// 獲取所有Services
	serviceList, err := clientset.CoreV1().Services("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		logger.Error("獲取Service列表失敗", "cluster", cluster.Name, "error", err)
		response.InternalError(c, fmt.Sprintf("獲取Service列表失敗: %v", err))
		return
	}

	// 統計每個命名空間的Service數量
	nsMap := make(map[string]int)
	for _, svc := range serviceList.Items {
		nsMap[svc.Namespace]++
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
