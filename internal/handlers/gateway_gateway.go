package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/pkg/logger"
)

// --- Gateway ---

// ListGateways 列出 Gateway（支援 namespace 過濾）
// GET /api/v1/clusters/:clusterID/gateways?namespace=xxx
func (h *GatewayHandler) ListGateways(c *gin.Context) {
	logger.Info("列出 Gateway")
	namespace := c.Query("namespace")

	svc, ok := h.getGatewaySvc(c, true)
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	items, err := svc.ListGateways(ctx, namespace)
	if err != nil {
		logger.Error("列出 Gateway 失敗", "error", err)
		response.InternalError(c, fmt.Sprintf("列出 Gateway 失敗: %v", err))
		return
	}

	response.OK(c, gin.H{"items": items, "total": len(items)})
}

// GetGateway 取得單一 Gateway
// GET /api/v1/clusters/:clusterID/gateways/:namespace/:name
func (h *GatewayHandler) GetGateway(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")

	svc, ok := h.getGatewaySvc(c, true)
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	item, err := svc.GetGateway(ctx, namespace, name)
	if err != nil {
		logger.Error("取得 Gateway 失敗", "error", err, "namespace", namespace, "name", name)
		response.InternalError(c, fmt.Sprintf("取得 Gateway 失敗: %v", err))
		return
	}

	response.OK(c, item)
}

// GetGatewayYAML 取得 Gateway YAML
// GET /api/v1/clusters/:clusterID/gateways/:namespace/:name/yaml
func (h *GatewayHandler) GetGatewayYAML(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")

	svc, ok := h.getGatewaySvc(c, true)
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	yamlStr, err := svc.GetGatewayYAML(ctx, namespace, name)
	if err != nil {
		logger.Error("取得 Gateway YAML 失敗", "error", err, "namespace", namespace, "name", name)
		response.InternalError(c, fmt.Sprintf("取得 Gateway YAML 失敗: %v", err))
		return
	}

	response.OK(c, gin.H{"yaml": yamlStr})
}

// --- Gateway CRUD（Phase 2）---

// CreateGateway 建立 Gateway
// POST /api/v1/clusters/:clusterID/gateways
func (h *GatewayHandler) CreateGateway(c *gin.Context) {
	var req struct {
		Namespace string `json:"namespace" binding:"required"`
		YAML      string `json:"yaml" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "請求格式錯誤: "+err.Error())
		return
	}

	svc, ok := h.getGatewaySvc(c, true)
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	item, err := svc.CreateGateway(ctx, req.Namespace, req.YAML)
	if err != nil {
		logger.Error("建立 Gateway 失敗", "error", err)
		response.BadRequest(c, fmt.Sprintf("建立 Gateway 失敗: %v", err))
		return
	}
	response.OK(c, item)
}

// UpdateGateway 更新 Gateway
// PUT /api/v1/clusters/:clusterID/gateways/:namespace/:name
func (h *GatewayHandler) UpdateGateway(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")

	var req struct {
		YAML string `json:"yaml" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "請求格式錯誤: "+err.Error())
		return
	}

	svc, ok := h.getGatewaySvc(c, true)
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	item, err := svc.UpdateGateway(ctx, namespace, name, req.YAML)
	if err != nil {
		logger.Error("更新 Gateway 失敗", "error", err, "namespace", namespace, "name", name)
		response.BadRequest(c, fmt.Sprintf("更新 Gateway 失敗: %v", err))
		return
	}
	response.OK(c, item)
}

// DeleteGateway 刪除 Gateway
// DELETE /api/v1/clusters/:clusterID/gateways/:namespace/:name
func (h *GatewayHandler) DeleteGateway(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")

	svc, ok := h.getGatewaySvc(c, true)
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	if err := svc.DeleteGateway(ctx, namespace, name); err != nil {
		logger.Error("刪除 Gateway 失敗", "error", err, "namespace", namespace, "name", name)
		response.InternalError(c, fmt.Sprintf("刪除 Gateway 失敗: %v", err))
		return
	}
	response.NoContent(c)
}
