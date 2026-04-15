package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/pkg/logger"
)

// ---------------------------------------------------------------------------
// EnvironmentHandler — Environment CRUD（CICD_ARCHITECTURE §13, M17）
//
// Routes（per-pipeline, under /clusters/:clusterID/pipelines/:pipelineID）：
//   GET    /environments            → List
//   POST   /environments            → Create
//   PUT    /environments/:envID     → Update
//   DELETE /environments/:envID     → Delete
// ---------------------------------------------------------------------------

// EnvironmentHandler 管理 Pipeline Environment CRUD。
type EnvironmentHandler struct {
	envSvc *services.EnvironmentService
}

// NewEnvironmentHandler 建立 EnvironmentHandler。
func NewEnvironmentHandler(envSvc *services.EnvironmentService) *EnvironmentHandler {
	return &EnvironmentHandler{envSvc: envSvc}
}

// ─── DTOs ──────────────────────────────────────────────────────────────────

// CreateEnvironmentRequest 建立 Environment 的請求。
type CreateEnvironmentRequest struct {
	Name              string            `json:"name"               binding:"required,max=100"`
	ClusterID         uint              `json:"cluster_id"         binding:"required"`
	Namespace         string            `json:"namespace"          binding:"required,max=253"`
	OrderIndex        int               `json:"order_index"`
	AutoPromote       bool              `json:"auto_promote"`
	ApprovalRequired  bool              `json:"approval_required"`
	ApproverIDs       []uint            `json:"approver_ids"`
	SmokeTestStepName string            `json:"smoke_test_step_name"`
	NotifyChannelIDs  []uint            `json:"notify_channel_ids"`
	Variables         map[string]string `json:"variables,omitempty"` // 環境特定變數覆寫
}

// UpdateEnvironmentRequest 更新 Environment 的請求。
type UpdateEnvironmentRequest struct {
	Name              *string           `json:"name,omitempty"`
	ClusterID         *uint             `json:"cluster_id,omitempty"`
	Namespace         *string           `json:"namespace,omitempty"`
	OrderIndex        *int              `json:"order_index,omitempty"`
	AutoPromote       *bool             `json:"auto_promote,omitempty"`
	ApprovalRequired  *bool             `json:"approval_required,omitempty"`
	ApproverIDs       []uint            `json:"approver_ids,omitempty"`
	SmokeTestStepName *string           `json:"smoke_test_step_name,omitempty"`
	NotifyChannelIDs  []uint            `json:"notify_channel_ids,omitempty"`
	Variables         map[string]string `json:"variables,omitempty"` // 環境特定變數覆寫
}

// ─── Handlers ──────────────────────────────────────────────────────────────

