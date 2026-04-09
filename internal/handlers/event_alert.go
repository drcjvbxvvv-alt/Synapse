package handlers

import (
	"strconv"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/pkg/logger"

	"github.com/gin-gonic/gin"
)

// EventAlertHandler Event 告警規則處理器
type EventAlertHandler struct {
	svc *services.EventAlertService
}

// NewEventAlertHandler 建立 Event 告警處理器
func NewEventAlertHandler(svc *services.EventAlertService) *EventAlertHandler {
	return &EventAlertHandler{svc: svc}
}

// ListRules 取得告警規則列表
func (h *EventAlertHandler) ListRules(c *gin.Context) {
	clusterIDStr := c.Param("clusterID")
	clusterID, err := parseClusterID(clusterIDStr)
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}

	page := parsePage(c)
	pageSize := parsePageSize(c, 20)

	rules, total, err := h.svc.ListRules(clusterID, page, pageSize)
	if err != nil {
		logger.Error("取得告警規則失敗", "error", err)
		response.InternalError(c, err.Error())
		return
	}
	response.PagedList(c, rules, total, page, pageSize)
}

// CreateRule 建立告警規則
func (h *EventAlertHandler) CreateRule(c *gin.Context) {
	clusterIDStr := c.Param("clusterID")
	clusterID, err := parseClusterID(clusterIDStr)
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}

	var rule models.EventAlertRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		response.BadRequest(c, "參數錯誤: "+err.Error())
		return
	}
	rule.ClusterID = clusterID

	if err := h.svc.CreateRule(&rule); err != nil {
		logger.Error("建立告警規則失敗", "error", err)
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, rule)
}

// UpdateRule 更新告警規則
func (h *EventAlertHandler) UpdateRule(c *gin.Context) {
	clusterIDStr := c.Param("clusterID")
	clusterID, err := parseClusterID(clusterIDStr)
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}

	ruleIDStr := c.Param("ruleID")
	ruleID, err := strconv.ParseUint(ruleIDStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "無效的規則 ID")
		return
	}

	var rule models.EventAlertRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		response.BadRequest(c, "參數錯誤: "+err.Error())
		return
	}
	rule.ID = uint(ruleID)
	rule.ClusterID = clusterID

	if err := h.svc.UpdateRule(&rule); err != nil {
		logger.Error("更新告警規則失敗", "error", err)
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, rule)
}

// DeleteRule 刪除告警規則
func (h *EventAlertHandler) DeleteRule(c *gin.Context) {
	ruleIDStr := c.Param("ruleID")
	ruleID, err := strconv.ParseUint(ruleIDStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "無效的規則 ID")
		return
	}

	if err := h.svc.DeleteRule(uint(ruleID)); err != nil {
		logger.Error("刪除告警規則失敗", "error", err)
		response.InternalError(c, err.Error())
		return
	}
	response.NoContent(c)
}

// ToggleRule 啟用/停用規則
func (h *EventAlertHandler) ToggleRule(c *gin.Context) {
	ruleIDStr := c.Param("ruleID")
	ruleID, err := strconv.ParseUint(ruleIDStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "無效的規則 ID")
		return
	}

	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "參數錯誤: "+err.Error())
		return
	}

	if err := h.svc.ToggleRule(uint(ruleID), req.Enabled); err != nil {
		logger.Error("切換告警規則狀態失敗", "error", err)
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, gin.H{"enabled": req.Enabled})
}

// ListHistory 取得告警歷史
func (h *EventAlertHandler) ListHistory(c *gin.Context) {
	clusterIDStr := c.Param("clusterID")
	clusterID, err := parseClusterID(clusterIDStr)
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}

	page := parsePage(c)
	pageSize := parsePageSize(c, 20)

	items, total, err := h.svc.ListHistory(clusterID, page, pageSize)
	if err != nil {
		logger.Error("取得告警歷史失敗", "error", err)
		response.InternalError(c, err.Error())
		return
	}
	response.PagedList(c, items, total, page, pageSize)
}
