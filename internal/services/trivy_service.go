package services

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"gorm.io/gorm"

	"github.com/clay-wangzhi/Synapse/internal/models"
	"github.com/clay-wangzhi/Synapse/pkg/logger"
)

// TrivyService handles async Trivy image scanning.
type TrivyService struct {
	db *gorm.DB
}

func NewTrivyService(db *gorm.DB) *TrivyService {
	return &TrivyService{db: db}
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

// TriggerScan creates a pending record and kicks off an async goroutine.
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
