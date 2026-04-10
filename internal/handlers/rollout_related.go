package handlers

import (
	"context"
	"time"

	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/pkg/logger"

	"github.com/gin-gonic/gin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetRolloutPods 獲取Rollout關聯的Pods
func (h *RolloutHandler) GetRolloutPods(c *gin.Context) {
	clusterId := c.Param("clusterID")
	namespace := c.Param("namespace")
	name := c.Param("name")

	logger.Info("獲取Rollout關聯的Pods: %s/%s/%s", clusterId, namespace, name)

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

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	// 獲取Rollout
	rolloutClient, err := k8sClient.GetRolloutClient()
	if err != nil {
		response.InternalError(c, "獲取Rollout客戶端失敗: "+err.Error())
		return
	}

	rollout, err := rolloutClient.ArgoprojV1alpha1().Rollouts(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		response.NotFound(c, "Rollout不存在: "+err.Error())
		return
	}

	// 獲取關聯的Pods
	clientset := k8sClient.GetClientset()
	podList, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: metav1.FormatLabelSelector(rollout.Spec.Selector),
	})
	if err != nil {
		response.InternalError(c, "獲取Pods失敗: "+err.Error())
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

// GetRolloutServices 獲取Rollout關聯的Services
func (h *RolloutHandler) GetRolloutServices(c *gin.Context) {
	clusterId := c.Param("clusterID")
	namespace := c.Param("namespace")
	name := c.Param("name")

	logger.Info("獲取Rollout關聯的Services: %s/%s/%s", clusterId, namespace, name)

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

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	// 獲取Rollout
	rolloutClient, err := k8sClient.GetRolloutClient()
	if err != nil {
		response.InternalError(c, "獲取Rollout客戶端失敗: "+err.Error())
		return
	}

	rollout, err := rolloutClient.ArgoprojV1alpha1().Rollouts(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		response.NotFound(c, "Rollout不存在: "+err.Error())
		return
	}

	// 獲取所有Services
	clientset := k8sClient.GetClientset()
	serviceList, err := clientset.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		response.InternalError(c, "獲取Services失敗: "+err.Error())
		return
	}

	// 篩選匹配的Services
	rolloutLabels := rollout.Spec.Selector.MatchLabels
	matchedServices := make([]map[string]interface{}, 0)
	for _, svc := range serviceList.Items {
		// 檢查Service的selector是否匹配Rollout的labels
		matches := true
		for key, value := range svc.Spec.Selector {
			if rolloutLabels[key] != value {
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

// GetRolloutIngresses 獲取Rollout關聯的Ingresses
func (h *RolloutHandler) GetRolloutIngresses(c *gin.Context) {
	clusterId := c.Param("clusterID")
	namespace := c.Param("namespace")
	name := c.Param("name")

	logger.Info("獲取Rollout關聯的Ingresses: %s/%s/%s", clusterId, namespace, name)

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

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	// 獲取Rollout物件
	rolloutClient, err := k8sClient.GetRolloutClient()
	if err != nil {
		response.InternalError(c, "獲取Rollout客戶端失敗: "+err.Error())
		return
	}

	rollout, err := rolloutClient.ArgoprojV1alpha1().Rollouts(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		response.NotFound(c, "Rollout不存在: "+err.Error())
		return
	}

	// 收集Rollout關聯的Service名稱
	relatedServices := make(map[string]bool)

	// 從Canary策略獲取關聯的Service
	if rollout.Spec.Strategy.Canary != nil {
		if rollout.Spec.Strategy.Canary.StableService != "" {
			relatedServices[rollout.Spec.Strategy.Canary.StableService] = true
		}
		if rollout.Spec.Strategy.Canary.CanaryService != "" {
			relatedServices[rollout.Spec.Strategy.Canary.CanaryService] = true
		}
		// 檢查TrafficRouting中的Ingress配置
		if rollout.Spec.Strategy.Canary.TrafficRouting != nil {
			if rollout.Spec.Strategy.Canary.TrafficRouting.Nginx != nil {
				if rollout.Spec.Strategy.Canary.TrafficRouting.Nginx.StableIngress != "" {
					// 記錄Nginx Ingress名稱，後續直接匹配
					relatedServices["__nginx_ingress__:"+rollout.Spec.Strategy.Canary.TrafficRouting.Nginx.StableIngress] = true
				}
			}
			if rollout.Spec.Strategy.Canary.TrafficRouting.ALB != nil {
				if rollout.Spec.Strategy.Canary.TrafficRouting.ALB.Ingress != "" {
					relatedServices["__alb_ingress__:"+rollout.Spec.Strategy.Canary.TrafficRouting.ALB.Ingress] = true
				}
			}
		}
	}

	// 從BlueGreen策略獲取關聯的Service
	if rollout.Spec.Strategy.BlueGreen != nil {
		if rollout.Spec.Strategy.BlueGreen.ActiveService != "" {
			relatedServices[rollout.Spec.Strategy.BlueGreen.ActiveService] = true
		}
		if rollout.Spec.Strategy.BlueGreen.PreviewService != "" {
			relatedServices[rollout.Spec.Strategy.BlueGreen.PreviewService] = true
		}
	}

	// 同時透過Selector匹配獲取關聯的Services
	clientset := k8sClient.GetClientset()
	serviceList, err := clientset.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	if err == nil {
		rolloutLabels := rollout.Spec.Selector.MatchLabels
		for _, svc := range serviceList.Items {
			matches := true
			for key, value := range svc.Spec.Selector {
				if rolloutLabels[key] != value {
					matches = false
					break
				}
			}
			if matches {
				relatedServices[svc.Name] = true
			}
		}
	}

	// 獲取Ingresses
	ingressList, err := clientset.NetworkingV1().Ingresses(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		response.InternalError(c, "獲取Ingresses失敗: "+err.Error())
		return
	}

	// 篩選與Rollout關聯的Ingresses
	matchedIngresses := make([]map[string]interface{}, 0)
	for _, ingress := range ingressList.Items {
		isRelated := false

		// 檢查是否是TrafficRouting直接配置的Ingress
		if relatedServices["__nginx_ingress__:"+ingress.Name] || relatedServices["__alb_ingress__:"+ingress.Name] {
			isRelated = true
		}

		// 檢查Ingress的backend是否指向關聯的Service
		if !isRelated {
			for _, rule := range ingress.Spec.Rules {
				if rule.HTTP != nil {
					for _, path := range rule.HTTP.Paths {
						if path.Backend.Service != nil && relatedServices[path.Backend.Service.Name] {
							isRelated = true
							break
						}
					}
				}
				if isRelated {
					break
				}
			}
		}

		// 檢查預設backend
		if !isRelated && ingress.Spec.DefaultBackend != nil && ingress.Spec.DefaultBackend.Service != nil {
			if relatedServices[ingress.Spec.DefaultBackend.Service.Name] {
				isRelated = true
			}
		}

		if isRelated {
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
			matchedIngresses = append(matchedIngresses, ingressInfo)
		}
	}

	response.List(c, matchedIngresses, int64(len(matchedIngresses)))
}
