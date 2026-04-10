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

// ApplyYAML 應用Deployment YAML
func (h *DeploymentHandler) ApplyYAML(c *gin.Context) {
	clusterId := c.Param("clusterID")

	var req YAMLApplyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "參數錯誤: "+err.Error())
		return
	}

	logger.Info("應用Deployment YAML: cluster=%s, dryRun=%v", clusterId, req.DryRun)

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

	// 解析YAML
	var objMap map[string]interface{}
	if err := yaml.Unmarshal([]byte(req.YAML), &objMap); err != nil {
		response.BadRequest(c, "YAML格式錯誤: "+err.Error())
		return
	}

	// 驗證必要欄位
	if objMap["apiVersion"] == nil || objMap["kind"] == nil {
		response.BadRequest(c, "YAML缺少必要欄位: apiVersion 或 kind")
		return
	}

	kind := objMap["kind"].(string)
	if kind != "Deployment" {
		response.BadRequest(c, "YAML型別錯誤，期望Deployment，實際為: "+kind)
		return
	}

	// 獲取metadata
	metadata, ok := objMap["metadata"].(map[string]interface{})
	if !ok {
		response.BadRequest(c, "YAML缺少 metadata 欄位")
		return
	}

	namespace, _ := metadata["namespace"].(string)
	if namespace == "" {
		namespace = "default"
	}

	// 應用YAML
	result, err := h.applyYAML(ctx, k8sClient, req.YAML, namespace, req.DryRun)
	if err != nil {
		response.InternalError(c, "YAML應用失敗: "+err.Error())
		return
	}

	response.OK(c, result)
}

// applyYAML 輔助方法：應用YAML
func (h *DeploymentHandler) applyYAML(ctx context.Context, k8sClient *services.K8sClient, yamlContent string, namespace string, dryRun bool) (interface{}, error) {
	// 建立解碼器
	decode := serializer.NewCodecFactory(scheme.Scheme).UniversalDeserializer().Decode
	obj, _, err := decode([]byte(yamlContent), nil, nil)
	if err != nil {
		return nil, fmt.Errorf("解析YAML失敗: %w", err)
	}

	deployment, ok := obj.(*appsv1.Deployment)
	if !ok {
		return nil, fmt.Errorf("無法轉換為Deployment型別")
	}

	clientset := k8sClient.GetClientset()
	var dryRunOpt []string
	if dryRun {
		dryRunOpt = []string{metav1.DryRunAll}
	}

	// 嘗試獲取現有資源
	existing, err := clientset.AppsV1().Deployments(deployment.Namespace).Get(ctx, deployment.Name, metav1.GetOptions{})
	if err == nil {
		// 資源存在，執行更新
		deployment.ResourceVersion = existing.ResourceVersion
		result, err := clientset.AppsV1().Deployments(deployment.Namespace).Update(ctx, deployment, metav1.UpdateOptions{DryRun: dryRunOpt})
		if err != nil {
			return nil, err
		}
		return result, nil
	}

	// 資源不存在，執行建立
	result, err := clientset.AppsV1().Deployments(deployment.Namespace).Create(ctx, deployment, metav1.CreateOptions{DryRun: dryRunOpt})
	if err != nil {
		return nil, err
	}
	return result, nil
}
