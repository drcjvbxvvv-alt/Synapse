package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/shaia/Synapse/internal/k8s"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/pkg/logger"

	"github.com/gin-gonic/gin"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HPAHandler HPA 處理器
type HPAHandler struct {
	clusterService *services.ClusterService
	k8sMgr         *k8s.ClusterInformerManager
}

// NewHPAHandler 建立 HPA 處理器
func NewHPAHandler(clusterService *services.ClusterService, k8sMgr *k8s.ClusterInformerManager) *HPAHandler {
	return &HPAHandler{
		clusterService: clusterService,
		k8sMgr:         k8sMgr,
	}
}

// HPARequest 建立/更新 HPA 請求
type HPARequest struct {
	Name                 string `json:"name" binding:"required"`
	Namespace            string `json:"namespace" binding:"required"`
	TargetKind           string `json:"targetKind" binding:"required"` // Deployment / StatefulSet / Rollout
	TargetName           string `json:"targetName" binding:"required"`
	MinReplicas          int32  `json:"minReplicas" binding:"required,min=1"`
	MaxReplicas          int32  `json:"maxReplicas" binding:"required,min=1"`
	CPUTargetUtilization *int32 `json:"cpuTargetUtilization"` // nil = 不設定 CPU 指標
	MemTargetUtilization *int32 `json:"memTargetUtilization"` // nil = 不設定 Memory 指標
}

// hpaToInfo 將 K8s HPA 物件轉換為回應格式
func hpaToInfo(hpa *autoscalingv2.HorizontalPodAutoscaler) map[string]interface{} {
	metrics := make([]map[string]interface{}, 0, len(hpa.Spec.Metrics))
	for _, metric := range hpa.Spec.Metrics {
		m := map[string]interface{}{"type": string(metric.Type)}
		if metric.Resource != nil {
			m["resource"] = map[string]interface{}{
				"name":   metric.Resource.Name,
				"target": metric.Resource.Target,
			}
		}
		metrics = append(metrics, m)
	}

	conditions := make([]map[string]interface{}, 0, len(hpa.Status.Conditions))
	for _, c := range hpa.Status.Conditions {
		conditions = append(conditions, map[string]interface{}{
			"type":    string(c.Type),
			"status":  string(c.Status),
			"reason":  c.Reason,
			"message": c.Message,
		})
	}

	minReplicas := int32(1)
	if hpa.Spec.MinReplicas != nil {
		minReplicas = *hpa.Spec.MinReplicas
	}

	return map[string]interface{}{
		"name":            hpa.Name,
		"namespace":       hpa.Namespace,
		"targetKind":      hpa.Spec.ScaleTargetRef.Kind,
		"targetName":      hpa.Spec.ScaleTargetRef.Name,
		"minReplicas":     minReplicas,
		"maxReplicas":     hpa.Spec.MaxReplicas,
		"currentReplicas": hpa.Status.CurrentReplicas,
		"desiredReplicas": hpa.Status.DesiredReplicas,
		"metrics":         metrics,
		"conditions":      conditions,
		"createdAt":       hpa.CreationTimestamp.Time,
	}
}

// buildHPASpec 依據請求建立 HPA Spec
func buildHPASpec(req *HPARequest) autoscalingv2.HorizontalPodAutoscalerSpec {
	apiVersion := "apps/v1"
	if req.TargetKind == "Rollout" {
		apiVersion = "argoproj.io/v1alpha1"
	}

	spec := autoscalingv2.HorizontalPodAutoscalerSpec{
		ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
			APIVersion: apiVersion,
			Kind:       req.TargetKind,
			Name:       req.TargetName,
		},
		MinReplicas: &req.MinReplicas,
		MaxReplicas: req.MaxReplicas,
		Metrics:     []autoscalingv2.MetricSpec{},
	}

	if req.CPUTargetUtilization != nil {
		spec.Metrics = append(spec.Metrics, autoscalingv2.MetricSpec{
			Type: autoscalingv2.ResourceMetricSourceType,
			Resource: &autoscalingv2.ResourceMetricSource{
				Name: corev1.ResourceCPU,
				Target: autoscalingv2.MetricTarget{
					Type:               autoscalingv2.UtilizationMetricType,
					AverageUtilization: req.CPUTargetUtilization,
				},
			},
		})
	}

	if req.MemTargetUtilization != nil {
		spec.Metrics = append(spec.Metrics, autoscalingv2.MetricSpec{
			Type: autoscalingv2.ResourceMetricSourceType,
			Resource: &autoscalingv2.ResourceMetricSource{
				Name: corev1.ResourceMemory,
				Target: autoscalingv2.MetricTarget{
					Type:               autoscalingv2.UtilizationMetricType,
					AverageUtilization: req.MemTargetUtilization,
				},
			},
		})
	}

	return spec
}

