package handlers

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shaia/Synapse/internal/features"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/pkg/logger"
)

// FeatureFlagHandler manages platform-wide feature flags.
// All routes require PlatformAdminRequired middleware (enforced in routes_system.go).
type FeatureFlagHandler struct {
	svc     *services.FeatureFlagService
	dbStore *features.DBStore // invalidated after each Set so changes are instant
}

func NewFeatureFlagHandler(svc *services.FeatureFlagService, dbStore *features.DBStore) *FeatureFlagHandler {
	return &FeatureFlagHandler{svc: svc, dbStore: dbStore}
}

// UpdateFeatureFlagRequest is the request body for PUT /system/feature-flags/:key.
type UpdateFeatureFlagRequest struct {
	Enabled     bool   `json:"enabled"`
	Description string `json:"description"`
}

// List returns all feature flags.
// GET /system/feature-flags
func (h *FeatureFlagHandler) List(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	flags, err := h.svc.ListFlags(ctx)
	if err != nil {
		logger.Error("failed to list feature flags", "error", err)
		response.InternalError(c, "failed to list feature flags: "+err.Error())
		return
	}

	response.List(c, flags, int64(len(flags)))
}

// Update enables or disables a feature flag.
// PUT /system/feature-flags/:key
func (h *FeatureFlagHandler) Update(c *gin.Context) {
	key := c.Param("key")
	if key == "" {
		response.BadRequest(c, "flag key is required")
		return
	}

	var req UpdateFeatureFlagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body: "+err.Error())
		return
	}

	// Extract the requesting user's name from the JWT claims set by AuthRequired.
	updatedBy, _ := c.Get("username")
	updatedByStr, _ := updatedBy.(string)

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	logger.Info("feature flag updated",
		"key", key,
		"enabled", req.Enabled,
		"updated_by", updatedByStr,
	)

	if err := h.svc.SetFlag(ctx, key, req.Enabled, req.Description, updatedByStr); err != nil {
		logger.Error("failed to update feature flag", "key", key, "error", err)
		response.InternalError(c, "failed to update feature flag: "+err.Error())
		return
	}

	// Invalidate the DB store cache so the new value takes effect immediately.
	if h.dbStore != nil {
		h.dbStore.Invalidate()
	}

	response.OK(c, gin.H{"key": key, "enabled": req.Enabled})
}
