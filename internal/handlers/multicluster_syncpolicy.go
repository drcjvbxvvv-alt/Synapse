package handlers

import (
	"context"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/response"
)

// ListSyncPolicies GET /multicluster/sync-policies
func (h *MultiClusterHandler) ListSyncPolicies(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	policies, err := h.syncPolicySvc.ListPolicies(ctx)
	if err != nil {
		response.InternalError(c, "查詢同步策略失敗")
		return
	}
	response.List(c, policies, int64(len(policies)))
}

// CreateSyncPolicy POST /multicluster/sync-policies
func (h *MultiClusterHandler) CreateSyncPolicy(c *gin.Context) {
	var policy models.SyncPolicy
	if err := c.ShouldBindJSON(&policy); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	if err := h.syncPolicySvc.CreatePolicy(ctx, &policy); err != nil {
		response.InternalError(c, "建立同步策略失敗")
		return
	}
	response.Created(c, policy)
}

// GetSyncPolicy GET /multicluster/sync-policies/:id
func (h *MultiClusterHandler) GetSyncPolicy(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		response.BadRequest(c, "無效 ID")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	policy, err := h.syncPolicySvc.GetPolicy(ctx, uint(id)) //nolint:gosec // id > 0 verified
	if err != nil {
		response.NotFound(c, "同步策略不存在")
		return
	}
	response.OK(c, policy)
}

// UpdateSyncPolicy PUT /multicluster/sync-policies/:id
func (h *MultiClusterHandler) UpdateSyncPolicy(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		response.BadRequest(c, "無效 ID")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	policy, err := h.syncPolicySvc.GetPolicy(ctx, uint(id)) //nolint:gosec // id > 0 verified
	if err != nil {
		response.NotFound(c, "同步策略不存在")
		return
	}
	if err := c.ShouldBindJSON(policy); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	policy.ID = uint(id) //nolint:gosec // id > 0 verified above
	if err := h.syncPolicySvc.UpdatePolicy(ctx, policy); err != nil {
		response.InternalError(c, "更新同步策略失敗")
		return
	}
	response.OK(c, policy)
}

// DeleteSyncPolicy DELETE /multicluster/sync-policies/:id
func (h *MultiClusterHandler) DeleteSyncPolicy(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		response.BadRequest(c, "無效 ID")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	if err := h.syncPolicySvc.DeletePolicy(ctx, uint(id)); err != nil { //nolint:gosec // id > 0 verified
		response.InternalError(c, "刪除同步策略失敗")
		return
	}
	response.OK(c, gin.H{"deleted": true})
}

// TriggerSync POST /multicluster/sync-policies/:id/trigger
func (h *MultiClusterHandler) TriggerSync(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		response.BadRequest(c, "無效 ID")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	policy, err := h.syncPolicySvc.GetPolicy(ctx, uint(id)) //nolint:gosec // id > 0 verified
	if err != nil {
		response.NotFound(c, "同步策略不存在")
		return
	}

	now := time.Now()
	policy.LastSyncAt = &now

	status, message, details := h.executeSync(policy)
	finished := time.Now()
	hist := models.SyncHistory{
		PolicyID:    policy.ID,
		TriggeredBy: "manual",
		StartedAt:   now,
		Status:      status,
		Message:     message,
		Details:     details,
		FinishedAt:  &finished,
	}

	h.syncPolicySvc.RecordSyncHistory(ctx, &hist)
	h.syncPolicySvc.UpdatePolicySyncStatus(ctx, policy, status)

	response.OK(c, hist)
}

// GetSyncHistory GET /multicluster/sync-policies/:id/history
func (h *MultiClusterHandler) GetSyncHistory(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		response.BadRequest(c, "無效 ID")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	history, err := h.syncPolicySvc.ListSyncHistory(ctx, uint(id), 50) //nolint:gosec // id > 0 verified
	if err != nil {
		response.InternalError(c, "查詢同步歷史失敗")
		return
	}
	response.List(c, history, int64(len(history)))
}
