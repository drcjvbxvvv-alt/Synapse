package handlers

import (
	"context"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shaia/Synapse/internal/response"
	"github.com/shaia/Synapse/internal/services"
	"github.com/shaia/Synapse/pkg/logger"
)

// ComplianceHandler manages compliance reports, evidence, and violation timeline.
type ComplianceHandler struct {
	complianceSvc *services.ComplianceService
}

// NewComplianceHandler creates a ComplianceHandler.
func NewComplianceHandler(complianceSvc *services.ComplianceService) *ComplianceHandler {
	return &ComplianceHandler{complianceSvc: complianceSvc}
}

// ─── Reports ───────────────────────────────────────────────────────────────

// GenerateReport triggers compliance report generation.
// POST /clusters/:clusterID/compliance/reports
func (h *ComplianceHandler) GenerateReport(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "invalid cluster ID")
		return
	}

	var req services.GenerateReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body: "+err.Error())
		return
	}
	req.UserID = c.GetUint("user_id")

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	logger.Info("generating compliance report",
		"cluster_id", clusterID,
		"framework", req.Framework,
		"user_id", req.UserID,
	)

	report, err := h.complianceSvc.GenerateReport(ctx, clusterID, req)
	if err != nil {
		response.InternalError(c, "failed to generate report: "+err.Error())
		return
	}
	response.Created(c, report)
}

// ListReports lists compliance reports for a cluster.
// GET /clusters/:clusterID/compliance/reports?framework=SOC2
func (h *ComplianceHandler) ListReports(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "invalid cluster ID")
		return
	}
	framework := c.Query("framework")

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	reports, err := h.complianceSvc.ListReports(ctx, clusterID, framework)
	if err != nil {
		response.InternalError(c, "failed to list reports: "+err.Error())
		return
	}
	response.OK(c, reports)
}

// GetReport returns a single compliance report with full results.
// GET /clusters/:clusterID/compliance/reports/:id
func (h *ComplianceHandler) GetReport(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "invalid cluster ID")
		return
	}
	reportID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid report ID")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	report, err := h.complianceSvc.GetReport(ctx, clusterID, uint(reportID))
	if err != nil {
		response.NotFound(c, "report not found")
		return
	}
	response.OK(c, report)
}

// ExportReport exports a compliance report as JSON.
// GET /clusters/:clusterID/compliance/reports/:id/export
func (h *ComplianceHandler) ExportReport(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "invalid cluster ID")
		return
	}
	reportID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid report ID")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	report, err := h.complianceSvc.GetReport(ctx, clusterID, uint(reportID))
	if err != nil {
		response.NotFound(c, "report not found")
		return
	}

	c.Header("Content-Disposition", "attachment; filename=compliance-report-"+report.Framework+".json")
	c.Header("Content-Type", "application/json")
	response.OK(c, report)
}

// DeleteReport deletes a compliance report.
// DELETE /clusters/:clusterID/compliance/reports/:id
func (h *ComplianceHandler) DeleteReport(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "invalid cluster ID")
		return
	}
	reportID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid report ID")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	logger.Info("deleting compliance report",
		"cluster_id", clusterID,
		"report_id", reportID,
	)

	if err := h.complianceSvc.DeleteReport(ctx, clusterID, uint(reportID)); err != nil {
		response.InternalError(c, "failed to delete report: "+err.Error())
		return
	}
	response.OK(c, gin.H{"message": "deleted"})
}

// ─── Violations ────────────────────────────────────────────────────────────

