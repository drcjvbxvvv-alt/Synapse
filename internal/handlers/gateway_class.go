package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/pkg/logger"
)

// --- GatewayClass ---

// ListGatewayClasses 列出所有 GatewayClass（cluster-scoped）
// GET /api/v1/clusters/:clusterID/gatewayclasses
func (h *GatewayHandler) ListGatewayClasses(c *gin.Context) {
	logger.Info("列出 GatewayClass")

	svc, ok := h.getGatewaySvc(c, true)
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	items, err := svc.ListGatewayClasses(ctx)
	if err != nil {
		logger.Error("列出 GatewayClass 失敗", "error", err)
		response.InternalError(c, fmt.Sprintf("列出 GatewayClass 失敗: %v", err))
		return
	}

	response.OK(c, gin.H{"items": items, "total": len(items)})
}

// GetGatewayClass 取得單一 GatewayClass
// GET /api/v1/clusters/:clusterID/gatewayclasses/:name
func (h *GatewayHandler) GetGatewayClass(c *gin.Context) {
	name := c.Param("name")

	svc, ok := h.getGatewaySvc(c, true)
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	item, err := svc.GetGatewayClass(ctx, name)
	if err != nil {
		logger.Error("取得 GatewayClass 失敗", "error", err, "name", name)
		response.InternalError(c, fmt.Sprintf("取得 GatewayClass 失敗: %v", err))
		return
	}

	response.OK(c, item)
}
