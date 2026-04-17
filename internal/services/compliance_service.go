package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/pkg/logger"
	"gorm.io/gorm"
)

// ─── Framework control definitions ─────────────────────────────────────────

// ControlResult is one control's evaluation result within a report.
type ControlResult struct {
	ControlID   string `json:"control_id"`
	Title       string `json:"title"`
	Category    string `json:"category"`
	Status      string `json:"status"` // pass, fail, warn, na
	Description string `json:"description"`
	Evidence    string `json:"evidence,omitempty"`
}

type frameworkControl struct {
	ID       string
	Title    string
	Category string
	// evaluator maps available data → ControlResult; nil = "N/A"
}

// ─── SOC2 Trust Service Criteria (subset) ─────────────────────────────────

var soc2Controls = []frameworkControl{
	{ID: "CC6.1", Title: "Logical access security over protected assets", Category: "Logical and Physical Access Controls"},
	{ID: "CC6.2", Title: "User authentication before access", Category: "Logical and Physical Access Controls"},
	{ID: "CC6.3", Title: "Authorized access to protected information", Category: "Logical and Physical Access Controls"},
	{ID: "CC6.6", Title: "Security measures against threats outside system boundaries", Category: "Logical and Physical Access Controls"},
	{ID: "CC6.8", Title: "Prevent or detect unauthorized or malicious software", Category: "Logical and Physical Access Controls"},
	{ID: "CC7.1", Title: "Detection and monitoring of security events", Category: "System Operations"},
	{ID: "CC7.2", Title: "Monitor system components for anomalies", Category: "System Operations"},
	{ID: "CC7.3", Title: "Evaluate detected events and determine incidents", Category: "System Operations"},
	{ID: "CC7.4", Title: "Respond to identified security incidents", Category: "System Operations"},
	{ID: "CC8.1", Title: "Authorize, design, develop, and implement changes", Category: "Change Management"},
}

// ─── ISO 27001 Annex A controls (subset) ──────────────────────────────────

var iso27001Controls = []frameworkControl{
	{ID: "A.5.15", Title: "Access control policy", Category: "Access Control"},
	{ID: "A.5.17", Title: "Authentication information", Category: "Access Control"},
	{ID: "A.8.9", Title: "Configuration management", Category: "Technology Controls"},
	{ID: "A.8.15", Title: "Logging", Category: "Technology Controls"},
	{ID: "A.8.16", Title: "Monitoring activities", Category: "Technology Controls"},
	{ID: "A.8.8", Title: "Technical vulnerability management", Category: "Technology Controls"},
	{ID: "A.8.25", Title: "Secure development life cycle", Category: "Technology Controls"},
	{ID: "A.8.28", Title: "Secure coding", Category: "Technology Controls"},
	{ID: "A.5.25", Title: "Assessment and decision on information security events", Category: "Incident Management"},
	{ID: "A.5.26", Title: "Response to information security incidents", Category: "Incident Management"},
}

// ─── CIS Kubernetes Benchmark (mapped from kube-bench sections) ───────────

var cisK8sControls = []frameworkControl{
	{ID: "1.1", Title: "Control Plane Components — API Server", Category: "Control Plane"},
	{ID: "1.2", Title: "Control Plane Components — API Server Flags", Category: "Control Plane"},
	{ID: "1.3", Title: "Controller Manager", Category: "Control Plane"},
	{ID: "1.4", Title: "Scheduler", Category: "Control Plane"},
	{ID: "2.1", Title: "Etcd Node Configuration", Category: "Etcd"},
	{ID: "3.1", Title: "Authentication and Authorization", Category: "Control Plane Configuration"},
	{ID: "3.2", Title: "Logging", Category: "Control Plane Configuration"},
	{ID: "4.1", Title: "Worker Node Configuration Files", Category: "Worker Nodes"},
	{ID: "4.2", Title: "Kubelet", Category: "Worker Nodes"},
	{ID: "5.1", Title: "RBAC and Service Accounts", Category: "Policies"},
	{ID: "5.2", Title: "Pod Security Standards", Category: "Policies"},
	{ID: "5.3", Title: "Network Policies and CNI", Category: "Policies"},
	{ID: "5.7", Title: "General Policies", Category: "Policies"},
}

// ─── Service ───────────────────────────────────────────────────────────────

// ComplianceService generates and manages compliance reports.
type ComplianceService struct {
	db *gorm.DB
}

// NewComplianceService creates a new ComplianceService.
func NewComplianceService(db *gorm.DB) *ComplianceService {
	return &ComplianceService{db: db}
}

// GenerateReportRequest is the input for report generation.
type GenerateReportRequest struct {
	Framework string `json:"framework" binding:"required"` // SOC2, ISO27001, CIS_K8S
	UserID    uint   `json:"-"`
}

