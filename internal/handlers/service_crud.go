package handlers

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/pkg/logger"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/yaml"
)

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

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	var service *corev1.Service

	// 根據建立方式選擇處理邏輯
	if req.YAML != "" {
		// YAML方式建立
		service, err = h.createServiceFromYAML(ctx, clientset, req.Namespace, req.YAML)
	} else if req.FormData != nil {
		// 表單方式建立
		service, err = h.createServiceFromForm(ctx, clientset, req.Namespace, req.FormData)
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

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	var service *corev1.Service

	// 根據更新方式選擇處理邏輯
	if req.YAML != "" {
		// YAML方式更新
		service, err = h.updateServiceFromYAML(ctx, clientset, namespace, name, req.YAML)
	} else if req.FormData != nil {
		// 表單方式更新
		service, err = h.updateServiceFromForm(ctx, clientset, namespace, name, req.FormData)
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
func (h *ServiceHandler) createServiceFromYAML(ctx context.Context, clientset kubernetes.Interface, namespace, yamlContent string) (*corev1.Service, error) {
	var service corev1.Service
	if err := yaml.Unmarshal([]byte(yamlContent), &service); err != nil {
		return nil, fmt.Errorf("解析YAML失敗: %w", err)
	}

	// 確保namespace正確
	if service.Namespace == "" {
		service.Namespace = namespace
	}

	createdService, err := clientset.CoreV1().Services(service.Namespace).Create(ctx, &service, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	return createdService, nil
}

// createServiceFromForm 從表單建立Service
func (h *ServiceHandler) createServiceFromForm(ctx context.Context, clientset kubernetes.Interface, namespace string, formData *ServiceFormData) (*corev1.Service, error) {
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

	createdService, err := clientset.CoreV1().Services(namespace).Create(ctx, service, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	return createdService, nil
}

// updateServiceFromYAML 從YAML更新Service
func (h *ServiceHandler) updateServiceFromYAML(ctx context.Context, clientset kubernetes.Interface, namespace, name, yamlContent string) (*corev1.Service, error) {
	var service corev1.Service
	if err := yaml.Unmarshal([]byte(yamlContent), &service); err != nil {
		return nil, fmt.Errorf("解析YAML失敗: %w", err)
	}

	// 獲取現有Service
	existingService, err := clientset.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	// 保留ResourceVersion
	service.ResourceVersion = existingService.ResourceVersion
	service.Namespace = namespace
	service.Name = name

	updatedService, err := clientset.CoreV1().Services(namespace).Update(ctx, &service, metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}

	return updatedService, nil
}

// updateServiceFromForm 從表單更新Service
func (h *ServiceHandler) updateServiceFromForm(ctx context.Context, clientset kubernetes.Interface, namespace, name string, formData *ServiceFormData) (*corev1.Service, error) {
	// 獲取現有Service
	existingService, err := clientset.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
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

	updatedService, err := clientset.CoreV1().Services(namespace).Update(ctx, existingService, metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}

	return updatedService, nil
}
