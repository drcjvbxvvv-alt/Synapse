package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/clay-wangzhi/KubePolaris/internal/config"
	"github.com/clay-wangzhi/KubePolaris/internal/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// AuditHandler 审计处理器
type AuditHandler struct {
	db           *gorm.DB
	cfg          *config.Config
	auditService *services.AuditService
}

// NewAuditHandler 创建审计处理器
func NewAuditHandler(db *gorm.DB, cfg *config.Config) *AuditHandler {
	return &AuditHandler{
		db:           db,
		cfg:          cfg,
		auditService: services.NewAuditService(db),
	}
}

// GetAuditLogs 获取审计日志
func (h *AuditHandler) GetAuditLogs(c *gin.Context) {
	// TODO: 实现审计日志查询逻辑
	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "获取成功",
		"data": gin.H{
			"items":    []interface{}{},
			"total":    0,
			"page":     1,
			"pageSize": 10,
		},
	})
}

// GetTerminalSessions 获取终端会话记录
func (h *AuditHandler) GetTerminalSessions(c *gin.Context) {
	// 解析查询参数
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	userIDStr := c.Query("userId")
	clusterIDStr := c.Query("clusterId")
	targetType := c.Query("targetType")
	status := c.Query("status")
	startTimeStr := c.Query("startTime")
	endTimeStr := c.Query("endTime")
	keyword := c.Query("keyword")

	req := &services.SessionListRequest{
		Page:       page,
		PageSize:   pageSize,
		TargetType: targetType,
		Status:     status,
		Keyword:    keyword,
	}

	if userIDStr != "" {
		if uid, err := strconv.ParseUint(userIDStr, 10, 32); err == nil {
			req.UserID = uint(uid)
		}
	}
	if clusterIDStr != "" {
		if cid, err := strconv.ParseUint(clusterIDStr, 10, 32); err == nil {
			req.ClusterID = uint(cid)
		}
	}
	if startTimeStr != "" {
		if t, err := time.Parse(time.RFC3339, startTimeStr); err == nil {
			req.StartTime = &t
		}
	}
	if endTimeStr != "" {
		if t, err := time.Parse(time.RFC3339, endTimeStr); err == nil {
			req.EndTime = &t
		}
	}

	resp, err := h.auditService.GetSessions(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取会话列表失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "获取成功",
		"data":    resp,
	})
}

// GetTerminalSession 获取终端会话详情
func (h *AuditHandler) GetTerminalSession(c *gin.Context) {
	sessionIDStr := c.Param("sessionId")
	sessionID, err := strconv.ParseUint(sessionIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的会话ID",
		})
		return
	}

	session, err := h.auditService.GetSessionDetail(uint(sessionID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "会话不存在",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "获取成功",
		"data":    session,
	})
}

// GetTerminalCommands 获取终端命令记录
func (h *AuditHandler) GetTerminalCommands(c *gin.Context) {
	sessionIDStr := c.Param("sessionId")
	sessionID, err := strconv.ParseUint(sessionIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的会话ID",
		})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "100"))

	resp, err := h.auditService.GetSessionCommands(uint(sessionID), page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取命令记录失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "获取成功",
		"data":    resp,
	})
}

// GetTerminalStats 获取终端会话统计
func (h *AuditHandler) GetTerminalStats(c *gin.Context) {
	stats, err := h.auditService.GetSessionStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取统计信息失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "获取成功",
		"data":    stats,
	})
}
