package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/shaia/Synapse/internal/response"
)

// --- GRPCRoute（Phase 3）---

func (h *GatewayHandler) ListGRPCRoutes(c *gin.Context) {
	svc, ok := h.getGatewaySvc(c, true)
	if !ok {
		return
	}
	ns := c.Query("namespace")
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	items, err := svc.ListGRPCRoutes(ctx, ns)
	if err != nil {
		response.InternalError(c, fmt.Sprintf("列出 GRPCRoute 失敗: %v", err))
		return
	}
	response.OK(c, gin.H{"items": items, "total": len(items)})
}

func (h *GatewayHandler) GetGRPCRoute(c *gin.Context) {
	namespace, name := c.Param("namespace"), c.Param("name")
	svc, ok := h.getGatewaySvc(c, true)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	item, err := svc.GetGRPCRoute(ctx, namespace, name)
	if err != nil {
		response.InternalError(c, fmt.Sprintf("取得 GRPCRoute 失敗: %v", err))
		return
	}
	response.OK(c, item)
}

func (h *GatewayHandler) GetGRPCRouteYAML(c *gin.Context) {
	namespace, name := c.Param("namespace"), c.Param("name")
	svc, ok := h.getGatewaySvc(c, true)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	yamlStr, err := svc.GetGRPCRouteYAML(ctx, namespace, name)
	if err != nil {
		response.InternalError(c, fmt.Sprintf("取得 GRPCRoute YAML 失敗: %v", err))
		return
	}
	response.OK(c, gin.H{"yaml": yamlStr})
}

func (h *GatewayHandler) CreateGRPCRoute(c *gin.Context) {
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
	item, err := svc.CreateGRPCRoute(ctx, req.Namespace, req.YAML, req.DryRun)
	if err != nil {
		response.BadRequest(c, fmt.Sprintf("建立 GRPCRoute 失敗: %v", err))
		return
	}
	response.OK(c, item)
}

func (h *GatewayHandler) UpdateGRPCRoute(c *gin.Context) {
	namespace, name := c.Param("namespace"), c.Param("name")
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
	item, err := svc.UpdateGRPCRoute(ctx, namespace, name, req.YAML, req.DryRun)
	if err != nil {
		response.BadRequest(c, fmt.Sprintf("更新 GRPCRoute 失敗: %v", err))
		return
	}
	response.OK(c, item)
}

func (h *GatewayHandler) DeleteGRPCRoute(c *gin.Context) {
	namespace, name := c.Param("namespace"), c.Param("name")
	svc, ok := h.getGatewaySvc(c, true)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	if err := svc.DeleteGRPCRoute(ctx, namespace, name); err != nil {
		response.InternalError(c, fmt.Sprintf("刪除 GRPCRoute 失敗: %v", err))
		return
	}
	response.NoContent(c)
}
