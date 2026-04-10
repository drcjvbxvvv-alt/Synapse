package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/shaia/Synapse/internal/response"
)

// --- ReferenceGrant（Phase 3）---

func (h *GatewayHandler) ListReferenceGrants(c *gin.Context) {
	svc, ok := h.getGatewaySvc(c, true)
	if !ok {
		return
	}
	ns := c.Query("namespace")
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	items, err := svc.ListReferenceGrants(ctx, ns)
	if err != nil {
		response.InternalError(c, fmt.Sprintf("列出 ReferenceGrant 失敗: %v", err))
		return
	}
	response.OK(c, gin.H{"items": items, "total": len(items)})
}

func (h *GatewayHandler) GetReferenceGrantYAML(c *gin.Context) {
	namespace, name := c.Param("namespace"), c.Param("name")
	svc, ok := h.getGatewaySvc(c, true)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	yamlStr, err := svc.GetReferenceGrantYAML(ctx, namespace, name)
	if err != nil {
		response.InternalError(c, fmt.Sprintf("取得 ReferenceGrant YAML 失敗: %v", err))
		return
	}
	response.OK(c, gin.H{"yaml": yamlStr})
}

func (h *GatewayHandler) CreateReferenceGrant(c *gin.Context) {
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
	item, err := svc.CreateReferenceGrant(ctx, req.Namespace, req.YAML, req.DryRun)
	if err != nil {
		response.BadRequest(c, fmt.Sprintf("建立 ReferenceGrant 失敗: %v", err))
		return
	}
	response.OK(c, item)
}

func (h *GatewayHandler) DeleteReferenceGrant(c *gin.Context) {
	namespace, name := c.Param("namespace"), c.Param("name")
	svc, ok := h.getGatewaySvc(c, true)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	if err := svc.DeleteReferenceGrant(ctx, namespace, name); err != nil {
		response.InternalError(c, fmt.Sprintf("刪除 ReferenceGrant 失敗: %v", err))
		return
	}
	response.NoContent(c)
}

// --- Topology（Phase 3）---

func (h *GatewayHandler) GetTopology(c *gin.Context) {
	svc, ok := h.getGatewaySvc(c, true)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	data, err := svc.GetTopology(ctx)
	if err != nil {
		response.InternalError(c, fmt.Sprintf("取得拓撲失敗: %v", err))
		return
	}
	response.OK(c, data)
}
