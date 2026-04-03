package handlers

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/clay-wangzhi/Synapse/internal/apierrors"
	"github.com/clay-wangzhi/Synapse/internal/config"
	"github.com/clay-wangzhi/Synapse/internal/response"
	"github.com/clay-wangzhi/Synapse/internal/services"
)

// AuditHandler 审计处理器
type AuditHandler struct {
	db           *gorm.DB
	cfg          *config.Config
	auditService *services.AuditService
	opLogService *services.OperationLogService
}

// NewAuditHandler 创建审计处理器
func NewAuditHandler(db *gorm.DB, cfg *config.Config) *AuditHandler {
	return &AuditHandler{
		db:           db,
		cfg:          cfg,
		auditService: services.NewAuditService(db),
		opLogService: services.NewOperationLogService(db),
	}
}

// GetAuditLogs 获取统一审计日志（委派 OperationLogService.List）
// 支援查詢參數：page, pageSize, username, module, action, result(success/failed), startTime, endTime, keyword
func (h *AuditHandler) GetAuditLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

	req := &services.OperationLogListRequest{
		Page:         page,
		PageSize:     pageSize,
		Username:     c.Query("username"),
		Module:       c.Query("module"),
		Action:       c.Query("action"),
		ResourceType: c.Query("resourceType"),
		Keyword:      c.Query("keyword"),
	}

	// result=success|failed
	if result := c.Query("result"); result != "" {
		ok := result == "success"
		req.Success = &ok
	}

	// clusterID
	if cidStr := c.Query("clusterId"); cidStr != "" {
		if cid, err := strconv.ParseUint(cidStr, 10, 32); err == nil {
			uid := uint(cid)
			req.ClusterID = &uid
		}
	}

	if userIDStr := c.Query("userId"); userIDStr != "" {
		if uid, err := strconv.ParseUint(userIDStr, 10, 32); err == nil {
			uidVal := uint(uid)
			req.UserID = &uidVal
		}
	}

	if startStr := c.Query("startTime"); startStr != "" {
		if t, err := time.Parse(time.RFC3339, startStr); err == nil {
			req.StartTime = &t
		}
	}
	if endStr := c.Query("endTime"); endStr != "" {
		if t, err := time.Parse(time.RFC3339, endStr); err == nil {
			req.EndTime = &t
		}
	}

	resp, err := h.opLogService.List(req)
	if err != nil {
		response.FromError(c, apierrors.ErrInternal("获取审计日志失败"))
		return
	}

	response.OK(c, resp)
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
		response.FromError(c, apierrors.ErrInternal("获取会话列表失败"))
		return
	}

	response.OK(c, resp)
}

// GetTerminalSession 获取终端会话详情
func (h *AuditHandler) GetTerminalSession(c *gin.Context) {
	sessionIDStr := c.Param("sessionId")
	sessionID, err := strconv.ParseUint(sessionIDStr, 10, 32)
	if err != nil {
		response.FromError(c, apierrors.ErrBadRequest("无效的会话ID"))
		return
	}

	session, err := h.auditService.GetSessionDetail(uint(sessionID))
	if err != nil {
		response.FromError(c, apierrors.ErrInternal("会话不存在"))
		return
	}

	response.OK(c, session)
}

// GetTerminalCommands 获取终端命令记录
func (h *AuditHandler) GetTerminalCommands(c *gin.Context) {
	sessionIDStr := c.Param("sessionId")
	sessionID, err := strconv.ParseUint(sessionIDStr, 10, 32)
	if err != nil {
		response.FromError(c, apierrors.ErrBadRequest("无效的会话ID"))
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "100"))

	resp, err := h.auditService.GetSessionCommands(uint(sessionID), page, pageSize)
	if err != nil {
		response.FromError(c, apierrors.ErrInternal("获取命令记录失败"))
		return
	}

	response.OK(c, resp)
}

// GetTerminalStats 获取终端会话统计
func (h *AuditHandler) GetTerminalStats(c *gin.Context) {
	stats, err := h.auditService.GetSessionStats()
	if err != nil {
		response.FromError(c, apierrors.ErrInternal("获取统计信息失败"))
		return
	}

	response.OK(c, stats)
}
