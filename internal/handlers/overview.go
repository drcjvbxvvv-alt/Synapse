package handlers

import (
	"context"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/pkg/logger"
)

// OverviewHandler 總覽處理器
type OverviewHandler struct {
	overviewService *services.OverviewService
	permissionSvc   *services.PermissionService
}

// NewOverviewHandler 建立總覽處理器
func NewOverviewHandler(overviewSvc *services.OverviewService, permSvc *services.PermissionService) *OverviewHandler {
	return &OverviewHandler{
		overviewService: overviewSvc,
		permissionSvc:   permSvc,
	}
}

// filteredContext 在 context 中注入使用者可訪問的叢集過濾條件
func (h *OverviewHandler) filteredContext(c *gin.Context) context.Context {
	userID := c.GetUint("user_id")
	clusterIDs, isAll, err := h.permissionSvc.GetUserAccessibleClusterIDs(userID)
	if err != nil || isAll {
		return c.Request.Context()
	}
	return services.ContextWithClusterFilter(c.Request.Context(), clusterIDs)
}

// GetStats 獲取總覽統計資料
// @Summary 獲取總覽統計資料
// @Description 返回叢集、節點、Pod 的統計資料以及版本分佈
// @Tags Overview
// @Accept json
// @Produce json
// @Success 200 {object} services.OverviewStatsResponse
// @Router /api/v1/overview/stats [get]
func (h *OverviewHandler) GetStats(c *gin.Context) {
	logger.Info("獲取總覽統計資料")

	stats, err := h.overviewService.GetOverviewStats(h.filteredContext(c))
	if err != nil {
		logger.Error("獲取總覽統計資料失敗", "error", err)
		response.InternalError(c, "獲取統計資料失敗: "+err.Error())
		return
	}

	response.OK(c, stats)
}

// GetResourceUsage 獲取資源使用率
// @Summary 獲取資源使用率
// @Description 返回 CPU、記憶體、儲存的使用率
// @Tags Overview
// @Accept json
// @Produce json
// @Success 200 {object} services.ResourceUsageResponse
// @Router /api/v1/overview/resource-usage [get]
func (h *OverviewHandler) GetResourceUsage(c *gin.Context) {
	logger.Info("獲取資源使用率")

	usage, err := h.overviewService.GetResourceUsage(h.filteredContext(c))
	if err != nil {
		logger.Error("獲取資源使用率失敗", "error", err)
		response.InternalError(c, "獲取資源使用率失敗: "+err.Error())
		return
	}

	response.OK(c, usage)
}

// GetDistribution 獲取資源分佈
// @Summary 獲取資源分佈
// @Description 返回各叢集的 Pod、Node、CPU、記憶體分佈
// @Tags Overview
// @Accept json
// @Produce json
// @Success 200 {object} services.ResourceDistributionResponse
// @Router /api/v1/overview/distribution [get]
func (h *OverviewHandler) GetDistribution(c *gin.Context) {
	logger.Info("獲取資源分佈")

	distribution, err := h.overviewService.GetResourceDistribution(h.filteredContext(c))
	if err != nil {
		logger.Error("獲取資源分佈失敗", "error", err)
		response.InternalError(c, "獲取資源分佈失敗: "+err.Error())
		return
	}

	response.OK(c, distribution)
}

// GetTrends 獲取趨勢資料
// @Summary 獲取趨勢資料
// @Description 返回 Pod 和 Node 的歷史趨勢資料
// @Tags Overview
// @Accept json
// @Produce json
// @Param timeRange query string false "時間範圍: 7d, 30d" default(7d)
// @Param step query string false "步長: 1h, 6h, 1d" default(1h)
// @Success 200 {object} services.TrendResponse
// @Router /api/v1/overview/trends [get]
func (h *OverviewHandler) GetTrends(c *gin.Context) {
	startTime := time.Now()
	timeRange := c.DefaultQuery("timeRange", "7d")
	step := c.DefaultQuery("step", "")

	logger.Info("獲取趨勢資料開始", "timeRange", timeRange, "step", step)

	trends, err := h.overviewService.GetTrends(h.filteredContext(c), timeRange, step)

	elapsed := time.Since(startTime)
	logger.Info("獲取趨勢資料完成", "耗時", elapsed.String())

	if err != nil {
		logger.Error("獲取趨勢資料失敗", "error", err, "耗時", elapsed.String())
		response.InternalError(c, "獲取趨勢資料失敗: "+err.Error())
		return
	}

	response.OK(c, trends)
}

// GetAbnormalWorkloads 獲取異常工作負載
// @Summary 獲取異常工作負載
// @Description 返回異常的 Pod、Deployment、StatefulSet 列表
// @Tags Overview
// @Accept json
// @Produce json
// @Param limit query int false "返回數量限制" default(20)
// @Success 200 {array} services.AbnormalWorkload
// @Router /api/v1/overview/abnormal-workloads [get]
func (h *OverviewHandler) GetAbnormalWorkloads(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "20")
	limit, _ := strconv.Atoi(limitStr)

	logger.Info("獲取異常工作負載", "limit", limit)

	workloads, err := h.overviewService.GetAbnormalWorkloads(h.filteredContext(c), limit)
	if err != nil {
		logger.Error("獲取異常工作負載失敗", "error", err)
		response.InternalError(c, "獲取異常工作負載失敗: "+err.Error())
		return
	}

	response.OK(c, workloads)
}

// GetAlertStats 獲取全域性告警統計
// @Summary 獲取全域性告警統計
// @Description 返回所有叢集的告警彙總統計
// @Tags Overview
// @Accept json
// @Produce json
// @Success 200 {object} services.GlobalAlertStats
// @Router /api/v1/overview/alert-stats [get]
func (h *OverviewHandler) GetAlertStats(c *gin.Context) {
	logger.Info("獲取全域性告警統計")

	stats, err := h.overviewService.GetGlobalAlertStats(h.filteredContext(c))
	if err != nil {
		logger.Error("獲取全域性告警統計失敗", "error", err)
		response.InternalError(c, "獲取全域性告警統計失敗: "+err.Error())
		return
	}

	response.OK(c, stats)
}
