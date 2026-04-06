package handlers

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/clay-wangzhi/Synapse/internal/constants"
	"github.com/clay-wangzhi/Synapse/internal/response"
	"github.com/clay-wangzhi/Synapse/internal/services"
)

// OperationLogHandler 操作日誌處理器
type OperationLogHandler struct {
	opLogSvc *services.OperationLogService
}

// NewOperationLogHandler 建立操作日誌處理器
func NewOperationLogHandler(opLogSvc *services.OperationLogService) *OperationLogHandler {
	return &OperationLogHandler{
		opLogSvc: opLogSvc,
	}
}

// GetOperationLogs 獲取操作日誌列表
func (h *OperationLogHandler) GetOperationLogs(c *gin.Context) {
	req := &services.OperationLogListRequest{
		Page:         getIntParam(c, "page", 1),
		PageSize:     getIntParam(c, "pageSize", 20),
		Username:     c.Query("username"),
		Module:       c.Query("module"),
		Action:       c.Query("action"),
		ResourceType: c.Query("resourceType"),
		Keyword:      c.Query("keyword"),
	}

	// 解析使用者ID
	if userIDStr := c.Query("userId"); userIDStr != "" {
		if uid, err := strconv.ParseUint(userIDStr, 10, 32); err == nil {
			uidVal := uint(uid)
			req.UserID = &uidVal
		}
	}

	// 解析叢集ID
	if clusterIDStr := c.Query("clusterId"); clusterIDStr != "" {
		if cid, err := strconv.ParseUint(clusterIDStr, 10, 32); err == nil {
			cidVal := uint(cid)
			req.ClusterID = &cidVal
		}
	}

	// 解析成功/失敗
	if successStr := c.Query("success"); successStr != "" {
		successVal := successStr == "true"
		req.Success = &successVal
	}

	// 解析時間範圍
	if startTimeStr := c.Query("startTime"); startTimeStr != "" {
		if t, err := time.Parse(time.RFC3339, startTimeStr); err == nil {
			req.StartTime = &t
		}
	}
	if endTimeStr := c.Query("endTime"); endTimeStr != "" {
		if t, err := time.Parse(time.RFC3339, endTimeStr); err == nil {
			req.EndTime = &t
		}
	}

	resp, err := h.opLogSvc.List(req)
	if err != nil {
		response.InternalError(c, "獲取操作日誌失敗: "+err.Error())
		return
	}

	response.OK(c, resp)
}

// GetOperationLog 獲取操作日誌詳情
func (h *OperationLogHandler) GetOperationLog(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "無效的日誌ID")
		return
	}

	log, err := h.opLogSvc.GetDetail(uint(id))
	if err != nil {
		response.NotFound(c, "日誌不存在")
		return
	}

	response.OK(c, log)
}

// GetOperationLogStats 獲取操作日誌統計
func (h *OperationLogHandler) GetOperationLogStats(c *gin.Context) {
	var startTime, endTime *time.Time

	if startTimeStr := c.Query("startTime"); startTimeStr != "" {
		if t, err := time.Parse(time.RFC3339, startTimeStr); err == nil {
			startTime = &t
		}
	}
	if endTimeStr := c.Query("endTime"); endTimeStr != "" {
		if t, err := time.Parse(time.RFC3339, endTimeStr); err == nil {
			endTime = &t
		}
	}

	stats, err := h.opLogSvc.GetStats(startTime, endTime)
	if err != nil {
		response.InternalError(c, "獲取統計資訊失敗: "+err.Error())
		return
	}

	response.OK(c, stats)
}

// GetModules 獲取模組列表
func (h *OperationLogHandler) GetModules(c *gin.Context) {
	modules := []map[string]string{}
	for key, name := range constants.ModuleNames {
		modules = append(modules, map[string]string{
			"key":  key,
			"name": name,
		})
	}

	response.OK(c, modules)
}

// GetActions 獲取操作列表
func (h *OperationLogHandler) GetActions(c *gin.Context) {
	actions := []map[string]string{}
	for key, name := range constants.ActionNames {
		actions = append(actions, map[string]string{
			"key":  key,
			"name": name,
		})
	}

	response.OK(c, actions)
}

// getIntParam 獲取整數參數
func getIntParam(c *gin.Context, key string, defaultValue int) int {
	if str := c.Query(key); str != "" {
		if val, err := strconv.Atoi(str); err == nil {
			return val
		}
	}
	return defaultValue
}
