package services

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/shaia/Synapse/internal/models"
)

// ListReports returns compliance reports for a cluster.
func (s *ComplianceService) ListReports(ctx context.Context, clusterID uint, framework string) ([]models.ComplianceReport, error) {
	var reports []models.ComplianceReport
	q := s.db.WithContext(ctx).Where("cluster_id = ?", clusterID)
	if framework != "" {
		q = q.Where("framework = ?", framework)
	}
	if err := q.Order("created_at DESC").Limit(50).Find(&reports).Error; err != nil {
		return nil, fmt.Errorf("list compliance reports: %w", err)
	}
	// Clear large JSON from list response
	for i := range reports {
		reports[i].ResultJSON = ""
	}
	return reports, nil
}

// GetReport returns a single compliance report.
func (s *ComplianceService) GetReport(ctx context.Context, clusterID, reportID uint) (*models.ComplianceReport, error) {
	var report models.ComplianceReport
	if err := s.db.WithContext(ctx).Where("id = ? AND cluster_id = ?", reportID, clusterID).First(&report).Error; err != nil {
		return nil, fmt.Errorf("get compliance report: %w", err)
	}
	return &report, nil
}

// DeleteReport removes a compliance report.
func (s *ComplianceService) DeleteReport(ctx context.Context, clusterID, reportID uint) error {
	result := s.db.WithContext(ctx).Where("id = ? AND cluster_id = ?", reportID, clusterID).Delete(&models.ComplianceReport{})
	if result.Error != nil {
		return fmt.Errorf("delete compliance report: %w", result.Error)
	}
	return nil
}

// ─── Violation Timeline ────────────────────────────────────────────────────

// ViolationFilter defines query parameters for violation timeline.
type ViolationFilter struct {
	Source   string
	Severity string
	Resolved *bool
	Since    *time.Time
	Until    *time.Time
}

// ListViolations returns paginated violation events.
func (s *ComplianceService) ListViolations(ctx context.Context, clusterID uint, filter ViolationFilter, page, pageSize int) ([]models.ViolationEvent, int64, error) {
	q := s.db.WithContext(ctx).Model(&models.ViolationEvent{}).Where("cluster_id = ?", clusterID)
	if filter.Source != "" {
		q = q.Where("source = ?", filter.Source)
	}
	if filter.Severity != "" {
		q = q.Where("severity = ?", filter.Severity)
	}
	if filter.Resolved != nil {
		if *filter.Resolved {
			q = q.Where("resolved_at IS NOT NULL")
		} else {
			q = q.Where("resolved_at IS NULL")
		}
	}
	if filter.Since != nil {
		q = q.Where("created_at >= ?", *filter.Since)
	}
	if filter.Until != nil {
		q = q.Where("created_at <= ?", *filter.Until)
	}

	var total int64
	q.Count(&total)

	var events []models.ViolationEvent
	offset := (page - 1) * pageSize
	if err := q.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&events).Error; err != nil {
		return nil, 0, fmt.Errorf("list violations: %w", err)
	}
	return events, total, nil
}

// ViolationStats is the aggregated violation count.
type ViolationStats struct {
	TotalOpen     int64            `json:"total_open"`
	TotalResolved int64            `json:"total_resolved"`
	BySource      map[string]int64 `json:"by_source"`
	BySeverity    map[string]int64 `json:"by_severity"`
}

// GetViolationStats returns aggregated violation stats for a cluster.
func (s *ComplianceService) GetViolationStats(ctx context.Context, clusterID uint) (*ViolationStats, error) {
	stats := &ViolationStats{
		BySource:   make(map[string]int64),
		BySeverity: make(map[string]int64),
	}

	s.db.WithContext(ctx).Model(&models.ViolationEvent{}).
		Where("cluster_id = ? AND resolved_at IS NULL", clusterID).Count(&stats.TotalOpen)
	s.db.WithContext(ctx).Model(&models.ViolationEvent{}).
		Where("cluster_id = ? AND resolved_at IS NOT NULL", clusterID).Count(&stats.TotalResolved)

	type kv struct {
		Key   string
		Count int64
	}

	var sourceRows []kv
	s.db.WithContext(ctx).Model(&models.ViolationEvent{}).
		Select("source as key, count(*) as count").
		Where("cluster_id = ? AND resolved_at IS NULL", clusterID).
		Group("source").Find(&sourceRows)
	for _, r := range sourceRows {
		stats.BySource[r.Key] = r.Count
	}

	var sevRows []kv
	s.db.WithContext(ctx).Model(&models.ViolationEvent{}).
		Select("severity as key, count(*) as count").
		Where("cluster_id = ? AND resolved_at IS NULL", clusterID).
		Group("severity").Find(&sevRows)
	for _, r := range sevRows {
		stats.BySeverity[r.Key] = r.Count
	}

	return stats, nil
}

