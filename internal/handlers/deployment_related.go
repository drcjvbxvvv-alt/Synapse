package handlers

import (
	"context"
	"time"

	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/pkg/logger"

	"github.com/gin-gonic/gin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// GetDeploymentPods 獲取Deployment關聯的Pods
func (h *DeploymentHandler) GetDeploymentPods(c *gin.Context) {
	clusterId := c.Param("clusterID")
	namespace := c.Param("namespace")
	name := c.Param("name")

	logger.Info("獲取Deployment Pods: cluster=%s, namespace=%s, name=%s", clusterId, namespace, name)

	// 獲取叢集資訊
	clusterID, err := parseClusterID(clusterId)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}

	// 獲取快取的 K8s 客戶端
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, "獲取K8s客戶端失敗: "+err.Error())
		return
	}

	clientset := k8sClient.GetClientset()
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	// 獲取Deployment
	deployment, err := clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		response.NotFound(c, "Deployment不存在")
		return
	}

	// 使用selector查詢Pods
	selector := labels.SelectorFromSet(deployment.Spec.Selector.MatchLabels)
	podList, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: selector.String(),
	})
	if err != nil {
		response.InternalError(c, "獲取Pod列表失敗: "+err.Error())
		return
	}

	// 轉換Pod資訊
	pods := make([]map[string]interface{}, 0, len(podList.Items))
	for _, pod := range podList.Items {
		podInfo := map[string]interface{}{
			"name":         pod.Name,
			"namespace":    pod.Namespace,
			"phase":        string(pod.Status.Phase),
			"nodeName":     pod.Spec.NodeName,
			"nodeIP":       pod.Status.HostIP,
			"podIP":        pod.Status.PodIP,
			"restartCount": 0,
			"createdAt":    pod.CreationTimestamp.Time,
		}

		// 計算重啟次數和提取資源限制
		var totalRestarts int32
		var cpuRequest, cpuLimit, memoryRequest, memoryLimit string
		for _, container := range pod.Spec.Containers {
			// 資源限制
			if container.Resources.Requests != nil {
				if cpu, ok := container.Resources.Requests["cpu"]; ok {
					cpuRequest = cpu.String()
				}
				if mem, ok := container.Resources.Requests["memory"]; ok {
					memoryRequest = mem.String()
				}
			}
			if container.Resources.Limits != nil {
				if cpu, ok := container.Resources.Limits["cpu"]; ok {
					cpuLimit = cpu.String()
				}
				if mem, ok := container.Resources.Limits["memory"]; ok {
					memoryLimit = mem.String()
				}
			}
		}

		// 統計重啟次數
		for _, containerStatus := range pod.Status.ContainerStatuses {
			totalRestarts += containerStatus.RestartCount
		}

		podInfo["restartCount"] = totalRestarts
		podInfo["cpuRequest"] = cpuRequest
		podInfo["cpuLimit"] = cpuLimit
		podInfo["memoryRequest"] = memoryRequest
		podInfo["memoryLimit"] = memoryLimit

		pods = append(pods, podInfo)
	}

	response.List(c, pods, int64(len(pods)))
}

// GetDeploymentServices 獲取Deployment關聯的Services
func (h *DeploymentHandler) GetDeploymentServices(c *gin.Context) {
	clusterId := c.Param("clusterID")
	namespace := c.Param("namespace")
	name := c.Param("name")

	logger.Info("獲取Deployment Services: cluster=%s, namespace=%s, name=%s", clusterId, namespace, name)

	// 獲取叢集資訊
	clusterID, err := parseClusterID(clusterId)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}

	// 獲取快取的 K8s 客戶端
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, "獲取K8s客戶端失敗: "+err.Error())
		return
	}

	clientset := k8sClient.GetClientset()
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	// 獲取Deployment
	deployment, err := clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		response.NotFound(c, "Deployment不存在")
		return
	}

	// 獲取Services
	serviceList, err := clientset.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		response.InternalError(c, "獲取Service列表失敗: "+err.Error())
		return
	}

	// 篩選匹配的Services
	deploymentLabels := deployment.Spec.Selector.MatchLabels
	matchedServices := make([]map[string]interface{}, 0)
	for _, svc := range serviceList.Items {
		// 檢查Service的selector是否匹配Deployment的labels
		matches := true
		for key, value := range svc.Spec.Selector {
			if deploymentLabels[key] != value {
				matches = false
				break
			}
		}

		if matches {
			ports := make([]map[string]interface{}, 0, len(svc.Spec.Ports))
			for _, port := range svc.Spec.Ports {
				ports = append(ports, map[string]interface{}{
					"name":       port.Name,
					"protocol":   port.Protocol,
					"port":       port.Port,
					"targetPort": port.TargetPort.String(),
					"nodePort":   port.NodePort,
				})
			}

			serviceInfo := map[string]interface{}{
				"name":        svc.Name,
				"namespace":   svc.Namespace,
				"type":        string(svc.Spec.Type),
				"clusterIP":   svc.Spec.ClusterIP,
				"externalIPs": svc.Spec.ExternalIPs,
				"ports":       ports,
				"selector":    svc.Spec.Selector,
				"createdAt":   svc.CreationTimestamp.Time,
			}
			matchedServices = append(matchedServices, serviceInfo)
		}
	}

	response.List(c, matchedServices, int64(len(matchedServices)))
}

// GetDeploymentIngresses 獲取Deployment關聯的Ingresses
func (h *DeploymentHandler) GetDeploymentIngresses(c *gin.Context) {
	clusterId := c.Param("clusterID")
	namespace := c.Param("namespace")
	name := c.Param("name")

	logger.Info("獲取Deployment Ingresses: cluster=%s, namespace=%s, name=%s", clusterId, namespace, name)

	// 獲取叢集資訊
	clusterID, err := parseClusterID(clusterId)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}

	// 獲取快取的 K8s 客戶端
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, "獲取K8s客戶端失敗: "+err.Error())
		return
	}

	clientset := k8sClient.GetClientset()
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	// 獲取Ingresses
	ingressList, err := clientset.NetworkingV1().Ingresses(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		response.InternalError(c, "獲取Ingress列表失敗: "+err.Error())
		return
	}

	// 轉換Ingress資訊
	ingresses := make([]map[string]interface{}, 0, len(ingressList.Items))
	for _, ingress := range ingressList.Items {
		rules := make([]map[string]interface{}, 0, len(ingress.Spec.Rules))
		for _, rule := range ingress.Spec.Rules {
			paths := make([]map[string]interface{}, 0)
			if rule.HTTP != nil {
				for _, path := range rule.HTTP.Paths {
					paths = append(paths, map[string]interface{}{
						"path":     path.Path,
						"pathType": string(*path.PathType),
						"backend": map[string]interface{}{
							"serviceName": path.Backend.Service.Name,
							"servicePort": path.Backend.Service.Port.Number,
						},
					})
				}
			}
			rules = append(rules, map[string]interface{}{
				"host":  rule.Host,
				"paths": paths,
			})
		}

		ingressInfo := map[string]interface{}{
			"name":             ingress.Name,
			"namespace":        ingress.Namespace,
			"ingressClassName": ingress.Spec.IngressClassName,
			"rules":            rules,
			"createdAt":        ingress.CreationTimestamp.Time,
		}
		ingresses = append(ingresses, ingressInfo)
	}

	response.List(c, ingresses, int64(len(ingresses)))
}
