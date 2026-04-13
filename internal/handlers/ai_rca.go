package handlers

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/shaia/Synapse/internal/k8s"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/pkg/logger"
)

// AIRCAHandler AI 根因分析處理器
type AIRCAHandler struct {
	clusterService *services.ClusterService
	k8sMgr         *k8s.ClusterInformerManager
	rcaSvc         *services.RCAService
}

// NewAIRCAHandler 建立 AI RCA 處理器
func NewAIRCAHandler(
	clusterSvc *services.ClusterService,
	k8sMgr *k8s.ClusterInformerManager,
	rcaSvc *services.RCAService,
) *AIRCAHandler {
	return &AIRCAHandler{
		clusterService: clusterSvc,
		k8sMgr:         k8sMgr,
		rcaSvc:         rcaSvc,
	}
}

// AnalyzePod 對 Pod 進行根因分析
// POST /clusters/:clusterID/ai/rca
func (h *AIRCAHandler) AnalyzePod(c *gin.Context) {
	// Step 1: Parse params
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "invalid cluster ID")
		return
	}

	var req struct {
		Namespace string `json:"namespace" binding:"required"`
		PodName   string `json:"pod_name"  binding:"required"`
		Language  string `json:"language"`  // e.g. "Traditional Chinese", "English" — optional
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body: "+err.Error())
		return
	}

	// Step 2: Resolve cluster
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

	// Step 3: Context with timeout (60s for AI call)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	// Step 4: Call RCA service
	logger.Info("AI RCA: analyzing pod",
		"cluster_id", clusterID,
		"namespace", req.Namespace,
		"pod", req.PodName,
	)

	result, err := h.rcaSvc.AnalyzePod(ctx, k8sClient.GetClientset(), req.Namespace, req.PodName, req.Language)
	if err != nil {
		logger.Error("AI RCA analysis failed",
			"error", err,
			"cluster_id", clusterID,
			"namespace", req.Namespace,
			"pod", req.PodName,
		)
		response.InternalError(c, "RCA analysis failed: "+err.Error())
		return
	}

	// Step 5: Response
	response.OK(c, result)
}
