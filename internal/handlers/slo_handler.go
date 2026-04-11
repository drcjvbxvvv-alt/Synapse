package handlers

import (
	"context"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shaia/Synapse/internal/k8s"
	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/pkg/logger"
	"k8s.io/client-go/dynamic"
)

// SLOHandler manages SLO CRUD and status endpoints.
type SLOHandler struct {
	clusterService *services.ClusterService
	sloSvc         *services.SLOService
	// Optional: when set, GetSLOStatus annotates chaos_active flag.
	k8sMgr   *k8s.ClusterInformerManager
	chaosSvc *services.ChaosService
}

// NewSLOHandler wires dependencies.
func NewSLOHandler(
	clusterSvc *services.ClusterService,
	sloSvc *services.SLOService,
	k8sMgr *k8s.ClusterInformerManager,
	chaosSvc *services.ChaosService,
) *SLOHandler {
	return &SLOHandler{
		clusterService: clusterSvc,
		sloSvc:         sloSvc,
		k8sMgr:         k8sMgr,
		chaosSvc:       chaosSvc,
	}
}

// ── Request / Response DTOs ──────────────────────────────────────────────────

// CreateSLORequest is the body for POST /slos.
type CreateSLORequest struct {
	Name             string  `json:"name"               binding:"required,max=255"`
	Description      string  `json:"description"`
	Namespace        string  `json:"namespace"`
	SLIType          string  `json:"sli_type"           binding:"required,oneof=availability latency error_rate custom"`
	PromQuery        string  `json:"prom_query"         binding:"required"`
	TotalQuery       string  `json:"total_query"`
	Target           float64 `json:"target"             binding:"required,min=0.001,max=0.9999"`
	Window           string  `json:"window"             binding:"required,oneof=7d 28d 30d"`
	BurnRateWarning  float64 `json:"burn_rate_warning"`
	BurnRateCritical float64 `json:"burn_rate_critical"`
	Enabled          bool    `json:"enabled"`
}

// UpdateSLORequest is the body for PUT /slos/:id.
type UpdateSLORequest struct {
	Name             string  `json:"name"               binding:"required,max=255"`
	Description      string  `json:"description"`
	Namespace        string  `json:"namespace"`
	SLIType          string  `json:"sli_type"           binding:"required,oneof=availability latency error_rate custom"`
	PromQuery        string  `json:"prom_query"         binding:"required"`
	TotalQuery       string  `json:"total_query"`
	Target           float64 `json:"target"             binding:"required,min=0.001,max=0.9999"`
	Window           string  `json:"window"             binding:"required,oneof=7d 28d 30d"`
	BurnRateWarning  float64 `json:"burn_rate_warning"`
	BurnRateCritical float64 `json:"burn_rate_critical"`
	Enabled          bool    `json:"enabled"`
}

// ── Handlers ──────────────────────────────────────────────────────────────────

// ListSLOs returns all SLOs for a cluster.
// GET /clusters/:clusterID/slos?namespace=default
func (h *SLOHandler) ListSLOs(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "invalid cluster ID")
		return
	}
	namespace := c.DefaultQuery("namespace", "")

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	items, err := h.sloSvc.ListSLOs(ctx, clusterID, namespace)
	if err != nil {
		logger.Error("list SLOs failed", "cluster_id", clusterID, "error", err)
		response.InternalError(c, "failed to list SLOs: "+err.Error())
		return
	}
	response.List(c, items, int64(len(items)))
}

// GetSLO returns a single SLO.
// GET /clusters/:clusterID/slos/:id
func (h *SLOHandler) GetSLO(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "invalid cluster ID")
		return
	}
	sloID, err := parseSLOID(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid SLO ID")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	slo, err := h.sloSvc.GetSLO(ctx, clusterID, sloID)
	if err != nil {
		response.NotFound(c, "SLO not found")
		return
	}
	response.OK(c, slo)
}

