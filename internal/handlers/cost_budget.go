package handlers

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/shaia/Synapse/internal/k8s"
	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/pkg/logger"
)

// CostBudgetHandler 命名空間預算處理器
type CostBudgetHandler struct {
	clusterService *services.ClusterService
	k8sMgr         *k8s.ClusterInformerManager
	budgetSvc      *services.CostBudgetService
	costSvc        *services.CostService
}

// NewCostBudgetHandler 建立預算處理器
func NewCostBudgetHandler(
	clusterSvc *services.ClusterService,
	k8sMgr *k8s.ClusterInformerManager,
	budgetSvc *services.CostBudgetService,
	costSvc *services.CostService,
) *CostBudgetHandler {
	return &CostBudgetHandler{
		clusterService: clusterSvc,
		k8sMgr:         k8sMgr,
		budgetSvc:      budgetSvc,
		costSvc:        costSvc,
	}
}

// ─── DTOs ──────────────────────────────────────────────────────────────────

// UpsertBudgetRequest 新增/更新預算請求
type UpsertBudgetRequest struct {
	CPUCoresLimit    float64 `json:"cpu_cores_limit"`
	MemoryGiBLimit   float64 `json:"memory_gib_limit"`
	MonthlyCostLimit float64 `json:"monthly_cost_limit"`
	AlertThreshold   float64 `json:"alert_threshold"`
	Enabled          *bool   `json:"enabled"` // pointer to distinguish zero-value from absent
}

// ─── Handlers ──────────────────────────────────────────────────────────────

// ListBudgets 列出叢集所有預算
// GET /clusters/:clusterID/cost/budgets
func (h *CostBudgetHandler) ListBudgets(c *gin.Context) {
	// Step 1: Parse params
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}

	// Step 2: (no K8s client needed for DB-only query)
	// Step 3: Context with timeout
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	// Step 4: Call service
	budgets, err := h.budgetSvc.ListBudgets(ctx, clusterID)
	if err != nil {
		logger.Error("列出預算失敗", "error", err, "cluster_id", clusterID)
		response.InternalError(c, "列出預算失敗: "+err.Error())
		return
	}

	// Step 5: Response
	response.OK(c, budgets)
}

// UpsertBudget 新增或更新預算
// PUT /clusters/:clusterID/cost/budgets/:namespace
func (h *CostBudgetHandler) UpsertBudget(c *gin.Context) {
	// Step 1: Parse params
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}
	namespace := c.Param("namespace")
	if namespace == "" {
		response.BadRequest(c, "namespace 為必填")
		return
	}

	var req UpsertBudgetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "參數錯誤: "+err.Error())
		return
	}

	// Step 2: (no K8s client needed)
	// Step 3: Context with timeout
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	budget := &models.NamespaceBudget{
		ClusterID:        clusterID,
		Namespace:        namespace,
		CPUCoresLimit:    req.CPUCoresLimit,
		MemoryGiBLimit:   req.MemoryGiBLimit,
		MonthlyCostLimit: req.MonthlyCostLimit,
		AlertThreshold:   req.AlertThreshold,
		Enabled:          enabled,
	}

	logger.Info("更新命名空間預算",
		"cluster_id", clusterID,
		"namespace", namespace,
	)

	// Step 4: Call service
	if err := h.budgetSvc.UpsertBudget(ctx, budget); err != nil {
		logger.Error("更新預算失敗", "error", err, "cluster_id", clusterID, "namespace", namespace)
		response.InternalError(c, "更新預算失敗: "+err.Error())
		return
	}

	// Step 5: Response
	response.OK(c, budget)
}

// DeleteBudget 刪除預算
// DELETE /clusters/:clusterID/cost/budgets/:namespace
func (h *CostBudgetHandler) DeleteBudget(c *gin.Context) {
	// Step 1: Parse params
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}
	namespace := c.Param("namespace")
	if namespace == "" {
		response.BadRequest(c, "namespace 為必填")
		return
	}

	// Step 2: (no K8s client needed)
	// Step 3: Context with timeout
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	logger.Info("刪除命名空間預算",
		"cluster_id", clusterID,
		"namespace", namespace,
	)

	// Step 4: Call service
	if err := h.budgetSvc.DeleteBudget(ctx, clusterID, namespace); err != nil {
		logger.Error("刪除預算失敗", "error", err, "cluster_id", clusterID, "namespace", namespace)
		response.NotFound(c, "預算不存在: "+err.Error())
		return
	}

	// Step 5: Response
	response.OK(c, gin.H{"message": "deleted"})
}

// CheckBudgets 檢查預算狀態
// GET /clusters/:clusterID/cost/budgets/check
func (h *CostBudgetHandler) CheckBudgets(c *gin.Context) {
	// Step 1: Parse params
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}

	// Step 2: Resolve cluster + K8s client
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}

	// Ensure informer is synced
	if err := h.k8sMgr.EnsureSync(context.Background(), cluster, 5*time.Second); err != nil {
		response.InternalError(c, "informer 同步失敗: "+err.Error())
		return
	}

	// Step 3: Context with timeout
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	// Step 4: Aggregate namespace-level usage from pod informer cache
	pods, err := h.k8sMgr.PodsLister(cluster.ID).List(labels.Everything())
	if err != nil {
		logger.Error("讀取 Pod 快取失敗", "error", err, "cluster_id", clusterID)
		response.InternalError(c, "讀取 Pod 快取失敗: "+err.Error())
		return
	}

	// Get cost config for cost estimation
	costCfg, err := h.costSvc.GetConfig(clusterID)
	if err != nil {
		logger.Error("取得成本設定失敗", "error", err, "cluster_id", clusterID)
		response.InternalError(c, "取得成本設定失敗: "+err.Error())
		return
	}

	// Aggregate per-namespace resource usage
	usageMap := make(map[string]*services.NamespaceUsage)
	for _, pod := range pods {
		if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
			continue
		}
		ns := pod.Namespace
		if usageMap[ns] == nil {
			usageMap[ns] = &services.NamespaceUsage{}
		}
		for _, container := range pod.Spec.Containers {
			cpuMilli := float64(container.Resources.Requests.Cpu().MilliValue())
			memMiB := float64(container.Resources.Requests.Memory().Value()) / 1024 / 1024
			usageMap[ns].CPUMillicores += cpuMilli
			usageMap[ns].MemoryMiB += memMiB
		}
	}

	// Estimate monthly cost per namespace: (cpuCores * cpuPrice + memGiB * memPrice) * 730 hours
	const hoursPerMonth = 730
	for _, u := range usageMap {
		cpuCores := u.CPUMillicores / 1000.0
		memGiB := u.MemoryMiB / 1024.0
		u.EstCost = (cpuCores*costCfg.CpuPricePerCore + memGiB*costCfg.MemPricePerGiB) * hoursPerMonth
	}

	// Check budgets against current usage
	results, err := h.budgetSvc.CheckBudgets(ctx, clusterID, usageMap)
	if err != nil {
		logger.Error("檢查預算失敗", "error", err, "cluster_id", clusterID)
		response.InternalError(c, "檢查預算失敗: "+err.Error())
		return
	}

	// Step 5: Response
	response.OK(c, results)
}
