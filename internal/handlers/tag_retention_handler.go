package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/pkg/logger"
)

// ---------------------------------------------------------------------------
// TagRetentionHandler — Tag 保留策略 CRUD + 評估（CICD_ARCHITECTURE §11, M15）
// ---------------------------------------------------------------------------

// TagRetentionHandler 管理 Tag 保留策略 API。
type TagRetentionHandler struct {
	retentionSvc *services.TagRetentionService
}

// NewTagRetentionHandler 建立 TagRetentionHandler。
func NewTagRetentionHandler(retentionSvc *services.TagRetentionService) *TagRetentionHandler {
	return &TagRetentionHandler{retentionSvc: retentionSvc}
}

// ─── DTOs ──────────────────────────────────────────────────────────────────

// CreateTagRetentionPolicyRequest 建立保留策略的請求。
type CreateTagRetentionPolicyRequest struct {
	RepositoryMatch string `json:"repository_match" binding:"required,max=512"`
	TagMatch        string `json:"tag_match"`
	RetentionType   string `json:"retention_type" binding:"required,oneof=keep_last_n keep_by_age keep_by_regex"`
	KeepCount       int    `json:"keep_count,omitempty"`
	KeepDays        int    `json:"keep_days,omitempty"`
	KeepPattern     string `json:"keep_pattern,omitempty"`
	CronExpr        string `json:"cron_expr,omitempty"`
}

// UpdateTagRetentionPolicyRequest 更新保留策略的請求。
type UpdateTagRetentionPolicyRequest struct {
	RepositoryMatch *string `json:"repository_match,omitempty"`
	TagMatch        *string `json:"tag_match,omitempty"`
	RetentionType   *string `json:"retention_type,omitempty"`
	KeepCount       *int    `json:"keep_count,omitempty"`
	KeepDays        *int    `json:"keep_days,omitempty"`
	KeepPattern     *string `json:"keep_pattern,omitempty"`
	CronExpr        *string `json:"cron_expr,omitempty"`
	Enabled         *bool   `json:"enabled,omitempty"`
}

// ─── Handlers ──────────────────────────────────────────────────────────────

// List 列出 Registry 的保留策略。
// GET /system/registries/:id/retention-policies
func (h *TagRetentionHandler) List(c *gin.Context) {
	registryID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid registry ID")
		return
	}

	policies, err := h.retentionSvc.ListPolicies(c.Request.Context(), uint(registryID))
	if err != nil {
		logger.Error("failed to list retention policies", "registry_id", registryID, "error", err)
		response.InternalError(c, "failed to list retention policies: "+err.Error())
		return
	}
	response.List(c, policies, int64(len(policies)))
}

// Create 建立保留策略。
// POST /system/registries/:id/retention-policies
func (h *TagRetentionHandler) Create(c *gin.Context) {
	registryID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid registry ID")
		return
	}

	var req CreateTagRetentionPolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body: "+err.Error())
		return
	}

	userID, _ := c.Get("userID")
	uid, _ := userID.(uint)

	tagMatch := req.TagMatch
	if tagMatch == "" {
		tagMatch = "*"
	}

	policy := &models.TagRetentionPolicy{
		RegistryID:      uint(registryID),
		RepositoryMatch: req.RepositoryMatch,
		TagMatch:        tagMatch,
		RetentionType:   req.RetentionType,
		KeepCount:       req.KeepCount,
		KeepDays:        req.KeepDays,
		KeepPattern:     req.KeepPattern,
		CronExpr:        req.CronExpr,
		Enabled:         true,
		CreatedBy:       uid,
	}

	if err := h.retentionSvc.CreatePolicy(c.Request.Context(), policy); err != nil {
		logger.Error("failed to create retention policy", "registry_id", registryID, "error", err)
		response.InternalError(c, "failed to create retention policy: "+err.Error())
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":             policy.ID,
		"registry_id":    policy.RegistryID,
		"retention_type": policy.RetentionType,
	})
}

// Update 更新保留策略。
// PUT /system/registries/:id/retention-policies/:policyID
func (h *TagRetentionHandler) Update(c *gin.Context) {
	_, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid registry ID")
		return
	}
	policyID, err := strconv.ParseUint(c.Param("policyID"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid policy ID")
		return
	}

	var req UpdateTagRetentionPolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body: "+err.Error())
		return
	}

	updates := make(map[string]interface{})
	if req.RepositoryMatch != nil {
		updates["repository_match"] = *req.RepositoryMatch
	}
	if req.TagMatch != nil {
		updates["tag_match"] = *req.TagMatch
	}
	if req.RetentionType != nil {
		updates["retention_type"] = *req.RetentionType
	}
	if req.KeepCount != nil {
		updates["keep_count"] = *req.KeepCount
	}
	if req.KeepDays != nil {
		updates["keep_days"] = *req.KeepDays
	}
	if req.KeepPattern != nil {
		updates["keep_pattern"] = *req.KeepPattern
	}
	if req.CronExpr != nil {
		updates["cron_expr"] = *req.CronExpr
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}

	if len(updates) == 0 {
		response.BadRequest(c, "no fields to update")
		return
	}

	if err := h.retentionSvc.UpdatePolicy(c.Request.Context(), uint(policyID), updates); err != nil {
		logger.Error("failed to update retention policy", "policy_id", policyID, "error", err)
		response.InternalError(c, "failed to update retention policy: "+err.Error())
		return
	}

	logger.Info("retention policy updated", "policy_id", policyID)
	response.OK(c, gin.H{"message": "updated"})
}

// Delete 刪除保留策略。
// DELETE /system/registries/:id/retention-policies/:policyID
func (h *TagRetentionHandler) Delete(c *gin.Context) {
	_, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid registry ID")
		return
	}
	policyID, err := strconv.ParseUint(c.Param("policyID"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid policy ID")
		return
	}

	if err := h.retentionSvc.DeletePolicy(c.Request.Context(), uint(policyID)); err != nil {
		logger.Error("failed to delete retention policy", "policy_id", policyID, "error", err)
		response.InternalError(c, "failed to delete retention policy: "+err.Error())
		return
	}

	logger.Info("retention policy deleted", "policy_id", policyID)
	response.OK(c, gin.H{"message": "deleted"})
}

// Evaluate 評估保留策略（dry-run）。
// POST /system/registries/:id/retention-policies/:policyID/evaluate
func (h *TagRetentionHandler) Evaluate(c *gin.Context) {
	_, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid registry ID")
		return
	}
	policyID, err := strconv.ParseUint(c.Param("policyID"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid policy ID")
		return
	}

	results, err := h.retentionSvc.EvaluatePolicy(c.Request.Context(), uint(policyID))
	if err != nil {
		logger.Error("failed to evaluate retention policy", "policy_id", policyID, "error", err)
		response.InternalError(c, "failed to evaluate retention policy: "+err.Error())
		return
	}

	response.OK(c, gin.H{
		"policy_id": policyID,
		"dry_run":   true,
		"results":   results,
	})
}
