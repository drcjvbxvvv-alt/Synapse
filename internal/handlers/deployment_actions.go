package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/pkg/logger"

	"github.com/gin-gonic/gin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetDeploymentHPA 獲取Deployment的HPA
func (h *DeploymentHandler) GetDeploymentHPA(c *gin.Context) {
	clusterId := c.Param("clusterID")
	namespace := c.Param("namespace")
	name := c.Param("name")

	logger.Info("獲取Deployment HPA: cluster=%s, namespace=%s, name=%s", clusterId, namespace, name)

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

	// 獲取HPA列表
	hpaList, err := clientset.AutoscalingV2().HorizontalPodAutoscalers(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		response.InternalError(c, "獲取HPA列表失敗: "+err.Error())
		return
	}

	// 查詢匹配的HPA
	for _, hpa := range hpaList.Items {
		if hpa.Spec.ScaleTargetRef.Kind == "Deployment" && hpa.Spec.ScaleTargetRef.Name == name {
			metrics := make([]map[string]interface{}, 0, len(hpa.Spec.Metrics))
			for _, metric := range hpa.Spec.Metrics {
				metricInfo := map[string]interface{}{
					"type": string(metric.Type),
				}
				if metric.Resource != nil {
					metricInfo["resource"] = map[string]interface{}{
						"name":   metric.Resource.Name,
						"target": metric.Resource.Target,
					}
				}
				metrics = append(metrics, metricInfo)
			}

			conditions := make([]map[string]interface{}, 0, len(hpa.Status.Conditions))
			for _, condition := range hpa.Status.Conditions {
				conditions = append(conditions, map[string]interface{}{
					"type":    string(condition.Type),
					"status":  string(condition.Status),
					"reason":  condition.Reason,
					"message": condition.Message,
				})
			}

			hpaInfo := map[string]interface{}{
				"name":            hpa.Name,
				"namespace":       hpa.Namespace,
				"minReplicas":     *hpa.Spec.MinReplicas,
				"maxReplicas":     hpa.Spec.MaxReplicas,
				"currentReplicas": hpa.Status.CurrentReplicas,
				"desiredReplicas": hpa.Status.DesiredReplicas,
				"metrics":         metrics,
				"conditions":      conditions,
			}

			response.OK(c, hpaInfo)
			return
		}
	}

	// 未找到HPA
	response.NotFound(c, "未找到HPA")
}

// GetDeploymentReplicaSets 獲取Deployment的ReplicaSets
func (h *DeploymentHandler) GetDeploymentReplicaSets(c *gin.Context) {
	clusterId := c.Param("clusterID")
	namespace := c.Param("namespace")
	name := c.Param("name")

	logger.Info("獲取Deployment ReplicaSets: cluster=%s, namespace=%s, name=%s", clusterId, namespace, name)

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

	// 檢查Deployment是否存在
	_, err = clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		response.NotFound(c, "Deployment不存在")
		return
	}

	// 獲取ReplicaSets
	rsList, err := clientset.AppsV1().ReplicaSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		response.InternalError(c, "獲取ReplicaSet列表失敗: "+err.Error())
		return
	}

	// 篩選匹配的ReplicaSets
	matchedReplicaSets := make([]map[string]interface{}, 0)
	for _, rs := range rsList.Items {
		// 檢查owner reference
		isOwned := false
		for _, owner := range rs.OwnerReferences {
			if owner.Kind == "Deployment" && owner.Name == name {
				isOwned = true
				break
			}
		}

		if isOwned {
			// 提取映像列表
			images := make([]string, 0)
			for _, container := range rs.Spec.Template.Spec.Containers {
				images = append(images, container.Image)
			}

			// 獲取revision號
			revision := rs.Annotations["deployment.kubernetes.io/revision"]

			rsInfo := map[string]interface{}{
				"name":              rs.Name,
				"namespace":         rs.Namespace,
				"replicas":          *rs.Spec.Replicas,
				"readyReplicas":     rs.Status.ReadyReplicas,
				"availableReplicas": rs.Status.AvailableReplicas,
				"revision":          revision,
				"images":            images,
				"createdAt":         rs.CreationTimestamp.Time,
			}
			matchedReplicaSets = append(matchedReplicaSets, rsInfo)
		}
	}

	response.List(c, matchedReplicaSets, int64(len(matchedReplicaSets)))
}

// GetDeploymentEvents 獲取Deployment的Events
func (h *DeploymentHandler) GetDeploymentEvents(c *gin.Context) {
	clusterId := c.Param("clusterID")
	namespace := c.Param("namespace")
	name := c.Param("name")

	logger.Info("獲取Deployment Events: cluster=%s, namespace=%s, name=%s", clusterId, namespace, name)

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

	// 獲取Events
	eventList, err := clientset.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=Deployment", name),
	})
	if err != nil {
		response.InternalError(c, "獲取Events失敗: "+err.Error())
		return
	}

	// 轉換Event資訊
	events := make([]map[string]interface{}, 0, len(eventList.Items))
	for _, event := range eventList.Items {
		eventInfo := map[string]interface{}{
			"type":           event.Type,
			"reason":         event.Reason,
			"message":        event.Message,
			"source":         event.Source,
			"count":          event.Count,
			"firstTimestamp": event.FirstTimestamp.Time,
			"lastTimestamp":  event.LastTimestamp.Time,
			"involvedObject": map[string]interface{}{
				"kind":      event.InvolvedObject.Kind,
				"name":      event.InvolvedObject.Name,
				"namespace": event.InvolvedObject.Namespace,
			},
		}
		events = append(events, eventInfo)
	}

	response.List(c, events, int64(len(events)))
}
