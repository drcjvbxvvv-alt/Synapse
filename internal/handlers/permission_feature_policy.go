package handlers

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/pkg/logger"
)

// ─── DTOs ─────────────────────────────────────────────────────────────────────

// FeaturePolicyResponse is the body for GET /permissions/cluster-permissions/:id/features.
type FeaturePolicyResponse struct {
	PermissionID   uint            `json:"permission_id"`
	PermissionType string          `json:"permission_type"`
	Ceiling        []string        `json:"ceiling"`  // max allowed keys for this type
	Policy         map[string]bool `json:"policy"`   // stored overrides (explicit false entries)
	Effective      []string        `json:"effective"` // ceiling ∩ policy — what the user actually gets
}

// UpdateFeaturePolicyRequest is the body for PATCH /permissions/cluster-permissions/:id/features.
type UpdateFeaturePolicyRequest struct {
	Policy map[string]bool `json:"policy" binding:"required"`
}

// ─── Handlers ─────────────────────────────────────────────────────────────────

// GetFeaturePolicy returns the feature policy for a ClusterPermission record.
// GET /api/v1/permissions/cluster-permissions/:id/features
func (h *PermissionHandler) GetFeaturePolicy(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid permission ID")
		return
	}

	permission, err := h.permissionService.GetClusterPermission(uint(id))
	if err != nil {
		response.NotFound(c, "permission not found")
		return
	}

	ceiling := models.FeatureCeilings[permission.PermissionType]
	if ceiling == nil {
		ceiling = []string{}
	}

	response.OK(c, FeaturePolicyResponse{
		PermissionID:   permission.ID,
		PermissionType: permission.PermissionType,
		Ceiling:        ceiling,
		Policy:         permission.GetFeaturePolicyMap(),
		Effective:      models.ComputeAllowedFeatures(permission.PermissionType, permission.FeaturePolicy),
	})
}

// UpdateFeaturePolicy replaces the feature policy for a ClusterPermission record.
// PATCH /api/v1/permissions/cluster-permissions/:id/features
func (h *PermissionHandler) UpdateFeaturePolicy(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid permission ID")
		return
	}

	var req UpdateFeaturePolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body: "+err.Error())
		return
	}

	logger.Info("updating feature policy",
		"permission_id", id,
		"policy_keys", len(req.Policy),
	)

	updated, err := h.permissionService.UpdateFeaturePolicy(uint(id), req.Policy)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}

	response.OK(c, gin.H{
		"effective": models.ComputeAllowedFeatures(updated.PermissionType, updated.FeaturePolicy),
	})
}