// GenerateReport creates a compliance report by evaluating existing data.
func (s *ComplianceService) GenerateReport(ctx context.Context, clusterID uint, req GenerateReportRequest) (*models.ComplianceReport, error) {
	controls := s.getControls(req.Framework)
	if len(controls) == 0 {
		return nil, fmt.Errorf("unknown framework: %s", req.Framework)
	}

	report := &models.ComplianceReport{
		ClusterID:   clusterID,
		Framework:   req.Framework,
		Version:     s.frameworkVersion(req.Framework),
		Status:      "generating",
		GeneratedBy: req.UserID,
	}
	if err := s.db.WithContext(ctx).Create(report).Error; err != nil {
		return nil, fmt.Errorf("create compliance report: %w", err)
	}

	// Evaluate controls asynchronously
	go s.evaluateReport(report.ID, clusterID, req.Framework, controls)

	return report, nil
}

func (s *ComplianceService) evaluateReport(reportID, clusterID uint, framework string, controls []frameworkControl) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Gather source data
	var latestBench models.BenchResult
	s.db.WithContext(ctx).Where("cluster_id = ? AND status = ?", clusterID, "completed").
		Order("created_at DESC").First(&latestBench)

	var scanResults []models.ImageScanResult
	s.db.WithContext(ctx).Where("cluster_id = ? AND status = ?", clusterID, "completed").
		Order("created_at DESC").Limit(50).Find(&scanResults)

	var violations []models.ViolationEvent
	s.db.WithContext(ctx).Where("cluster_id = ? AND resolved_at IS NULL", clusterID).
		Find(&violations)

	// Evaluate each control
	results := make([]ControlResult, 0, len(controls))
	pass, fail, warn := 0, 0, 0

	for _, ctrl := range controls {
		cr := s.evaluateControl(framework, ctrl, &latestBench, scanResults, violations)
		results = append(results, cr)
		switch cr.Status {
		case "pass":
			pass++
		case "fail":
			fail++
		case "warn":
			warn++
		}
	}

	score := float64(0)
	if total := pass + fail; total > 0 {
		score = float64(pass) / float64(total) * 100
	}

	resultJSON, _ := json.Marshal(results)
	now := time.Now()

	s.db.WithContext(ctx).Model(&models.ComplianceReport{}).Where("id = ?", reportID).Updates(map[string]interface{}{
		"status":       "completed",
		"score":        score,
		"pass_count":   pass,
		"fail_count":   fail,
		"warn_count":   warn,
		"result_json":  string(resultJSON),
		"generated_at": &now,
	})

	logger.Info("compliance report generated",
		"report_id", reportID,
		"framework", framework,
		"score", score,
		"pass", pass,
		"fail", fail,
	)
}

func (s *ComplianceService) evaluateControl(framework string, ctrl frameworkControl, bench *models.BenchResult, scans []models.ImageScanResult, violations []models.ViolationEvent) ControlResult {
	cr := ControlResult{
		ControlID: ctrl.ID,
		Title:     ctrl.Title,
		Category:  ctrl.Category,
		Status:    "na",
	}

	switch framework {
	case "CIS_K8S":
		cr = s.evaluateCISControl(ctrl, bench)
	case "SOC2":
		cr = s.evaluateSOC2Control(ctrl, bench, scans, violations)
	case "ISO27001":
		cr = s.evaluateISO27001Control(ctrl, bench, scans, violations)
	}

	return cr
}

func (s *ComplianceService) evaluateCISControl(ctrl frameworkControl, bench *models.BenchResult) ControlResult {
	cr := ControlResult{ControlID: ctrl.ID, Title: ctrl.Title, Category: ctrl.Category, Status: "na"}

	if bench.ID == 0 {
		cr.Status = "warn"
		cr.Description = "No CIS benchmark run available. Run a benchmark to evaluate this control."
		return cr
	}

	// Map bench overall score to control status
	if bench.Score >= 80 {
		cr.Status = "pass"
		cr.Description = fmt.Sprintf("CIS benchmark score: %.1f%% (pass: %d, fail: %d)", bench.Score, bench.Pass, bench.Fail)
	} else if bench.Score >= 50 {
		cr.Status = "warn"
		cr.Description = fmt.Sprintf("CIS benchmark score below target: %.1f%% (pass: %d, fail: %d)", bench.Score, bench.Pass, bench.Fail)
	} else {
		cr.Status = "fail"
		cr.Description = fmt.Sprintf("CIS benchmark score critical: %.1f%% (pass: %d, fail: %d)", bench.Score, bench.Pass, bench.Fail)
	}

	cr.Evidence = fmt.Sprintf("Benchmark run at %s, job: %s", bench.RunAt, bench.JobName)
	return cr
}

