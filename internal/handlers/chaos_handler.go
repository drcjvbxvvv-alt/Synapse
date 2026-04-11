package handlers

import (
	"context"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/dynamic"

	"github.com/shaia/Synapse/internal/k8s"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/pkg/logger"
)

// ChaosHandler manages Chaos Mesh experiment endpoints.
type ChaosHandler struct {
	clusterService *services.ClusterService
	k8sMgr         *k8s.ClusterInformerManager
	chaosSvc       *services.ChaosService
}

// NewChaosHandler wires dependencies.
func NewChaosHandler(
	clusterSvc *services.ClusterService,
	k8sMgr *k8s.ClusterInformerManager,
	chaosSvc *services.ChaosService,
) *ChaosHandler {
	return &ChaosHandler{
		clusterService: clusterSvc,
		k8sMgr:         k8sMgr,
		chaosSvc:       chaosSvc,
	}
}

// ── Shared helpers ────────────────────────────────────────────────────────────

func (h *ChaosHandler) resolveDyn(c *gin.Context) (dynamic.Interface, uint, bool) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "invalid cluster ID")
		return nil, 0, false
	}
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "cluster not found")
		return nil, 0, false
	}
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, "failed to get K8s client: "+err.Error())
		return nil, 0, false
	}
	dyn, err := dynamic.NewForConfig(k8sClient.GetRestConfig())
	if err != nil {
		response.InternalError(c, "failed to build dynamic client: "+err.Error())
		return nil, 0, false
	}
	return dyn, clusterID, true
}

// ── Handlers ──────────────────────────────────────────────────────────────────

// GetChaosStatus checks whether Chaos Mesh is installed.
// GET /clusters/:clusterID/chaos/status
func (h *ChaosHandler) GetChaosStatus(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "invalid cluster ID")
		return
	}
	cluster, err := h.clusterService.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "cluster not found")
		return
	}
	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, "failed to get K8s client: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	status := h.chaosSvc.IsChaosMeshInstalled(ctx, k8sClient.GetClientset())
	response.OK(c, status)
}

// ListExperiments lists all Chaos Mesh experiments across all CRD types.
// GET /clusters/:clusterID/chaos/experiments?namespace=default
func (h *ChaosHandler) ListExperiments(c *gin.Context) {
	dyn, _, ok := h.resolveDyn(c)
	if !ok {
		return
	}
	namespace := c.DefaultQuery("namespace", "")

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	items, err := h.chaosSvc.ListExperiments(ctx, dyn, namespace)
	if err != nil {
		response.InternalError(c, "failed to list experiments: "+err.Error())
		return
	}
	if items == nil {
		items = []services.ChaosExperiment{}
	}
	response.List(c, items, int64(len(items)))
}

// GetExperiment fetches a single experiment.
// GET /clusters/:clusterID/chaos/experiments/:namespace/:kind/:name
func (h *ChaosHandler) GetExperiment(c *gin.Context) {
	dyn, _, ok := h.resolveDyn(c)
	if !ok {
		return
	}
	namespace := c.Param("namespace")
	kind := c.Param("kind")
	name := c.Param("name")

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	obj, err := h.chaosSvc.GetExperiment(ctx, dyn, kind, namespace, name)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			response.NotFound(c, "experiment not found")
			return
		}
		response.InternalError(c, "get experiment failed: "+err.Error())
		return
	}
	response.OK(c, obj.Object)
}

// CreateExperiment creates a new Chaos Mesh experiment.
// POST /clusters/:clusterID/chaos/experiments
func (h *ChaosHandler) CreateExperiment(c *gin.Context) {
	dyn, clusterID, ok := h.resolveDyn(c)
	if !ok {
		return
	}

	var req services.CreateChaosRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	logger.Info("creating chaos experiment",
		"cluster_id", clusterID,
		"kind", req.Kind,
		"namespace", req.Namespace,
		"name", req.Name,
	)
	exp, err := h.chaosSvc.CreateExperiment(ctx, dyn, req)
	if err != nil {
		if k8serrors.IsAlreadyExists(err) {
			response.BadRequest(c, "experiment already exists: "+req.Name)
			return
		}
		response.InternalError(c, "create experiment failed: "+err.Error())
		return
	}
	response.OK(c, exp)
}

