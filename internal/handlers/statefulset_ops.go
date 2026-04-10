package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/pkg/logger"

	"github.com/gin-gonic/gin"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes/scheme"
)

// ScaleStatefulSet 擴縮容StatefulSet
func (h *StatefulSetHandler) ScaleStatefulSet(c *gin.Context) {
	clusterId := c.Param("clusterID")
	namespace := c.Param("namespace")
	name := c.Param("name")

	var req ScaleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "參數錯誤: "+err.Error())
		return
	}

	logger.Info("擴縮容StatefulSet: %s/%s/%s to %d", clusterId, namespace, name, req.Replicas)

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

	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, "獲取K8s客戶端失敗: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	clientset := k8sClient.GetClientset()
	scale, err := clientset.AppsV1().StatefulSets(namespace).GetScale(ctx, name, metav1.GetOptions{})
	if err != nil {
		response.InternalError(c, "獲取StatefulSet Scale失敗: "+err.Error())
		return
	}

	scale.Spec.Replicas = req.Replicas
	_, err = clientset.AppsV1().StatefulSets(namespace).UpdateScale(ctx, name, scale, metav1.UpdateOptions{})
	if err != nil {
		response.InternalError(c, "擴縮容失敗: "+err.Error())
		return
	}

	response.OK(c, gin.H{"message": "擴縮容成功"})
}

// ApplyYAML 應用StatefulSet YAML
func (h *StatefulSetHandler) ApplyYAML(c *gin.Context) {
	clusterId := c.Param("clusterID")

	var req YAMLApplyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "參數錯誤: "+err.Error())
		return
	}

	logger.Info("應用StatefulSet YAML: cluster=%s, dryRun=%v", clusterId, req.DryRun)

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

	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, "獲取K8s客戶端失敗: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	var objMap map[string]interface{}
	if err := yaml.Unmarshal([]byte(req.YAML), &objMap); err != nil {
		response.BadRequest(c, "YAML格式錯誤: "+err.Error())
		return
	}

	if objMap["apiVersion"] == nil || objMap["kind"] == nil {
		response.BadRequest(c, "YAML缺少必要欄位: apiVersion 或 kind")
		return
	}

	kind := objMap["kind"].(string)
	if kind != "StatefulSet" {
		response.BadRequest(c, "YAML型別錯誤，期望StatefulSet，實際為: "+kind)
		return
	}

	metadata, ok := objMap["metadata"].(map[string]interface{})
	if !ok {
		response.BadRequest(c, "YAML缺少 metadata 欄位")
		return
	}

	namespace, _ := metadata["namespace"].(string)
	if namespace == "" {
		namespace = "default"
	}

	result, err := h.applyYAML(ctx, k8sClient, req.YAML, namespace, req.DryRun)
	if err != nil {
		response.InternalError(c, "YAML應用失敗: "+err.Error())
		return
	}

	response.OK(c, result)
}

// DeleteStatefulSet 刪除StatefulSet
func (h *StatefulSetHandler) DeleteStatefulSet(c *gin.Context) {
	clusterId := c.Param("clusterID")
	namespace := c.Param("namespace")
	name := c.Param("name")

	logger.Info("刪除StatefulSet: %s/%s/%s", clusterId, namespace, name)

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

	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, "獲取K8s客戶端失敗: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	clientset := k8sClient.GetClientset()
	err = clientset.AppsV1().StatefulSets(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		response.InternalError(c, "刪除失敗: "+err.Error())
		return
	}

	response.OK(c, gin.H{"message": "刪除成功"})
}

// convertToStatefulSetInfo 輔助方法
func (h *StatefulSetHandler) convertToStatefulSetInfo(ss *appsv1.StatefulSet) StatefulSetInfo {
	status := "Running"
	if ss.Status.Replicas == 0 {
		status = "Stopped"
	} else if ss.Status.ReadyReplicas < ss.Status.Replicas {
		status = "Degraded"
	}

	var images []string
	for _, container := range ss.Spec.Template.Spec.Containers {
		images = append(images, container.Image)
	}

	return StatefulSetInfo{
		ID:              fmt.Sprintf("%s/%s", ss.Namespace, ss.Name),
		Name:            ss.Name,
		Namespace:       ss.Namespace,
		Type:            "StatefulSet",
		Status:          status,
		Replicas:        *ss.Spec.Replicas,
		ReadyReplicas:   ss.Status.ReadyReplicas,
		CurrentReplicas: ss.Status.CurrentReplicas,
		UpdatedReplicas: ss.Status.UpdatedReplicas,
		Labels:          ss.Labels,
		Annotations:     ss.Annotations,
		CreatedAt:       ss.CreationTimestamp.Time,
		Images:          images,
		Selector:        ss.Spec.Selector.MatchLabels,
		ServiceName:     ss.Spec.ServiceName,
	}
}

func (h *StatefulSetHandler) applyYAML(ctx context.Context, k8sClient *services.K8sClient, yamlContent string, namespace string, dryRun bool) (interface{}, error) {
	decode := serializer.NewCodecFactory(scheme.Scheme).UniversalDeserializer().Decode
	obj, _, err := decode([]byte(yamlContent), nil, nil)
	if err != nil {
		return nil, fmt.Errorf("解析YAML失敗: %w", err)
	}

	statefulSet, ok := obj.(*appsv1.StatefulSet)
	if !ok {
		return nil, fmt.Errorf("無法轉換為StatefulSet型別")
	}

	clientset := k8sClient.GetClientset()
	var dryRunOpt []string
	if dryRun {
		dryRunOpt = []string{metav1.DryRunAll}
	}

	existing, err := clientset.AppsV1().StatefulSets(statefulSet.Namespace).Get(ctx, statefulSet.Name, metav1.GetOptions{})
	if err == nil {
		statefulSet.ResourceVersion = existing.ResourceVersion
		result, err := clientset.AppsV1().StatefulSets(statefulSet.Namespace).Update(ctx, statefulSet, metav1.UpdateOptions{DryRun: dryRunOpt})
		if err != nil {
			return nil, err
		}
		return result, nil
	}

	result, err := clientset.AppsV1().StatefulSets(statefulSet.Namespace).Create(ctx, statefulSet, metav1.CreateOptions{DryRun: dryRunOpt})
	if err != nil {
		return nil, err
	}
	return result, nil
}
