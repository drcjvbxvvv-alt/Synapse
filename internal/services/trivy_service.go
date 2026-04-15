package services

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"gorm.io/gorm"
	"k8s.io/client-go/kubernetes"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/pkg/logger"
)

// TrivyService handles async Trivy image scanning.
// Phase 3（§14.3）: supports both legacy host exec and K8s Job modes.
type TrivyService struct {
	db         *gorm.DB
	jobScanner *TrivyJobScanner // nil = K8s Job mode unavailable
}

// NewTrivyService creates a TrivyService (legacy host exec mode only).
func NewTrivyService(db *gorm.DB) *TrivyService {
	return &TrivyService{db: db}
}

// NewTrivyServiceWithJobScanner creates a TrivyService with K8s Job support (Phase 3).
func NewTrivyServiceWithJobScanner(db *gorm.DB, jobScanner *TrivyJobScanner) *TrivyService {
	return &TrivyService{db: db, jobScanner: jobScanner}
}

// HasJobScanner returns true if the K8s Job scanner is available.
func (s *TrivyService) HasJobScanner() bool {
	return s.jobScanner != nil
}

// trivyVulnerability represents a single CVE entry in Trivy JSON output.
type trivyVulnerability struct {
	VulnerabilityID  string `json:"VulnerabilityID"`
	PkgName          string `json:"PkgName"`
	InstalledVersion string `json:"InstalledVersion"`
	FixedVersion     string `json:"FixedVersion"`
	Severity         string `json:"Severity"`
	Title            string `json:"Title"`
	Description      string `json:"Description"`
}

type trivyResult struct {
	Target          string               `json:"Target"`
	Vulnerabilities []trivyVulnerability `json:"Vulnerabilities"`
}

type trivyReport struct {
	Results []trivyResult `json:"Results"`
}

// ScanResult is the sanitized struct returned to the frontend.
type ScanResult struct {
	ID            uint       `json:"id"`
	ClusterID     uint       `json:"cluster_id"`
	Namespace     string     `json:"namespace"`
	PodName       string     `json:"pod_name"`
	ContainerName string     `json:"container_name"`
	Image         string     `json:"image"`
	Status        string     `json:"status"`
	Critical      int        `json:"critical"`
	High          int        `json:"high"`
	Medium        int        `json:"medium"`
	Low           int        `json:"low"`
	Unknown       int        `json:"unknown"`
	Error         string     `json:"error,omitempty"`
	ScannedAt     *time.Time `json:"scanned_at"`
}

// TriggerJobScan creates a Trivy scan via K8s Job (Phase 3, §14.3).
// Falls back to host exec if jobScanner is not configured.
func (s *TrivyService) TriggerJobScan(
	ctx context.Context,
	clientset kubernetes.Interface,
	clusterID uint,
	namespace, podName, containerName, image string,
) (*models.ImageScanResult, error) {
	if s.jobScanner == nil {
		return nil, fmt.Errorf("K8s Job scanner not configured, use TriggerScan for host exec mode")
	}

	cfg := TrivyJobConfig{
		Image:         image,
		ClusterID:     clusterID,
		Namespace:     namespace,
		PodName:       podName,
		ContainerName: containerName,
		ScanSource:    TrivyScanSourceJob,
	}
	return s.jobScanner.TriggerJobScan(ctx, clientset, cfg)
}

// TriggerScan creates a pending record and kicks off an async goroutine.
// Deprecated: Legacy host exec mode — use TriggerJobScan for K8s Job mode (Phase 3).
func (s *TrivyService) TriggerScan(clusterID uint, namespace, podName, containerName, image string) (*models.ImageScanResult, error) {
	// Deduplicate: if a pending/scanning record exists for same image+cluster, return it
	var existing models.ImageScanResult
	if err := s.db.Where("cluster_id = ? AND image = ? AND status IN ?", clusterID, image, []string{"pending", "scanning"}).
		First(&existing).Error; err == nil {
		return &existing, nil
	}

	record := &models.ImageScanResult{
		ClusterID:     clusterID,
		Namespace:     namespace,
		PodName:       podName,
		ContainerName: containerName,
		Image:         image,
		Status:        "pending",
	}
	if err := s.db.Create(record).Error; err != nil {
		return nil, err
	}

	go s.runScan(record.ID, image)
	return record, nil
}

func (s *TrivyService) runScan(recordID uint, image string) {
	// Mark as scanning
	s.db.Model(&models.ImageScanResult{}).Where("id = ?", recordID).
		Updates(map[string]interface{}{"status": "scanning"})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "trivy", "image",
		"--format", "json",
		"--quiet",
		"--no-progress",
		image,
	)
	out, err := cmd.Output()

	now := time.Now()
	updates := map[string]interface{}{"scanned_at": &now}

	if err != nil {
		errMsg := err.Error()
		if len(out) > 0 {
			errMsg = fmt.Sprintf("%s: %s", err.Error(), truncate(string(out), 400))
		}
		updates["status"] = "failed"
		updates["error"] = errMsg
		s.db.Model(&models.ImageScanResult{}).Where("id = ?", recordID).Updates(updates)
		logger.Warn("trivy scan failed", "image", image, "error", errMsg)
		return
	}

	counts, resultJSON := parseTrivyOutput(out)
	updates["status"] = "completed"
	updates["critical"] = counts["CRITICAL"]
	updates["high"] = counts["HIGH"]
	updates["medium"] = counts["MEDIUM"]
	updates["low"] = counts["LOW"]
	updates["unknown"] = counts["UNKNOWN"]
	updates["result_json"] = resultJSON
	s.db.Model(&models.ImageScanResult{}).Where("id = ?", recordID).Updates(updates)
	logger.Info("trivy scan completed", "image", image, "critical", counts["CRITICAL"], "high", counts["HIGH"])
}

