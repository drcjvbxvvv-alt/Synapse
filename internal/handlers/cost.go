package handlers

import (
	"fmt"
	"strconv"
	"time"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/pkg/logger"

	"github.com/gin-gonic/gin"
)

// CostHandler 資源成本分析處理器
type CostHandler struct {
	svc *services.CostService
}

// NewCostHandler 建立處理器
func NewCostHandler(svc *services.CostService) *CostHandler {
	return &CostHandler{svc: svc}
}

// currentMonth 取得本月字串（YYYY-MM）
func currentMonth() string {
	return time.Now().UTC().Format("2006-01")
}

// GetConfig 取得定價設定
func (h *CostHandler) GetConfig(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}
	cfg, err := h.svc.GetConfig(clusterID)
	if err != nil {
		logger.Error("取得成本設定失敗", "error", err)
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, cfg)
}

// UpdateConfig 更新定價設定
func (h *CostHandler) UpdateConfig(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}
	var cfg models.CostConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		response.BadRequest(c, "參數錯誤: "+err.Error())
		return
	}
	cfg.ClusterID = clusterID
	if err := h.svc.UpsertConfig(&cfg); err != nil {
		logger.Error("更新成本設定失敗", "error", err)
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, cfg)
}

// GetOverview 取得本月成本總覽
func (h *CostHandler) GetOverview(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}
	month := c.DefaultQuery("month", currentMonth())
	overview, err := h.svc.GetOverview(clusterID, month)
	if err != nil {
		logger.Error("取得成本總覽失敗", "error", err)
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, overview)
}

// GetNamespaceCosts 取得命名空間成本排行
func (h *CostHandler) GetNamespaceCosts(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}
	month := c.DefaultQuery("month", currentMonth())
	items, err := h.svc.GetNamespaceCosts(clusterID, month)
	if err != nil {
		logger.Error("取得命名空間成本失敗", "error", err)
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, items)
}

// GetWorkloadCosts 取得工作負載成本明細
func (h *CostHandler) GetWorkloadCosts(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}
	month := c.DefaultQuery("month", currentMonth())
	namespace := c.DefaultQuery("namespace", "")
	page := parsePage(c)
	pageSize := parsePageSize(c, 20)

	items, total, err := h.svc.GetWorkloadCosts(clusterID, month, namespace, page, pageSize)
	if err != nil {
		logger.Error("取得工作負載成本失敗", "error", err)
		response.InternalError(c, err.Error())
		return
	}
	response.PagedList(c, items, total, page, pageSize)
}

// GetTrend 取得月度成本趨勢
func (h *CostHandler) GetTrend(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}
	months, _ := strconv.Atoi(c.DefaultQuery("months", "6"))
	if months < 1 || months > 12 {
		months = 6
	}
	points, err := h.svc.GetTrend(clusterID, months)
	if err != nil {
		logger.Error("取得成本趨勢失敗", "error", err)
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, points)
}

// GetWaste 取得資源浪費報告
func (h *CostHandler) GetWaste(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}
	items, err := h.svc.GetWaste(clusterID)
	if err != nil {
		logger.Error("取得浪費報告失敗", "error", err)
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, items)
}

// ExportCSV 匯出 CSV 報表
func (h *CostHandler) ExportCSV(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}
	month := c.DefaultQuery("month", currentMonth())
	data, err := h.svc.ExportCSV(clusterID, month)
	if err != nil {
		logger.Error("匯出 CSV 失敗", "error", err)
		response.InternalError(c, err.Error())
		return
	}
	filename := fmt.Sprintf("cost-%s.csv", month)
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Data(200, "text/csv", data)
}
