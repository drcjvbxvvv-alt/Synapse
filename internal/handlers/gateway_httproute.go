package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/pkg/logger"
)

// --- HTTPRoute ---

// ListHTTPRoutes 列出 HTTPRoute（支援 namespace 過濾）
// GET /api/v1/clusters/:clusterID/httproutes?namespace=xxx
func (h *GatewayHandler) ListHTTPRoutes(c *gin.Context) {
	logger.Info("列出 HTTPRoute")
	namespace := c.Query("namespace")

	svc, ok := h.getGatewaySvc(c, true)
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	items, err := svc.ListHTTPRoutes(ctx, namespace)
	if err != nil {
		logger.Error("列出 HTTPRoute 失敗", "error", err)
		response.InternalError(c, fmt.Sprintf("列出 HTTPRoute 失敗: %v", err))
		return
	}

	response.OK(c, gin.H{"items": items, "total": len(items)})
}

// GetHTTPRoute 取得單一 HTTPRoute
// GET /api/v1/clusters/:clusterID/httproutes/:namespace/:name
func (h *GatewayHandler) GetHTTPRoute(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")

	svc, ok := h.getGatewaySvc(c, true)
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	item, err := svc.GetHTTPRoute(ctx, namespace, name)
	if err != nil {
		logger.Error("取得 HTTPRoute 失敗", "error", err, "namespace", namespace, "name", name)
		response.InternalError(c, fmt.Sprintf("取得 HTTPRoute 失敗: %v", err))
		return
	}

	response.OK(c, item)
}

// GetHTTPRouteYAML 取得 HTTPRoute YAML
// GET /api/v1/clusters/:clusterID/httproutes/:namespace/:name/yaml
func (h *GatewayHandler) GetHTTPRouteYAML(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")

	svc, ok := h.getGatewaySvc(c, true)
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	yamlStr, err := svc.GetHTTPRouteYAML(ctx, namespace, name)
	if err != nil {
		logger.Error("取得 HTTPRoute YAML 失敗", "error", err, "namespace", namespace, "name", name)
		response.InternalError(c, fmt.Sprintf("取得 HTTPRoute YAML 失敗: %v", err))
		return
	}

	response.OK(c, gin.H{"yaml": yamlStr})
}

// --- HTTPRoute CRUD（Phase 2）---

// CreateHTTPRoute 建立 HTTPRoute
// POST /api/v1/clusters/:clusterID/httproutes
func (h *GatewayHandler) CreateHTTPRoute(c *gin.Context) {
	var req struct {
		Namespace string `json:"namespace" binding:"required"`
		YAML      string `json:"yaml" binding:"required"`
		DryRun    bool   `json:"dryRun"`
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

	item, err := svc.CreateHTTPRoute(ctx, req.Namespace, req.YAML, req.DryRun)
	if err != nil {
		logger.Error("建立 HTTPRoute 失敗", "error", err)
		response.BadRequest(c, fmt.Sprintf("建立 HTTPRoute 失敗: %v", err))
		return
	}
	response.OK(c, item)
}

// UpdateHTTPRoute 更新 HTTPRoute
// PUT /api/v1/clusters/:clusterID/httproutes/:namespace/:name
func (h *GatewayHandler) UpdateHTTPRoute(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")

	var req struct {
		YAML   string `json:"yaml" binding:"required"`
		DryRun bool   `json:"dryRun"`
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

	item, err := svc.UpdateHTTPRoute(ctx, namespace, name, req.YAML, req.DryRun)
	if err != nil {
		logger.Error("更新 HTTPRoute 失敗", "error", err, "namespace", namespace, "name", name)
		response.BadRequest(c, fmt.Sprintf("更新 HTTPRoute 失敗: %v", err))
		return
	}
	response.OK(c, item)
}

// DeleteHTTPRoute 刪除 HTTPRoute
// DELETE /api/v1/clusters/:clusterID/httproutes/:namespace/:name
func (h *GatewayHandler) DeleteHTTPRoute(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")

	svc, ok := h.getGatewaySvc(c, true)
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	if err := svc.DeleteHTTPRoute(ctx, namespace, name); err != nil {
		logger.Error("刪除 HTTPRoute 失敗", "error", err, "namespace", namespace, "name", name)
		response.InternalError(c, fmt.Sprintf("刪除 HTTPRoute 失敗: %v", err))
		return
	}
	response.NoContent(c)
}
