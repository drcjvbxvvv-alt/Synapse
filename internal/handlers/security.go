package handlers

import (
	"context"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/shaia/Synapse/internal/k8s"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
)

// SecurityHandler handles security scanning endpoints.
type SecurityHandler struct {
	trivy *services.TrivyService
	bench *services.BenchService
	k8sMgr *k8s.ClusterInformerManager
}

func NewSecurityHandler(trivy *services.TrivyService, bench *services.BenchService, k8sMgr *k8s.ClusterInformerManager) *SecurityHandler {
	return &SecurityHandler{trivy: trivy, bench: bench, k8sMgr: k8sMgr}
}

// --- Image Scanning ---

// TriggerScan POST /clusters/:clusterID/security/scans
func (h *SecurityHandler) TriggerScan(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}

	var req struct {
		Namespace     string `json:"namespace"`
		PodName       string `json:"pod_name"`
		ContainerName string `json:"container_name"`
		Image         string `json:"image" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "請求參數錯誤："+err.Error())
		return
	}

	record, err := h.trivy.TriggerScan(clusterID, req.Namespace, req.PodName, req.ContainerName, req.Image)
	if err != nil {
		response.InternalError(c, "掃描觸發失敗："+err.Error())
		return
	}
	response.OK(c, record)
}

// IngestScan POST /clusters/:clusterID/security/scans/ingest
// 接收外部 CI（GitLab CI / GitHub Actions 等）推送的 Trivy 掃描結果。
func (h *SecurityHandler) IngestScan(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "invalid cluster ID")
		return
	}

	var req services.IngestScanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	record, err := h.trivy.IngestScanResult(ctx, clusterID, &req)
	if err != nil {
		response.InternalError(c, "ingest scan result failed: "+err.Error())
		return
	}
	response.OK(c, record)
}

// GetScanResults GET /clusters/:clusterID/security/scans?namespace=xxx
func (h *SecurityHandler) GetScanResults(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}
	namespace := c.Query("namespace")
	results, err := h.trivy.GetScanResults(clusterID, namespace)
	if err != nil {
		response.InternalError(c, "查詢掃描結果失敗："+err.Error())
		return
	}
	response.OK(c, results)
}

// GetScanDetail GET /clusters/:clusterID/security/scans/:scanID
func (h *SecurityHandler) GetScanDetail(c *gin.Context) {
	scanIDStr := c.Param("scanID")
	scanID, err := strconv.ParseUint(scanIDStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "無效的掃描 ID")
		return
	}
	record, err := h.trivy.GetScanDetail(uint(scanID))
	if err != nil {
		response.NotFound(c, "掃描記錄不存在")
		return
	}
	response.OK(c, record)
}

// --- CIS Benchmark ---

// TriggerBenchmark POST /clusters/:clusterID/security/bench
func (h *SecurityHandler) TriggerBenchmark(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}
	record, err := h.bench.TriggerBenchmark(clusterID)
	if err != nil {
		response.InternalError(c, "基準測試觸發失敗："+err.Error())
		return
	}
	response.OK(c, record)
}

// GetBenchResults GET /clusters/:clusterID/security/bench
func (h *SecurityHandler) GetBenchResults(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}
	records, err := h.bench.GetBenchResults(clusterID)
	if err != nil {
		response.InternalError(c, "查詢基準測試結果失敗："+err.Error())
		return
	}
	response.OK(c, records)
}

// GetBenchDetail GET /clusters/:clusterID/security/bench/:benchID
func (h *SecurityHandler) GetBenchDetail(c *gin.Context) {
	benchIDStr := c.Param("benchID")
	benchID, err := strconv.ParseUint(benchIDStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "無效的基準測試 ID")
		return
	}
	record, err := h.bench.GetBenchDetail(uint(benchID))
	if err != nil {
		response.NotFound(c, "基準測試記錄不存在")
		return
	}
	response.OK(c, record)
}

// --- Gatekeeper ---

// GetGatekeeperViolations GET /clusters/:clusterID/security/gatekeeper
func (h *SecurityHandler) GetGatekeeperViolations(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}
	k8sClient := h.k8sMgr.GetK8sClientByID(clusterID)
	if k8sClient == nil {
		response.BadRequest(c, "叢集連線不可用")
		return
	}
	summary, err := services.GetGatekeeperViolations(k8sClient)
	if err != nil {
		response.InternalError(c, "查詢 Gatekeeper 違規失敗："+err.Error())
		return
	}
	response.OK(c, summary)
}