// ListHPA 列出命名空間下的所有 HPA
func (h *HPAHandler) ListHPA(c *gin.Context) {
	clusterIDStr := c.Param("clusterID")
	namespace := c.Query("namespace")

	clusterID, err := parseClusterID(clusterIDStr)
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, "取得 K8s 客戶端失敗: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	hpaList, err := k8sClient.GetClientset().AutoscalingV2().HorizontalPodAutoscalers(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		response.InternalError(c, "取得 HPA 列表失敗: "+err.Error())
		return
	}

	items := make([]map[string]interface{}, 0, len(hpaList.Items))
	for i := range hpaList.Items {
		items = append(items, hpaToInfo(&hpaList.Items[i]))
	}
	response.OK(c, map[string]interface{}{"items": items, "total": len(items)})
}

// CreateHPA 建立 HPA
func (h *HPAHandler) CreateHPA(c *gin.Context) {
	clusterIDStr := c.Param("clusterID")

	var req HPARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "請求參數錯誤: "+err.Error())
		return
	}
	if req.MaxReplicas < req.MinReplicas {
		response.BadRequest(c, "maxReplicas 不得小於 minReplicas")
		return
	}

	clusterID, err := parseClusterID(clusterIDStr)
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, "取得 K8s 客戶端失敗: "+err.Error())
		return
	}

	hpa := &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Name,
			Namespace: req.Namespace,
		},
		Spec: buildHPASpec(&req),
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	created, err := k8sClient.GetClientset().AutoscalingV2().HorizontalPodAutoscalers(req.Namespace).Create(ctx, hpa, metav1.CreateOptions{})
	if err != nil {
		if errors.IsAlreadyExists(err) {
			response.BadRequest(c, fmt.Sprintf("HPA '%s' 已存在", req.Name))
			return
		}
		response.InternalError(c, "建立 HPA 失敗: "+err.Error())
		return
	}

	logger.Info("建立 HPA", "cluster", clusterIDStr, "namespace", req.Namespace, "name", req.Name)
	response.OK(c, hpaToInfo(created))
}

// UpdateHPA 更新 HPA
func (h *HPAHandler) UpdateHPA(c *gin.Context) {
	clusterIDStr := c.Param("clusterID")
	namespace := c.Param("namespace")
	name := c.Param("name")

	var req HPARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "請求參數錯誤: "+err.Error())
		return
	}
	if req.MaxReplicas < req.MinReplicas {
		response.BadRequest(c, "maxReplicas 不得小於 minReplicas")
		return
	}

	clusterID, err := parseClusterID(clusterIDStr)
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, "取得 K8s 客戶端失敗: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	clientset := k8sClient.GetClientset()

	// 取得現有 HPA 以保留 ResourceVersion
	existing, err := clientset.AutoscalingV2().HorizontalPodAutoscalers(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			response.NotFound(c, "HPA 不存在")
			return
		}
		response.InternalError(c, "取得 HPA 失敗: "+err.Error())
		return
	}

	existing.Spec = buildHPASpec(&req)

	updated, err := clientset.AutoscalingV2().HorizontalPodAutoscalers(namespace).Update(ctx, existing, metav1.UpdateOptions{})
	if err != nil {
		response.InternalError(c, "更新 HPA 失敗: "+err.Error())
		return
	}

	logger.Info("更新 HPA", "cluster", clusterIDStr, "namespace", namespace, "name", name)
	response.OK(c, hpaToInfo(updated))
}

// DeleteHPA 刪除 HPA
func (h *HPAHandler) DeleteHPA(c *gin.Context) {
	clusterIDStr := c.Param("clusterID")
	namespace := c.Param("namespace")
	name := c.Param("name")

	clusterID, err := parseClusterID(clusterIDStr)
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, "取得 K8s 客戶端失敗: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	err = k8sClient.GetClientset().AutoscalingV2().HorizontalPodAutoscalers(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			response.NotFound(c, "HPA 不存在")
			return
		}
		response.InternalError(c, "刪除 HPA 失敗: "+err.Error())
		return
	}

	logger.Info("刪除 HPA", "cluster", clusterIDStr, "namespace", namespace, "name", name)
	response.OK(c, map[string]string{"message": "HPA 刪除成功"})
}
