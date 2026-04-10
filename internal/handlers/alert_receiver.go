package handlers

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/pkg/logger"
)

// ========== Receivers ==========

// GetReceivers 獲取接收器列表
func (h *AlertHandler) GetReceivers(c *gin.Context) {
	clusterIDStr := c.Param("clusterID")
	clusterID, err := strconv.ParseUint(clusterIDStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "無效的叢集ID")
		return
	}

	// 獲取配置
	config, err := h.alertManagerConfigService.GetAlertManagerConfig(uint(clusterID))
	if err != nil {
		logger.Error("獲取 Alertmanager 配置失敗", "error", err)
		response.InternalError(c, "獲取 Alertmanager 配置失敗: "+err.Error())
		return
	}

	if !config.Enabled {
		response.OK(c, []models.Receiver{})
		return
	}

	// 獲取接收器列表
	receivers, err := h.alertManagerService.GetReceivers(c.Request.Context(), config)
	if err != nil {
		logger.Error("獲取接收器列表失敗", "error", err)
		response.InternalError(c, "獲取接收器列表失敗: "+err.Error())
		return
	}

	response.OK(c, receivers)
}

// GetFullReceivers 取得完整 Receiver 設定（含各渠道詳細參數）
func (h *AlertHandler) GetFullReceivers(c *gin.Context) {
	_, config, ok := h.getAlertConfig(c)
	if !ok {
		return
	}
	if !config.Enabled {
		response.OK(c, []models.ReceiverConfig{})
		return
	}
	receivers, err := h.alertManagerService.GetFullReceivers(c.Request.Context(), config)
	if err != nil {
		logger.Error("取得完整 Receiver 列表失敗", "error", err)
		response.InternalError(c, "取得 Receiver 列表失敗: "+err.Error())
		return
	}
	response.OK(c, receivers)
}

// CreateReceiver 新增 Receiver
func (h *AlertHandler) CreateReceiver(c *gin.Context) {
	clusterID, config, ok := h.getAlertConfig(c)
	if !ok {
		return
	}
	if !config.Enabled {
		response.BadRequest(c, "Alertmanager 未啟用")
		return
	}
	var req models.CreateReceiverRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "請求參數錯誤: "+err.Error())
		return
	}
	clientset, err := h.getClientset(clusterID)
	if err != nil {
		response.InternalError(c, "取得叢集客戶端失敗: "+err.Error())
		return
	}
	if err := h.alertManagerService.CreateReceiver(c.Request.Context(), config, clientset, &req); err != nil {
		logger.Error("新增 Receiver 失敗", "error", err)
		response.InternalError(c, "新增 Receiver 失敗: "+err.Error())
		return
	}
	response.OK(c, gin.H{"message": "新增成功"})
}

// UpdateReceiver 更新 Receiver
func (h *AlertHandler) UpdateReceiver(c *gin.Context) {
	clusterID, config, ok := h.getAlertConfig(c)
	if !ok {
		return
	}
	if !config.Enabled {
		response.BadRequest(c, "Alertmanager 未啟用")
		return
	}
	name := c.Param("name")
	if name == "" {
		response.BadRequest(c, "Receiver 名稱不能為空")
		return
	}
	var req models.UpdateReceiverRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "請求參數錯誤: "+err.Error())
		return
	}
	clientset, err := h.getClientset(clusterID)
	if err != nil {
		response.InternalError(c, "取得叢集客戶端失敗: "+err.Error())
		return
	}
	if err := h.alertManagerService.UpdateReceiver(c.Request.Context(), config, clientset, name, &req); err != nil {
		logger.Error("更新 Receiver 失敗", "error", err)
		response.InternalError(c, "更新 Receiver 失敗: "+err.Error())
		return
	}
	response.OK(c, gin.H{"message": "更新成功"})
}

// DeleteReceiver 刪除 Receiver
func (h *AlertHandler) DeleteReceiver(c *gin.Context) {
	clusterID, config, ok := h.getAlertConfig(c)
	if !ok {
		return
	}
	if !config.Enabled {
		response.BadRequest(c, "Alertmanager 未啟用")
		return
	}
	name := c.Param("name")
	if name == "" {
		response.BadRequest(c, "Receiver 名稱不能為空")
		return
	}
	clientset, err := h.getClientset(clusterID)
	if err != nil {
		response.InternalError(c, "取得叢集客戶端失敗: "+err.Error())
		return
	}
	if err := h.alertManagerService.DeleteReceiver(c.Request.Context(), config, clientset, name); err != nil {
		logger.Error("刪除 Receiver 失敗", "error", err)
		response.InternalError(c, "刪除 Receiver 失敗: "+err.Error())
		return
	}
	response.OK(c, gin.H{"message": "刪除成功"})
}

// TestReceiver 傳送測試告警至指定 Receiver
func (h *AlertHandler) TestReceiver(c *gin.Context) {
	_, config, ok := h.getAlertConfig(c)
	if !ok {
		return
	}
	if !config.Enabled {
		response.BadRequest(c, "Alertmanager 未啟用")
		return
	}
	name := c.Param("name")
	if name == "" {
		response.BadRequest(c, "Receiver 名稱不能為空")
		return
	}
	var req models.TestReceiverRequest
	_ = c.ShouldBindJSON(&req) // 可選 body
	if err := h.alertManagerService.TestReceiver(c.Request.Context(), config, name, &req); err != nil {
		logger.Error("測試 Receiver 失敗", "error", err)
		response.InternalError(c, "測試告警傳送失敗: "+err.Error())
		return
	}
	response.OK(c, gin.H{"message": "測試告警已傳送"})
}