// DeleteExperiment deletes a chaos experiment.
// DELETE /clusters/:clusterID/chaos/experiments/:namespace/:kind/:name
func (h *ChaosHandler) DeleteExperiment(c *gin.Context) {
	dyn, clusterID, ok := h.resolveDyn(c)
	if !ok {
		return
	}
	namespace := c.Param("namespace")
	kind := c.Param("kind")
	name := c.Param("name")

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	logger.Info("deleting chaos experiment",
		"cluster_id", clusterID,
		"kind", kind,
		"namespace", namespace,
		"name", name,
	)
	if err := h.chaosSvc.DeleteExperiment(ctx, dyn, kind, namespace, name); err != nil {
		if k8serrors.IsNotFound(err) || isChaosNotFound(err) {
			response.NotFound(c, "experiment not found")
			return
		}
		response.InternalError(c, "delete experiment failed: "+err.Error())
		return
	}
	response.OK(c, gin.H{"message": "deleted"})
}

// ListSchedules lists Chaos Mesh scheduled experiments.
// GET /clusters/:clusterID/chaos/schedules?namespace=default
func (h *ChaosHandler) ListSchedules(c *gin.Context) {
	dyn, _, ok := h.resolveDyn(c)
	if !ok {
		return
	}
	namespace := c.DefaultQuery("namespace", "")

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	items, err := h.chaosSvc.ListSchedules(ctx, dyn, namespace)
	if err != nil {
		if isChaosNotFound(err) {
			response.OK(c, gin.H{"items": []interface{}{}, "total": 0})
			return
		}
		response.InternalError(c, "list schedules failed: "+err.Error())
		return
	}
	if items == nil {
		items = []services.ChaosSchedule{}
	}
	response.List(c, items, int64(len(items)))
}

// CreateSchedule creates a Chaos Mesh Schedule CRD.
// POST /clusters/:clusterID/chaos/schedules
func (h *ChaosHandler) CreateSchedule(c *gin.Context) {
	dyn, clusterID, ok := h.resolveDyn(c)
	if !ok {
		return
	}

	var req services.CreateScheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	logger.Info("creating chaos schedule",
		"cluster_id", clusterID,
		"namespace", req.Namespace,
		"name", req.Name,
		"cron", req.CronExpr,
	)
	sched, err := h.chaosSvc.CreateSchedule(ctx, dyn, req)
	if err != nil {
		if k8serrors.IsAlreadyExists(err) {
			response.BadRequest(c, "schedule already exists: "+req.Name)
			return
		}
		response.InternalError(c, "create schedule failed: "+err.Error())
		return
	}
	response.OK(c, sched)
}

// DeleteSchedule deletes a Chaos Mesh Schedule CRD.
// DELETE /clusters/:clusterID/chaos/schedules/:namespace/:name
func (h *ChaosHandler) DeleteSchedule(c *gin.Context) {
	dyn, clusterID, ok := h.resolveDyn(c)
	if !ok {
		return
	}
	namespace := c.Param("namespace")
	name := c.Param("name")

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	logger.Info("deleting chaos schedule",
		"cluster_id", clusterID,
		"namespace", namespace,
		"name", name,
	)
	if err := h.chaosSvc.DeleteSchedule(ctx, dyn, namespace, name); err != nil {
		if k8serrors.IsNotFound(err) || isChaosNotFound(err) {
			response.NotFound(c, "schedule not found")
			return
		}
		response.InternalError(c, "delete schedule failed: "+err.Error())
		return
	}
	response.OK(c, gin.H{"message": "deleted"})
}

// isChaosNotFound returns true when the Chaos Mesh CRD is absent or the resource was not found.
func isChaosNotFound(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "the server could not find the requested resource") ||
		strings.Contains(msg, "no kind is registered") ||
		k8serrors.IsNotFound(err)
}