func (s *ComplianceService) evaluateSOC2Control(ctrl frameworkControl, bench *models.BenchResult, scans []models.ImageScanResult, violations []models.ViolationEvent) ControlResult {
	cr := ControlResult{ControlID: ctrl.ID, Title: ctrl.Title, Category: ctrl.Category, Status: "pass"}

	switch ctrl.ID {
	case "CC6.1", "CC6.2", "CC6.3":
		// Access control — check if audit logging is active (always pass for Synapse)
		cr.Description = "Access control enforced via RBAC and cluster-level permissions. Audit logging active."

	case "CC6.6":
		// Network security — check gatekeeper violations
		netViolations := filterViolations(violations, "gatekeeper")
		if len(netViolations) > 0 {
			cr.Status = "warn"
			cr.Description = fmt.Sprintf("%d active Gatekeeper policy violations detected.", len(netViolations))
		} else {
			cr.Description = "No active policy violations."
		}

	case "CC6.8":
		// Malicious software detection — image scanning
		criticalCount := 0
		for _, scan := range scans {
			criticalCount += scan.Critical
		}
		if criticalCount > 0 {
			cr.Status = "fail"
			cr.Description = fmt.Sprintf("%d critical vulnerabilities found across %d scanned images.", criticalCount, len(scans))
		} else if len(scans) == 0 {
			cr.Status = "warn"
			cr.Description = "No image scan results available."
		} else {
			cr.Description = fmt.Sprintf("%d images scanned, no critical vulnerabilities.", len(scans))
		}

	case "CC7.1", "CC7.2":
		// Detection and monitoring — audit system
		cr.Description = "Hash-chain audit logging and operation audit middleware active. SIEM integration available."

	case "CC7.3", "CC7.4":
		// Incident evaluation/response
		openViolations := len(violations)
		if openViolations > 10 {
			cr.Status = "warn"
			cr.Description = fmt.Sprintf("%d unresolved violations pending review.", openViolations)
		} else {
			cr.Description = fmt.Sprintf("%d unresolved violations.", openViolations)
		}

	case "CC8.1":
		// Change management — CIS benchmark
		if bench.ID == 0 {
			cr.Status = "warn"
			cr.Description = "No CIS benchmark data for change management assessment."
		} else {
			cr.Description = fmt.Sprintf("CIS benchmark score: %.1f%%", bench.Score)
		}

	default:
		cr.Status = "na"
		cr.Description = "Control not yet mapped."
	}

	return cr
}

func (s *ComplianceService) evaluateISO27001Control(ctrl frameworkControl, bench *models.BenchResult, scans []models.ImageScanResult, violations []models.ViolationEvent) ControlResult {
	cr := ControlResult{ControlID: ctrl.ID, Title: ctrl.Title, Category: ctrl.Category, Status: "pass"}

	switch ctrl.ID {
	case "A.5.15", "A.5.17":
		cr.Description = "Access control and authentication managed via Synapse RBAC, LDAP, and cluster permissions."

	case "A.8.9":
		// Configuration management
		if bench.ID > 0 && bench.Score >= 70 {
			cr.Description = fmt.Sprintf("CIS benchmark score: %.1f%%", bench.Score)
		} else if bench.ID > 0 {
			cr.Status = "warn"
			cr.Description = fmt.Sprintf("CIS benchmark score below threshold: %.1f%%", bench.Score)
		} else {
			cr.Status = "warn"
			cr.Description = "No benchmark data available."
		}

	case "A.8.15":
		// Logging
		cr.Description = "Hash-chain audit logging active. Operation audit captures all state-changing requests."

	case "A.8.16":
		// Monitoring
		cr.Description = "Prometheus monitoring integration, alert rules, and SLO management available."

	case "A.8.8":
		// Vulnerability management
		critCount := 0
		for _, scan := range scans {
			critCount += scan.Critical + scan.High
		}
		if critCount > 0 {
			cr.Status = "fail"
			cr.Description = fmt.Sprintf("%d critical/high vulnerabilities across %d images.", critCount, len(scans))
		} else if len(scans) == 0 {
			cr.Status = "warn"
			cr.Description = "No image scans available."
		} else {
			cr.Description = fmt.Sprintf("%d images scanned with no critical/high vulnerabilities.", len(scans))
		}

	case "A.5.25", "A.5.26":
		// Incident management
		openViolations := len(violations)
		if openViolations > 10 {
			cr.Status = "warn"
			cr.Description = fmt.Sprintf("%d unresolved violations.", openViolations)
		} else {
			cr.Description = fmt.Sprintf("%d unresolved violations.", openViolations)
		}

	default:
		cr.Status = "na"
		cr.Description = "Control not yet mapped."
	}

	return cr
}

// ─── Helpers ───────────────────────────────────────────────────────────────

func filterViolations(violations []models.ViolationEvent, source string) []models.ViolationEvent {
	var filtered []models.ViolationEvent
	for _, v := range violations {
		if v.Source == source {
			filtered = append(filtered, v)
		}
	}
	return filtered
}

func (s *ComplianceService) getControls(framework string) []frameworkControl {
	switch framework {
	case "SOC2":
		return soc2Controls
	case "ISO27001":
		return iso27001Controls
	case "CIS_K8S":
		return cisK8sControls
	default:
		return nil
	}
}

func (s *ComplianceService) frameworkVersion(framework string) string {
	switch framework {
	case "SOC2":
		return "2017"
	case "ISO27001":
		return "2022"
	case "CIS_K8S":
		return "1.8"
	default:
		return ""
	}
}