// CreateSLO creates a new SLO.
// POST /clusters/:clusterID/slos
func (h *SLOHandler) CreateSLO(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "invalid cluster ID")
		return
	}

	var req CreateSLORequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body: "+err.Error())
		return
	}

	// default burn-rate thresholds
	if req.BurnRateWarning == 0 {
		req.BurnRateWarning = 2
	}
	if req.BurnRateCritical == 0 {
		req.BurnRateCritical = 10
	}

	slo := &models.SLO{
		ClusterID:        clusterID,
		Name:             req.Name,
		Description:      req.Description,
		Namespace:        req.Namespace,
		SLIType:          req.SLIType,
		PromQuery:        req.PromQuery,
		TotalQuery:       req.TotalQuery,
		Target:           req.Target,
		Window:           req.Window,
		BurnRateWarning:  req.BurnRateWarning,
		BurnRateCritical: req.BurnRateCritical,
		Enabled:          req.Enabled,
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	logger.Info("creating SLO", "cluster_id", clusterID, "name", req.Name)
	if err := h.sloSvc.CreateSLO(ctx, slo); err != nil {
		response.InternalError(c, "failed to create SLO: "+err.Error())
		return
	}
	response.OK(c, slo)
}

// UpdateSLO updates an existing SLO.
// PUT /clusters/:clusterID/slos/:id
func (h *SLOHandler) UpdateSLO(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "invalid cluster ID")
		return
	}
	sloID, err := parseSLOID(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid SLO ID")
		return
	}

	var req UpdateSLORequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body: "+err.Error())
		return
	}

	updates := &models.SLO{
		Name:             req.Name,
		Description:      req.Description,
		Namespace:        req.Namespace,
		SLIType:          req.SLIType,
		PromQuery:        req.PromQuery,
		TotalQuery:       req.TotalQuery,
		Target:           req.Target,
		Window:           req.Window,
		BurnRateWarning:  req.BurnRateWarning,
		BurnRateCritical: req.BurnRateCritical,
		Enabled:          req.Enabled,
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	logger.Info("updating SLO", "cluster_id", clusterID, "id", sloID)
	updated, err := h.sloSvc.UpdateSLO(ctx, clusterID, sloID, updates)
	if err != nil {
		response.NotFound(c, "SLO not found or update failed: "+err.Error())
		return
	}
	response.OK(c, updated)
}

// DeleteSLO soft-deletes an SLO.
// DELETE /clusters/:clusterID/slos/:id
func (h *SLOHandler) DeleteSLO(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "invalid cluster ID")
		return
	}
	sloID, err := parseSLOID(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid SLO ID")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	logger.Info("deleting SLO", "cluster_id", clusterID, "id", sloID)
	if err := h.sloSvc.DeleteSLO(ctx, clusterID, sloID); err != nil {
		response.NotFound(c, "SLO not found: "+err.Error())
		return
	}
	response.OK(c, gin.H{"message": "deleted"})
}

// GetSLOStatus queries Prometheus and returns the live SLI + burn-rate status.
// GET /clusters/:clusterID/slos/:id/status
func (h *SLOHandler) GetSLOStatus(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "invalid cluster ID")
		return
	}
	sloID, err := parseSLOID(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid SLO ID")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()

	st, err := h.sloSvc.GetSLOStatus(ctx, clusterID, sloID)
	if err != nil {
		response.NotFound(c, "SLO not found: "+err.Error())
		return
	}

	// ── Chaos check (best-effort, never blocks SLO response) ──────────────
	if h.k8sMgr != nil && h.chaosSvc != nil && st.HasData {
		slo, sloErr := h.sloSvc.GetSLO(ctx, clusterID, sloID)
		if sloErr == nil && slo.Namespace != "" {
			cluster, clErr := h.clusterService.GetCluster(clusterID)
			if clErr == nil {
				k8sClient, k8sErr := h.k8sMgr.GetK8sClient(cluster)
				if k8sErr == nil {
					dyn, dynErr := dynamic.NewForConfig(k8sClient.GetRestConfig())
					if dynErr == nil {
						chaosCtx, chaosCancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
						defer chaosCancel()
						st.ChaosActive = h.chaosSvc.HasActiveExperiments(chaosCtx, dyn, slo.Namespace)
					}
				}
			}
		}
	}

	response.OK(c, st)
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func parseSLOID(s string) (uint, error) {
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, err
	}
	return uint(v), nil
}