// ResolveViolation marks a violation as resolved.
func (s *ComplianceService) ResolveViolation(ctx context.Context, clusterID, violationID uint, resolvedBy string) error {
	now := time.Now()
	result := s.db.WithContext(ctx).Model(&models.ViolationEvent{}).
		Where("id = ? AND cluster_id = ? AND resolved_at IS NULL", violationID, clusterID).
		Updates(map[string]interface{}{"resolved_at": &now, "resolved_by": resolvedBy})
	if result.Error != nil {
		return fmt.Errorf("resolve violation: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("violation not found or already resolved")
	}
	return nil
}

// SyncViolationsFromScan imports violations from Trivy scan results.
func (s *ComplianceService) SyncViolationsFromScan(ctx context.Context, clusterID uint, scans []models.ImageScanResult) {
	for _, scan := range scans {
		if scan.Critical == 0 && scan.High == 0 {
			continue
		}
		sev := "high"
		if scan.Critical > 0 {
			sev = "critical"
		}
		fp := fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("trivy:%d:%s", clusterID, scan.Image))))
		event := models.ViolationEvent{
			ClusterID:    clusterID,
			Source:       "trivy",
			Severity:     sev,
			Title:        fmt.Sprintf("Image %s: %d critical, %d high vulnerabilities", scan.Image, scan.Critical, scan.High),
			Description:  fmt.Sprintf("Scanned at %v", scan.ScannedAt),
			ResourceType: "Image",
			ResourceRef:  scan.Image,
			Fingerprint:  fp[:64],
		}
		// Upsert: skip if fingerprint exists
		s.db.WithContext(ctx).Where("fingerprint = ?", event.Fingerprint).FirstOrCreate(&event)
	}
}

// SyncViolationsFromBench imports violations from a benchmark result.
func (s *ComplianceService) SyncViolationsFromBench(ctx context.Context, clusterID uint, bench *models.BenchResult) {
	if bench.Fail == 0 {
		return
	}
	sev := "medium"
	if bench.Score < 50 {
		sev = "high"
	}
	fp := fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("bench:%d:%d", clusterID, bench.ID))))
	event := models.ViolationEvent{
		ClusterID:    clusterID,
		Source:       "bench",
		Severity:     sev,
		Title:        fmt.Sprintf("CIS Benchmark: %d failures (score: %.1f%%)", bench.Fail, bench.Score),
		Description:  fmt.Sprintf("Benchmark run at %v, %d pass / %d fail / %d warn", bench.RunAt, bench.Pass, bench.Fail, bench.Warn),
		ResourceType: "Cluster",
		ResourceRef:  fmt.Sprintf("cluster/%d", clusterID),
		Fingerprint:  fp[:64],
	}
	s.db.WithContext(ctx).Where("fingerprint = ?", event.Fingerprint).FirstOrCreate(&event)
}

// ─── Evidence ──────────────────────────────────────────────────────────────

// CaptureEvidence captures an evidence snapshot for a control.
func (s *ComplianceService) CaptureEvidence(ctx context.Context, clusterID uint, framework, controlID, controlTitle, evidenceType, dataJSON string) (*models.ComplianceEvidence, error) {
	ev := &models.ComplianceEvidence{
		ClusterID:    clusterID,
		Framework:    framework,
		ControlID:    controlID,
		ControlTitle: controlTitle,
		EvidenceType: evidenceType,
		DataJSON:     dataJSON,
		CapturedAt:   time.Now(),
	}
	if err := s.db.WithContext(ctx).Create(ev).Error; err != nil {
		return nil, fmt.Errorf("capture evidence: %w", err)
	}
	return ev, nil
}

// ListEvidence returns evidence for a cluster, optionally filtered.
func (s *ComplianceService) ListEvidence(ctx context.Context, clusterID uint, framework, controlID string) ([]models.ComplianceEvidence, error) {
	q := s.db.WithContext(ctx).Where("cluster_id = ?", clusterID)
	if framework != "" {
		q = q.Where("framework = ?", framework)
	}
	if controlID != "" {
		q = q.Where("control_id = ?", controlID)
	}
	var items []models.ComplianceEvidence
	if err := q.Order("captured_at DESC").Limit(100).Find(&items).Error; err != nil {
		return nil, fmt.Errorf("list evidence: %w", err)
	}
	return items, nil
}

// GetEvidence returns a single evidence item.
func (s *ComplianceService) GetEvidence(ctx context.Context, evidenceID uint) (*models.ComplianceEvidence, error) {
	var ev models.ComplianceEvidence
	if err := s.db.WithContext(ctx).First(&ev, evidenceID).Error; err != nil {
		return nil, fmt.Errorf("get evidence: %w", err)
	}
	return &ev, nil
}
