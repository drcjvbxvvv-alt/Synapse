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

// SecurityAuditHandler 安全審計處理器
type SecurityAuditHandler struct {
	clusterService *services.ClusterService
	k8sMgr         *k8s.ClusterInformerManager
	auditSvc       *services.SecurityAuditService
}

// NewSecurityAuditHandler 建立安全審計處理器
func NewSecurityAuditHandler(
	clusterSvc *services.ClusterService,
	k8sMgr *k8s.ClusterInformerManager,
	auditSvc *services.SecurityAuditService,
) *SecurityAuditHandler {
	return &SecurityAuditHandler{
		clusterService: clusterSvc,
		k8sMgr:         k8sMgr,
		auditSvc:       auditSvc,
	}
}

// ScanSecretSprawl 掃描 Secret 蔓延
// GET /clusters/:clusterID/security/secret-sprawl?namespace=default
func (h *SecurityAuditHandler) ScanSecretSprawl(c *gin.Context) {
	// Step 1: Parse params
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "invalid cluster ID")
		return
	}
	namespace := c.DefaultQuery("namespace", "")

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

	// Step 3: Context with timeout (60s for potentially large clusters)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	// Step 4: Call service
	logger.Info("security audit: scanning secret sprawl",
		"cluster_id", clusterID,
		"namespace", namespace,
	)

	report, err := h.auditSvc.ScanSecretSprawl(ctx, k8sClient.GetClientset(), namespace)
	if err != nil {
		logger.Error("secret sprawl scan failed",
			"error", err,
			"cluster_id", clusterID,
		)
		response.InternalError(c, "secret sprawl scan failed: "+err.Error())
		return
	}

	// Step 5: Response
	response.OK(c, report)
}