func parseTrivyOutput(data []byte) (map[string]int, string) {
	counts := map[string]int{"CRITICAL": 0, "HIGH": 0, "MEDIUM": 0, "LOW": 0, "UNKNOWN": 0}

	var report trivyReport
	if err := json.Unmarshal(data, &report); err != nil {
		return counts, string(data)
	}
	for _, r := range report.Results {
		for _, v := range r.Vulnerabilities {
			sev := v.Severity
			if _, ok := counts[sev]; ok {
				counts[sev]++
			} else {
				counts["UNKNOWN"]++
			}
		}
	}
	return counts, string(data)
}

// ---------------------------------------------------------------------------
// IngestScanResult — 接收外部 CI 推送的掃描結果（CICD_ARCHITECTURE §5 方案 A）
// ---------------------------------------------------------------------------

// IngestScanRequest 代表外部 CI（GitLab / GitHub Actions 等）推送的掃描結果。
type IngestScanRequest struct {
	Image         string `json:"image" binding:"required"`
	Namespace     string `json:"namespace"`
	PodName       string `json:"pod_name"`
	ContainerName string `json:"container_name"`
	ScanSource    string `json:"scan_source"` // ci_push / jenkins / github_actions etc.
	ResultJSON    string `json:"result_json"`  // Trivy JSON output (optional)
	// 直接提供計數（若無 result_json 可手動填入）
	Critical int `json:"critical"`
	High     int `json:"high"`
	Medium   int `json:"medium"`
	Low      int `json:"low"`
	Unknown  int `json:"unknown"`
}

// IngestScanResult 將外部 CI 推送的掃描結果寫入資料庫。
// 若提供 result_json，自動解析覆蓋計數欄位。
func (s *TrivyService) IngestScanResult(ctx context.Context, clusterID uint, req *IngestScanRequest) (*models.ImageScanResult, error) {
	scanSource := req.ScanSource
	if scanSource == "" {
		scanSource = "ci_push"
	}

	now := time.Now()
	record := &models.ImageScanResult{
		ClusterID:     clusterID,
		Namespace:     req.Namespace,
		PodName:       req.PodName,
		ContainerName: req.ContainerName,
		Image:         req.Image,
		Status:        "completed",
		ScanSource:    scanSource,
		ScannedAt:     &now,
		Critical:      req.Critical,
		High:          req.High,
		Medium:        req.Medium,
		Low:           req.Low,
		Unknown:       req.Unknown,
	}

	// 若提供原始 Trivy JSON，解析計數並覆蓋
	if req.ResultJSON != "" {
		record.ResultJSON = req.ResultJSON
		counts, _ := parseTrivyOutput([]byte(req.ResultJSON))
		record.Critical = counts["CRITICAL"]
		record.High = counts["HIGH"]
		record.Medium = counts["MEDIUM"]
		record.Low = counts["LOW"]
		record.Unknown = counts["UNKNOWN"]
	}

	if err := s.db.WithContext(ctx).Create(record).Error; err != nil {
		return nil, fmt.Errorf("create ingest scan record: %w", err)
	}

	logger.Info("external scan result ingested",
		"cluster_id", clusterID,
		"image", req.Image,
		"scan_source", scanSource,
		"critical", record.Critical,
		"high", record.High,
	)

	return record, nil
}

// GetScanResults returns scan results for a cluster, optionally filtered by namespace.
func (s *TrivyService) GetScanResults(clusterID uint, namespace string) ([]ScanResult, error) {
	var records []models.ImageScanResult
	q := s.db.Where("cluster_id = ?", clusterID)
	if namespace != "" {
		q = q.Where("namespace = ?", namespace)
	}
	if err := q.Order("created_at DESC").Limit(200).Find(&records).Error; err != nil {
		return nil, err
	}

	results := make([]ScanResult, 0, len(records))
	for _, r := range records {
		results = append(results, ScanResult{
			ID:            r.ID,
			ClusterID:     r.ClusterID,
			Namespace:     r.Namespace,
			PodName:       r.PodName,
			ContainerName: r.ContainerName,
			Image:         r.Image,
			Status:        r.Status,
			Critical:      r.Critical,
			High:          r.High,
			Medium:        r.Medium,
			Low:           r.Low,
			Unknown:       r.Unknown,
			Error:         r.Error,
			ScannedAt:     r.ScannedAt,
		})
	}
	return results, nil
}

// GetScanDetail returns the full JSON result for a single scan.
func (s *TrivyService) GetScanDetail(id uint) (*models.ImageScanResult, error) {
	var record models.ImageScanResult
	if err := s.db.First(&record, id).Error; err != nil {
		return nil, err
	}
	return &record, nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