// ListViolations returns the violation event timeline.
// GET /clusters/:clusterID/compliance/violations?source=trivy&severity=critical&page=1&pageSize=20
func (h *ComplianceHandler) ListViolations(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "invalid cluster ID")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	filter := services.ViolationFilter{
		Source:   c.Query("source"),
		Severity: c.Query("severity"),
	}
	if c.Query("resolved") == "true" {
		t := true
		filter.Resolved = &t
	} else if c.Query("resolved") == "false" {
		f := false
		filter.Resolved = &f
	}
	if since := c.Query("since"); since != "" {
		if t, err := time.Parse(time.RFC3339, since); err == nil {
			filter.Since = &t
		}
	}
	if until := c.Query("until"); until != "" {
		if t, err := time.Parse(time.RFC3339, until); err == nil {
			filter.Until = &t
		}
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	events, total, err := h.complianceSvc.ListViolations(ctx, clusterID, filter, page, pageSize)
	if err != nil {
		response.InternalError(c, "failed to list violations: "+err.Error())
		return
	}
	response.PagedList(c, events, total, page, pageSize)
}

// GetViolationStats returns aggregated violation statistics.
// GET /clusters/:clusterID/compliance/violations/stats
func (h *ComplianceHandler) GetViolationStats(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "invalid cluster ID")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	stats, err := h.complianceSvc.GetViolationStats(ctx, clusterID)
	if err != nil {
		response.InternalError(c, "failed to get violation stats: "+err.Error())
		return
	}
	response.OK(c, stats)
}

// ResolveViolation marks a violation as resolved.
// PUT /clusters/:clusterID/compliance/violations/:id/resolve
func (h *ComplianceHandler) ResolveViolation(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "invalid cluster ID")
		return
	}
	violationID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid violation ID")
		return
	}

	username := c.GetString("username")

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	logger.Info("resolving violation",
		"cluster_id", clusterID,
		"violation_id", violationID,
		"resolved_by", username,
	)

	if err := h.complianceSvc.ResolveViolation(ctx, clusterID, uint(violationID), username); err != nil {
		response.InternalError(c, "failed to resolve violation: "+err.Error())
		return
	}
	response.OK(c, gin.H{"message": "resolved"})
}

// ─── Evidence ──────────────────────────────────────────────────────────────

// CaptureEvidence captures compliance evidence for a control.
// POST /clusters/:clusterID/compliance/evidence
func (h *ComplianceHandler) CaptureEvidence(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "invalid cluster ID")
		return
	}

	var req struct {
		Framework    string `json:"framework" binding:"required"`
		ControlID    string `json:"control_id" binding:"required"`
		ControlTitle string `json:"control_title"`
		EvidenceType string `json:"evidence_type" binding:"required"`
		DataJSON     string `json:"data_json" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	logger.Info("capturing compliance evidence",
		"cluster_id", clusterID,
		"framework", req.Framework,
		"control_id", req.ControlID,
	)

	ev, err := h.complianceSvc.CaptureEvidence(ctx, clusterID, req.Framework, req.ControlID, req.ControlTitle, req.EvidenceType, req.DataJSON)
	if err != nil {
		response.InternalError(c, "failed to capture evidence: "+err.Error())
		return
	}
	response.Created(c, ev)
}

// ListEvidence lists captured evidence.
// GET /clusters/:clusterID/compliance/evidence?framework=SOC2&control_id=CC6.1
func (h *ComplianceHandler) ListEvidence(c *gin.Context) {
	clusterID, err := parseClusterID(c.Param("clusterID"))
	if err != nil {
		response.BadRequest(c, "invalid cluster ID")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	items, err := h.complianceSvc.ListEvidence(ctx, clusterID, c.Query("framework"), c.Query("control_id"))
	if err != nil {
		response.InternalError(c, "failed to list evidence: "+err.Error())
		return
	}
	response.OK(c, items)
}

// GetEvidence returns a single evidence item.
// GET /clusters/:clusterID/compliance/evidence/:id
func (h *ComplianceHandler) GetEvidence(c *gin.Context) {
	evidenceID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid evidence ID")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	ev, err := h.complianceSvc.GetEvidence(ctx, uint(evidenceID))
	if err != nil {
		response.NotFound(c, "evidence not found")
		return
	}
	response.OK(c, ev)
}
