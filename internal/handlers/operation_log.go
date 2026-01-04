package handlers

import (
	"net/http"
	"strconv"
	"time"

	"kubepolaris/internal/constants"
	"kubepolaris/internal/services"

	"github.com/gin-gonic/gin"
)

// OperationLogHandler 操作日志处理器
type OperationLogHandler struct {
	opLogSvc *services.OperationLogService
}

// NewOperationLogHandler 创建操作日志处理器
func NewOperationLogHandler(opLogSvc *services.OperationLogService) *OperationLogHandler {
	return &OperationLogHandler{
		opLogSvc: opLogSvc,
	}
}

// GetOperationLogs 获取操作日志列表
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

	// 解析用户ID
	if userIDStr := c.Query("userId"); userIDStr != "" {
		if uid, err := strconv.ParseUint(userIDStr, 10, 32); err == nil {
			uidVal := uint(uid)
			req.UserID = &uidVal
		}
	}

	// 解析集群ID
	if clusterIDStr := c.Query("clusterId"); clusterIDStr != "" {
		if cid, err := strconv.ParseUint(clusterIDStr, 10, 32); err == nil {
			cidVal := uint(cid)
			req.ClusterID = &cidVal
		}
	}

	// 解析成功/失败
	if successStr := c.Query("success"); successStr != "" {
		successVal := successStr == "true"
		req.Success = &successVal
	}

	// 解析时间范围
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
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取操作日志失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "获取成功",
		"data":    resp,
	})
}

// GetOperationLog 获取操作日志详情
func (h *OperationLogHandler) GetOperationLog(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的日志ID",
		})
		return
	}

	log, err := h.opLogSvc.GetDetail(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "日志不存在",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "获取成功",
		"data":    log,
	})
}

// GetOperationLogStats 获取操作日志统计
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

// GetModules 获取模块列表
func (h *OperationLogHandler) GetModules(c *gin.Context) {
	modules := []map[string]string{}
	for key, name := range constants.ModuleNames {
		modules = append(modules, map[string]string{
			"key":  key,
			"name": name,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "获取成功",
		"data":    modules,
	})
}

// GetActions 获取操作列表
func (h *OperationLogHandler) GetActions(c *gin.Context) {
	actions := []map[string]string{}
	for key, name := range constants.ActionNames {
		actions = append(actions, map[string]string{
			"key":  key,
			"name": name,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "获取成功",
		"data":    actions,
	})
}

// getIntParam 获取整数参数
func getIntParam(c *gin.Context, key string, defaultValue int) int {
	if str := c.Query(key); str != "" {
		if val, err := strconv.Atoi(str); err == nil {
			return val
		}
	}
	return defaultValue
}

