package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/pkg/logger"

	rollouts "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	"github.com/gin-gonic/gin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	sigsyaml "sigs.k8s.io/yaml"
)

// ApplyYAML 應用Rollout YAML
func (h *RolloutHandler) ApplyYAML(c *gin.Context) {
	clusterId := c.Param("clusterID")

	var req YAMLApplyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "參數錯誤: "+err.Error())
		return
	}

	logger.Info("應用Rollout YAML: cluster=%s, dryRun=%v", clusterId, req.DryRun)

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
	if kind != "Rollout" {
		response.BadRequest(c, "YAML型別錯誤，期望Rollout，實際為: "+kind)
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

	// 檢查 Argo Rollouts CRD 是否已安裝
	if h.k8sMgr.RolloutsLister(cluster.ID) == nil {
		response.BadRequest(c, "叢集未安裝 Argo Rollouts，請先安裝後再建立 Rollout 資源")
		return
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
func (h *RolloutHandler) applyYAML(ctx context.Context, k8sClient *services.K8sClient, yamlContent string, namespace string, dryRun bool) (interface{}, error) {
	// 使用 sigsyaml 直接解析為 Rollout 結構（比 runtime serializer 更適合 CRD 類型）
	rollout := &rollouts.Rollout{}
	if err := sigsyaml.Unmarshal([]byte(yamlContent), rollout); err != nil {
		return nil, fmt.Errorf("解析YAML失敗: %w", err)
	}
	if rollout.Name == "" || rollout.Kind == "" {
		return nil, fmt.Errorf("YAML缺少必要欄位 (name 或 kind)")
	}

	rolloutClient, err := k8sClient.GetRolloutClient()
	if err != nil {
		return nil, fmt.Errorf("獲取Rollout客戶端失敗: %w", err)
	}

	var dryRunOpt []string
	if dryRun {
		dryRunOpt = []string{metav1.DryRunAll}
	}

	// 嘗試獲取現有資源
	existing, err := rolloutClient.ArgoprojV1alpha1().Rollouts(rollout.Namespace).Get(ctx, rollout.Name, metav1.GetOptions{})
	if err == nil {
		// 資源存在，執行更新
		rollout.ResourceVersion = existing.ResourceVersion
		result, err := rolloutClient.ArgoprojV1alpha1().Rollouts(rollout.Namespace).Update(ctx, rollout, metav1.UpdateOptions{DryRun: dryRunOpt})
		if err != nil {
			return nil, err
		}
		return result, nil
	}

	// 資源不存在，執行建立
	result, err := rolloutClient.ArgoprojV1alpha1().Rollouts(rollout.Namespace).Create(ctx, rollout, metav1.CreateOptions{DryRun: dryRunOpt})
	if err != nil {
		return nil, err
	}
	return result, nil
}
