package handlers

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"k8s.io/client-go/kubernetes"

	"github.com/shaia/Synapse/internal/k8s"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/pkg/logger"
)

// MultiClusterHandler 多叢集工作流程處理器
type MultiClusterHandler struct {
	syncPolicySvc  *services.SyncPolicyService
	clusterService *services.ClusterService
	k8sMgr         *k8s.ClusterInformerManager
}

// NewMultiClusterHandler 建立多叢集處理器
func NewMultiClusterHandler(syncPolicySvc *services.SyncPolicyService, clusterService *services.ClusterService, k8sMgr *k8s.ClusterInformerManager) *MultiClusterHandler {
	return &MultiClusterHandler{syncPolicySvc: syncPolicySvc, clusterService: clusterService, k8sMgr: k8sMgr}
}

// getClientByID 根據叢集 ID 取得 K8s 客戶端
func (h *MultiClusterHandler) getClientByID(id uint) (kubernetes.Interface, error) {
	cluster, err := h.clusterService.GetCluster(id)
	if err != nil {
		return nil, fmt.Errorf("叢集 %d 不存在: %v", id, err)
	}
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		return nil, fmt.Errorf("取得叢集 %d 客戶端失敗: %v", id, err)
	}
	return k8sClient.GetClientset(), nil
}

// MigrateCheck POST /multicluster/migrate/check — 遷移預檢
func (h *MultiClusterHandler) MigrateCheck(c *gin.Context) {
	var req MigrateCheckRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	srcClient, err := h.getClientByID(req.SourceClusterID)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	dstClient, err := h.getClientByID(req.TargetClusterID)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	ctx := context.Background()
	result := MigrateCheckResult{}

	// 1. 取得工作負載資源請求量
	cpuReq, memReq, cmNames, secNames, err := h.getWorkloadResources(ctx, srcClient, req)
	if err != nil {
		response.Error(c, http.StatusUnprocessableEntity, "WORKLOAD_NOT_FOUND", err.Error())
		return
	}
	result.WorkloadCPUReq = cpuReq
	result.WorkloadMemReq = memReq
	result.ConfigMapCount = len(cmNames)
	result.SecretCount = len(secNames)

	// 2. 計算目標叢集可用資源
	freeCPU, freeMem, err := h.calcFreeResources(ctx, dstClient)
	if err != nil {
		logger.Warn("無法取得目標叢集資源", "error", err)
		// 不阻斷，降級為可行
		result.Feasible = true
		result.Message = "無法取得目標叢集資源用量，請確認目標叢集有足夠資源"
		response.OK(c, result)
		return
	}
	result.TargetFreeCPU = freeCPU
	result.TargetFreeMem = freeMem

	if cpuReq > freeCPU || memReq > freeMem {
		result.Feasible = false
		result.Message = fmt.Sprintf("目標叢集資源不足（需要 CPU %.0fm / MEM %.0fMiB，可用 CPU %.0fm / MEM %.0fMiB）",
			cpuReq, memReq, freeCPU, freeMem)
	} else {
		result.Feasible = true
		result.Message = "資源充足，可以執行遷移"
	}
	response.OK(c, result)
}

// Migrate POST /multicluster/migrate — 執行遷移
func (h *MultiClusterHandler) Migrate(c *gin.Context) {
	var req MigrateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	srcClient, err := h.getClientByID(req.SourceClusterID)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	dstClient, err := h.getClientByID(req.TargetClusterID)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	ctx := context.Background()
	result := MigrateResult{}

	// 1. 確保目標命名空間存在
	if err := ensureNamespace(ctx, dstClient, req.TargetNamespace); err != nil {
		response.InternalError(c, fmt.Sprintf("建立命名空間失敗: %v", err))
		return
	}

	// 2. 同步 ConfigMap
	if req.SyncConfigMaps {
		synced, err := h.syncConfigMaps(ctx, srcClient, dstClient, req.SourceNamespace, req.TargetNamespace, req.WorkloadName)
		if err != nil {
			logger.Warn("同步 ConfigMap 部分失敗", "error", err)
		}
		result.ConfigMapsSynced = synced
	}

	// 3. 同步 Secret
	if req.SyncSecrets {
		synced, err := h.syncSecrets(ctx, srcClient, dstClient, req.SourceNamespace, req.TargetNamespace, req.WorkloadName)
		if err != nil {
			logger.Warn("同步 Secret 部分失敗", "error", err)
		}
		result.SecretsSynced = synced
	}

	// 4. 遷移工作負載
	if err := h.migrateWorkload(ctx, srcClient, dstClient, req); err != nil {
		response.InternalError(c, fmt.Sprintf("遷移工作負載失敗: %v", err))
		return
	}
	result.WorkloadCreated = true
	result.Success = true
	result.Message = fmt.Sprintf("%s/%s 已成功遷移至叢集 %d 的命名空間 %s",
		req.WorkloadKind, req.WorkloadName, req.TargetClusterID, req.TargetNamespace)

	response.OK(c, result)
}
