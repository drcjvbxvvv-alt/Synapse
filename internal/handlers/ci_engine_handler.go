// Package handlers — HTTP handlers for the CI engine adapter subsystem (M18a+M19b).
package handlers

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/shaia/Synapse/internal/apierrors"
	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/internal/services/pipeline/engine"
	"github.com/shaia/Synapse/pkg/logger"
)

// CIEngineHandler serves /api/v1/ci-engines endpoints.
type CIEngineHandler struct {
	svc *services.CIEngineService
}

// NewCIEngineHandler wires dependencies for CIEngineHandler.
func NewCIEngineHandler(svc *services.CIEngineService) *CIEngineHandler {
	return &CIEngineHandler{svc: svc}
}

// ---------------------------------------------------------------------------
// Probing
// ---------------------------------------------------------------------------

// ListAvailable returns the status of every registered engine (native + external).
// GET /api/v1/ci-engines/status
func (h *CIEngineHandler) ListAvailable(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	items, err := h.svc.ListAvailableEngines(ctx)
	if err != nil {
		logger.Error("list available ci engines failed", "error", err)
		response.InternalError(c, "failed to list engines: "+err.Error())
		return
	}
	response.List(c, items, int64(len(items)))
}

// ---------------------------------------------------------------------------
// CRUD on external engine configs
// ---------------------------------------------------------------------------

// List returns all external CI engine connection profiles.
// GET /api/v1/ci-engines
func (h *CIEngineHandler) List(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	items, err := h.svc.List(ctx)
	if err != nil {
		logger.Error("list ci engine configs failed", "error", err)
		response.InternalError(c, "failed to list configs: "+err.Error())
		return
	}
	response.List(c, items, int64(len(items)))
}

// Get returns a single engine config by id.
// GET /api/v1/ci-engines/:id
func (h *CIEngineHandler) Get(c *gin.Context) {
	id, err := parseCIEngineID(c.Param("id"))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	cfg, err := h.svc.Get(ctx, id)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	response.OK(c, cfg)
}

// Create persists a new engine config.
// POST /api/v1/ci-engines
func (h *CIEngineHandler) Create(c *gin.Context) {
	var req models.CIEngineConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body: "+err.Error())
		return
	}
	userID := c.GetUint("user_id")

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	logger.Info("ci engine config create requested",
		"engine_type", req.EngineType,
		"name", req.Name,
		"user_id", userID,
	)

	cfg, err := h.svc.Create(ctx, &req, userID)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	response.Created(c, cfg)
}

// Update replaces a config. Empty credential fields preserve existing values.
// PUT /api/v1/ci-engines/:id
func (h *CIEngineHandler) Update(c *gin.Context) {
	id, err := parseCIEngineID(c.Param("id"))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	var req models.CIEngineConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body: "+err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	logger.Info("ci engine config update requested",
		"id", id,
		"engine_type", req.EngineType,
		"user_id", c.GetUint("user_id"),
	)

	cfg, err := h.svc.Update(ctx, id, &req)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	response.OK(c, cfg)
}

// Delete removes a config.
// DELETE /api/v1/ci-engines/:id
func (h *CIEngineHandler) Delete(c *gin.Context) {
	id, err := parseCIEngineID(c.Param("id"))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	logger.Info("ci engine config delete requested", "id", id, "user_id", c.GetUint("user_id"))

	if err := h.svc.Delete(ctx, id); err != nil {
		writeServiceError(c, err)
		return
	}
	response.NoContent(c)
}

// ---------------------------------------------------------------------------
// Run operations
// ---------------------------------------------------------------------------

// TriggerRun starts a new run on the engine identified by :id.
// POST /api/v1/ci-engines/:id/runs
func (h *CIEngineHandler) TriggerRun(c *gin.Context) {
	id, err := parseCIEngineID(c.Param("id"))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	var req engine.TriggerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body: "+err.Error())
		return
	}
	req.TriggeredByUser = c.GetUint("user_id")

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	logger.Info("ci engine trigger run requested", "config_id", id, "ref", req.Ref)

	res, err := h.svc.TriggerRun(ctx, id, &req)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	response.Created(c, res)
}

// GetRun returns the current status of a run.
// GET /api/v1/ci-engines/:id/runs/:runId
func (h *CIEngineHandler) GetRun(c *gin.Context) {
	id, err := parseCIEngineID(c.Param("id"))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	runID := c.Param("runId")

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	status, err := h.svc.GetRun(ctx, id, runID)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	response.OK(c, status)
}

// CancelRun requests cancellation of a run.
// DELETE /api/v1/ci-engines/:id/runs/:runId
func (h *CIEngineHandler) CancelRun(c *gin.Context) {
	id, err := parseCIEngineID(c.Param("id"))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	runID := c.Param("runId")

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	logger.Info("ci engine cancel run requested", "config_id", id, "run_id", runID)

	if err := h.svc.CancelRun(ctx, id, runID); err != nil {
		writeServiceError(c, err)
		return
	}
	response.NoContent(c)
}

// StreamLogs streams logs for a run's step.
// GET /api/v1/ci-engines/:id/runs/:runId/logs?step=<stepID>
//
// The response body is plain text (the raw log bytes). An empty ?step= means
// "auto-select first step / whole-run aggregate" depending on the adapter.
func (h *CIEngineHandler) StreamLogs(c *gin.Context) {
	id, err := parseCIEngineID(c.Param("id"))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	runID := c.Param("runId")
	stepID := c.Query("step")

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Minute)
	defer cancel()

	rc, err := h.svc.StreamLogs(ctx, id, runID, stepID)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	defer rc.Close()

	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Status(http.StatusOK)
	if _, copyErr := io.Copy(c.Writer, rc); copyErr != nil {
		logger.Warn("ci engine stream logs: copy interrupted",
			"config_id", id, "run_id", runID, "error", copyErr)
	}
}

// GetArtifacts returns the artifact list for a completed run.
// GET /api/v1/ci-engines/:id/runs/:runId/artifacts
func (h *CIEngineHandler) GetArtifacts(c *gin.Context) {
	id, err := parseCIEngineID(c.Param("id"))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	runID := c.Param("runId")

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	arts, err := h.svc.GetArtifacts(ctx, id, runID)
	if err != nil {
		writeServiceError(c, err)
		return
	}
	response.List(c, arts, int64(len(arts)))
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// parseCIEngineID validates the :id path parameter.
func parseCIEngineID(raw string) (uint, error) {
	n, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || n == 0 {
		return 0, &apierrors.AppError{
			Code:       "CI_ENGINE_ID_INVALID",
			HTTPStatus: 400,
			Message:    "invalid ci engine id",
		}
	}
	return uint(n), nil
}

// writeServiceError translates a service-layer error into an HTTP response.
// Known AppError codes retain their status; everything else becomes 500.
func writeServiceError(c *gin.Context, err error) {
	if ae, ok := apierrors.As(err); ok {
		response.Error(c, ae.HTTPStatus, ae.Code, ae.Message)
		return
	}
	logger.Error("ci engine handler unexpected error", "error", err)
	response.InternalError(c, err.Error())
}
