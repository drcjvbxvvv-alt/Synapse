package handlers

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/shaia/Synapse/internal/apierrors"
	"github.com/shaia/Synapse/internal/config"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
)

// AuditHandler 審計處理器
type AuditHandler struct {
	db           *gorm.DB
	cfg          *config.Config
	auditService *services.AuditService
	opLogService *services.OperationLogService
}

// NewAuditHandler 建立審計處理器
func NewAuditHandler(db *gorm.DB, cfg *config.Config) *AuditHandler {
	return &AuditHandler{
		db:           db,
		cfg:          cfg,
		auditService: services.NewAuditService(db),
		opLogService: services.NewOperationLogService(db),
	}
}

// GetAuditLogs 獲取統一審計日誌（委派 OperationLogService.List）
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
		response.FromError(c, apierrors.ErrInternal("獲取審計日誌失敗"))
		return
	}

	response.OK(c, resp)
}

// GetTerminalSessions 獲取終端會話記錄
func (h *AuditHandler) GetTerminalSessions(c *gin.Context) {
	// 解析查詢參數
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
		response.FromError(c, apierrors.ErrInternal("獲取會話列表失敗"))
		return
	}

	response.OK(c, resp)
}

// GetTerminalSession 獲取終端會話詳情
func (h *AuditHandler) GetTerminalSession(c *gin.Context) {
	sessionIDStr := c.Param("sessionId")
	sessionID, err := strconv.ParseUint(sessionIDStr, 10, 32)
	if err != nil {
		response.FromError(c, apierrors.ErrBadRequest("無效的會話ID"))
		return
	}

	session, err := h.auditService.GetSessionDetail(uint(sessionID))
	if err != nil {
		response.FromError(c, apierrors.ErrInternal("會話不存在"))
		return
	}

	response.OK(c, session)
}

// GetTerminalCommands 獲取終端命令記錄
func (h *AuditHandler) GetTerminalCommands(c *gin.Context) {
	sessionIDStr := c.Param("sessionId")
	sessionID, err := strconv.ParseUint(sessionIDStr, 10, 32)
	if err != nil {
		response.FromError(c, apierrors.ErrBadRequest("無效的會話ID"))
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "100"))

	resp, err := h.auditService.GetSessionCommands(uint(sessionID), page, pageSize)
	if err != nil {
		response.FromError(c, apierrors.ErrInternal("獲取命令記錄失敗"))
		return
	}

	response.OK(c, resp)
}

// GetTerminalStats 獲取終端會話統計
func (h *AuditHandler) GetTerminalStats(c *gin.Context) {
	stats, err := h.auditService.GetSessionStats()
	if err != nil {
		response.FromError(c, apierrors.ErrInternal("獲取統計資訊失敗"))
		return
	}

	response.OK(c, stats)
}