// List 列出指定 Pipeline 的所有 Environment。
// GET /clusters/:clusterID/pipelines/:pipelineID/environments
func (h *EnvironmentHandler) List(c *gin.Context) {
	pipelineID, err := strconv.ParseUint(c.Param("pipelineID"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid pipeline ID")
		return
	}

	envs, err := h.envSvc.ListEnvironments(c.Request.Context(), uint(pipelineID))
	if err != nil {
		logger.Error("failed to list environments", "pipeline_id", pipelineID, "error", err)
		response.InternalError(c, "failed to list environments: "+err.Error())
		return
	}

	response.List(c, envs, int64(len(envs)))
}

// Create 建立新的 Environment。
// POST /clusters/:clusterID/pipelines/:pipelineID/environments
func (h *EnvironmentHandler) Create(c *gin.Context) {
	pipelineID, err := strconv.ParseUint(c.Param("pipelineID"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid pipeline ID")
		return
	}

	var req CreateEnvironmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body: "+err.Error())
		return
	}

	approverJSON := "[]"
	if len(req.ApproverIDs) > 0 {
		b, _ := json.Marshal(req.ApproverIDs)
		approverJSON = string(b)
	}

	notifyJSON := "[]"
	if len(req.NotifyChannelIDs) > 0 {
		b, _ := json.Marshal(req.NotifyChannelIDs)
		notifyJSON = string(b)
	}

	variablesJSON := "{}"
	if len(req.Variables) > 0 {
		b, _ := json.Marshal(req.Variables)
		variablesJSON = string(b)
	}

	env := &models.Environment{
		Name:              req.Name,
		PipelineID:        uint(pipelineID),
		ClusterID:         req.ClusterID,
		Namespace:         req.Namespace,
		OrderIndex:        req.OrderIndex,
		AutoPromote:       req.AutoPromote,
		ApprovalRequired:  req.ApprovalRequired,
		ApproverIDs:       approverJSON,
		SmokeTestStepName: req.SmokeTestStepName,
		NotifyChannelIDs:  notifyJSON,
		VariablesJSON:     variablesJSON,
	}

	if err := h.envSvc.CreateEnvironment(c.Request.Context(), env); err != nil {
		logger.Error("failed to create environment", "pipeline_id", pipelineID, "error", err)
		response.InternalError(c, "failed to create environment: "+err.Error())
		return
	}

	logger.Info("environment created",
		"environment_id", env.ID,
		"pipeline_id", pipelineID,
		"name", env.Name,
	)

	c.JSON(http.StatusCreated, env)
}

// Update 更新 Environment。
// PUT /clusters/:clusterID/pipelines/:pipelineID/environments/:envID
func (h *EnvironmentHandler) Update(c *gin.Context) {
	envID, err := strconv.ParseUint(c.Param("envID"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid environment ID")
		return
	}

	var req UpdateEnvironmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body: "+err.Error())
		return
	}

	updates := make(map[string]interface{})
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.ClusterID != nil {
		updates["cluster_id"] = *req.ClusterID
	}
	if req.Namespace != nil {
		updates["namespace"] = *req.Namespace
	}
	if req.OrderIndex != nil {
		updates["order_index"] = *req.OrderIndex
	}
	if req.AutoPromote != nil {
		updates["auto_promote"] = *req.AutoPromote
	}
	if req.ApprovalRequired != nil {
		updates["approval_required"] = *req.ApprovalRequired
	}
	if req.ApproverIDs != nil {
		b, _ := json.Marshal(req.ApproverIDs)
		updates["approver_ids"] = string(b)
	}
	if req.SmokeTestStepName != nil {
		updates["smoke_test_step_name"] = *req.SmokeTestStepName
	}
	if req.NotifyChannelIDs != nil {
		b, _ := json.Marshal(req.NotifyChannelIDs)
		updates["notify_channel_ids"] = string(b)
	}
	if req.Variables != nil {
		b, _ := json.Marshal(req.Variables)
		updates["variables_json"] = string(b)
	}

	if len(updates) == 0 {
		response.BadRequest(c, "no fields to update")
		return
	}

	if err := h.envSvc.UpdateEnvironment(c.Request.Context(), uint(envID), updates); err != nil {
		logger.Error("failed to update environment", "environment_id", envID, "error", err)
		response.InternalError(c, "failed to update environment: "+err.Error())
		return
	}

	logger.Info("environment updated", "environment_id", envID)
	response.OK(c, gin.H{"message": "updated"})
}

// Delete 刪除 Environment。
// DELETE /clusters/:clusterID/pipelines/:pipelineID/environments/:envID
func (h *EnvironmentHandler) Delete(c *gin.Context) {
	envID, err := strconv.ParseUint(c.Param("envID"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid environment ID")
		return
	}

	if err := h.envSvc.DeleteEnvironment(c.Request.Context(), uint(envID)); err != nil {
		logger.Error("failed to delete environment", "environment_id", envID, "error", err)
		response.InternalError(c, "failed to delete environment: "+err.Error())
		return
	}

	logger.Info("environment deleted", "environment_id", envID)
	response.OK(c, gin.H{"message": "deleted"})
}
