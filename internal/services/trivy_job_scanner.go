package services

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/pkg/logger"
	"gorm.io/gorm"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// ---------------------------------------------------------------------------
// TrivyJobScanner — K8s Job 模式的 Trivy 掃描器（CICD_ARCHITECTURE §14.3 Phase 2）
//
// 設計原則：
//   - 在目標叢集建立 K8s Job 執行 trivy image scan
//   - 透過 Pod log 讀取 trivy JSON 輸出
//   - 寫入 image_scan_results，scan_source = "pipeline" 或 "job"
//   - 支援 trivy-db-cache PVC 掛載（Phase 4）
//   - Job TTL 自動清理（ttlSecondsAfterFinished）
// ---------------------------------------------------------------------------

const (
	// TrivyDefaultImage is the default Trivy image for K8s Job scanner.
	TrivyDefaultImage = "aquasec/trivy:0.58.0"

	// TrivyJobNamePrefix is the prefix for Trivy scan Job names.
	TrivyJobNamePrefix = "synapse-trivy-scan-"

	// TrivyScanSourceJob is the scan_source value for K8s Job scans.
	TrivyScanSourceJob = "job"

	// TrivyScanSourcePipeline is the scan_source value for pipeline-triggered scans.
	TrivyScanSourcePipeline = "pipeline"

	// TrivyJobTTLSeconds is the TTL for completed Jobs (1 hour).
	TrivyJobTTLSeconds = 3600

	// TrivyJobBackoffLimit is the number of retries for failed Jobs.
	TrivyJobBackoffLimit = 0

	// TrivyDBCachePVCName is the PVC name for shared Trivy DB cache (Phase 4).
	TrivyDBCachePVCName = "trivy-db-cache"

	// TrivyDBCacheMountPath is the mount path for Trivy DB cache.
	TrivyDBCacheMountPath = "/root/.cache/trivy"
)

// TrivyJobScanner creates K8s Jobs to run Trivy scans.
type TrivyJobScanner struct {
	db *gorm.DB
}

// NewTrivyJobScanner creates a TrivyJobScanner.
func NewTrivyJobScanner(db *gorm.DB) *TrivyJobScanner {
	return &TrivyJobScanner{db: db}
}

// TrivyJobConfig holds configuration for a Trivy scan Job.
type TrivyJobConfig struct {
	Image         string // Target image to scan
	ClusterID     uint
	Namespace     string // Namespace to create the Job in (default: "synapse-system")
	PodName       string // Source pod name (optional, for tracking)
	ContainerName string // Source container name (optional)
	Severity      string // Severity filter (e.g., "HIGH,CRITICAL")
	TrivyImage    string // Trivy container image (default: TrivyDefaultImage)
	ScanSource    string // scan_source value (default: "job")
	UseDBCache    bool   // Whether to mount trivy-db-cache PVC

	// Pipeline association (optional)
	PipelineRunID *uint
	StepRunID     *uint
}

// TriggerJobScan creates a pending record and a K8s Job to scan the image.
func (s *TrivyJobScanner) TriggerJobScan(
	ctx context.Context,
	clientset kubernetes.Interface,
	cfg TrivyJobConfig,
) (*models.ImageScanResult, error) {
	// Defaults
	if cfg.TrivyImage == "" {
		cfg.TrivyImage = TrivyDefaultImage
	}
	if cfg.Namespace == "" {
		cfg.Namespace = "synapse-system"
	}
	if cfg.ScanSource == "" {
		cfg.ScanSource = TrivyScanSourceJob
	}

	// Deduplicate: if a pending/scanning record exists for same image+cluster, return it
	var existing models.ImageScanResult
	if err := s.db.WithContext(ctx).
		Where("cluster_id = ? AND image = ? AND status IN ?",
			cfg.ClusterID, cfg.Image, []string{"pending", "scanning"}).
		First(&existing).Error; err == nil {
		return &existing, nil
	}

	// Create DB record
	record := &models.ImageScanResult{
		ClusterID:     cfg.ClusterID,
		Namespace:     cfg.Namespace,
		PodName:       cfg.PodName,
		ContainerName: cfg.ContainerName,
		Image:         cfg.Image,
		Status:        "pending",
		ScanSource:    cfg.ScanSource,
		PipelineRunID: cfg.PipelineRunID,
		StepRunID:     cfg.StepRunID,
	}
	if err := s.db.WithContext(ctx).Create(record).Error; err != nil {
		return nil, fmt.Errorf("create trivy scan record: %w", err)
	}

	// Build and create K8s Job
	job := BuildTrivyScanJob(record.ID, cfg)
	created, err := clientset.BatchV1().Jobs(cfg.Namespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		// Mark record as failed
		s.db.WithContext(ctx).Model(&models.ImageScanResult{}).Where("id = ?", record.ID).
			Updates(map[string]interface{}{
				"status": "failed",
				"error":  fmt.Sprintf("create K8s Job: %v", err),
			})
		return nil, fmt.Errorf("create trivy scan Job: %w", err)
	}

	// Update record with Job name
	s.db.WithContext(ctx).Model(&models.ImageScanResult{}).Where("id = ?", record.ID).
		Updates(map[string]interface{}{
			"status": "scanning",
		})

	logger.Info("trivy scan Job created",
		"record_id", record.ID,
		"job_name", created.Name,
		"image", cfg.Image,
		"namespace", cfg.Namespace,
	)

	return record, nil
}

// CollectJobResult reads the Pod log of a completed Trivy Job and updates the scan record.
func (s *TrivyJobScanner) CollectJobResult(
	ctx context.Context,
	clientset kubernetes.Interface,
	recordID uint,
	jobName string,
	jobNamespace string,
) error {
	// Get Pod for this Job
	podList, err := clientset.CoreV1().Pods(jobNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("job-name=%s", jobName),
	})
	if err != nil {
		return fmt.Errorf("list pods for Job %s: %w", jobName, err)
	}
	if len(podList.Items) == 0 {
		return fmt.Errorf("no pods found for Job %s", jobName)
	}

	pod := podList.Items[0]
	logOpts := &corev1.PodLogOptions{
		Container: "trivy",
	}

	req := clientset.CoreV1().Pods(jobNamespace).GetLogs(pod.Name, logOpts)
	stream, err := req.Stream(ctx)
	if err != nil {
		return fmt.Errorf("stream logs for pod %s: %w", pod.Name, err)
	}
	defer stream.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, stream); err != nil {
		return fmt.Errorf("read logs for pod %s: %w", pod.Name, err)
	}

	// Parse trivy output (reuse existing parser)
	now := time.Now()
	output := buf.Bytes()
	counts, resultJSON := parseTrivyOutput(output)

	updates := map[string]interface{}{
		"status":      "completed",
		"scanned_at":  &now,
		"critical":    counts["CRITICAL"],
		"high":        counts["HIGH"],
		"medium":      counts["MEDIUM"],
		"low":         counts["LOW"],
		"unknown":     counts["UNKNOWN"],
		"result_json": resultJSON,
	}

	if err := s.db.WithContext(ctx).Model(&models.ImageScanResult{}).
		Where("id = ?", recordID).Updates(updates).Error; err != nil {
		return fmt.Errorf("update scan record %d: %w", recordID, err)
	}

	logger.Info("trivy Job scan result collected",
		"record_id", recordID,
		"job_name", jobName,
		"critical", counts["CRITICAL"],
		"high", counts["HIGH"],
	)
	return nil
}

// MarkJobFailed updates the scan record when the Job fails.
func (s *TrivyJobScanner) MarkJobFailed(ctx context.Context, recordID uint, reason string) error {
	now := time.Now()
	return s.db.WithContext(ctx).Model(&models.ImageScanResult{}).
		Where("id = ?", recordID).
		Updates(map[string]interface{}{
			"status":     "failed",
			"error":      reason,
			"scanned_at": &now,
		}).Error
}

// ---------------------------------------------------------------------------
// Job spec builder
// ---------------------------------------------------------------------------

// BuildTrivyScanJob constructs a K8s Job spec for Trivy image scanning.
func BuildTrivyScanJob(recordID uint, cfg TrivyJobConfig) *batchv1.Job {
	jobName := fmt.Sprintf("%s%d", TrivyJobNamePrefix, recordID)
	backoffLimit := int32(TrivyJobBackoffLimit)
	ttl := int32(TrivyJobTTLSeconds)

	// Build trivy command args
	args := []string{
		"image",
		"--format", "json",
		"--quiet",
		"--no-progress",
	}
	if cfg.Severity != "" {
		args = append(args, "--severity="+cfg.Severity)
	}
	args = append(args, cfg.Image)

	container := corev1.Container{
		Name:    "trivy",
		Image:   cfg.TrivyImage,
		Command: []string{"trivy"},
		Args:    args,
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    mustParseQuantity("100m"),
				corev1.ResourceMemory: mustParseQuantity("256Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    mustParseQuantity("500m"),
				corev1.ResourceMemory: mustParseQuantity("1Gi"),
			},
		},
	}

	// Phase 4: mount trivy-db-cache PVC if available
	var volumes []corev1.Volume
	if cfg.UseDBCache {
		volumes = append(volumes, corev1.Volume{
			Name: "trivy-db-cache",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: TrivyDBCachePVCName,
					ReadOnly:  true,
				},
			},
		})
		container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
			Name:      "trivy-db-cache",
			MountPath: TrivyDBCacheMountPath,
			ReadOnly:  true,
		})
		// Skip DB download when using cache
		container.Env = append(container.Env, corev1.EnvVar{
			Name:  "TRIVY_SKIP_DB_UPDATE",
			Value: "true",
		})
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: cfg.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "synapse",
				"app.kubernetes.io/component":  "trivy-scan",
				"synapse.io/scan-record-id":    fmt.Sprintf("%d", recordID),
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoffLimit,
			TTLSecondsAfterFinished: &ttl,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/managed-by": "synapse",
						"app.kubernetes.io/component":  "trivy-scan",
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers:    []corev1.Container{container},
					Volumes:       volumes,
				},
			},
		},
	}

	return job
}

// ---------------------------------------------------------------------------
// Phase 4: Trivy DB Cache — PVC + CronJob spec builders
// ---------------------------------------------------------------------------

// BuildTrivyDBCachePVC returns a PVC spec for shared Trivy DB cache.
func BuildTrivyDBCachePVC(namespace string) *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      TrivyDBCachePVCName,
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "synapse",
				"app.kubernetes.io/component":  "trivy-db-cache",
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: mustParseQuantity("2Gi"),
				},
			},
		},
	}
}

// BuildTrivyDBUpdateCronJob returns a CronJob spec that updates Trivy DB daily.
func BuildTrivyDBUpdateCronJob(namespace, trivyImage string) *batchv1.CronJob {
	if trivyImage == "" {
		trivyImage = TrivyDefaultImage
	}
	backoffLimit := int32(1)
	ttl := int32(TrivyJobTTLSeconds)

	return &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "synapse-trivy-db-update",
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "synapse",
				"app.kubernetes.io/component":  "trivy-db-cache",
			},
		},
		Spec: batchv1.CronJobSpec{
			Schedule: "0 3 * * *", // Daily at 03:00 UTC
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					BackoffLimit:            &backoffLimit,
					TTLSecondsAfterFinished: &ttl,
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							RestartPolicy: corev1.RestartPolicyNever,
							Containers: []corev1.Container{
								{
									Name:    "trivy-db-update",
									Image:   trivyImage,
									Command: []string{"trivy"},
									Args:    []string{"image", "--download-db-only"},
									VolumeMounts: []corev1.VolumeMount{
										{
											Name:      "trivy-db-cache",
											MountPath: TrivyDBCacheMountPath,
										},
									},
									Resources: corev1.ResourceRequirements{
										Requests: corev1.ResourceList{
											corev1.ResourceCPU:    mustParseQuantity("100m"),
											corev1.ResourceMemory: mustParseQuantity("256Mi"),
										},
										Limits: corev1.ResourceList{
											corev1.ResourceCPU:    mustParseQuantity("500m"),
											corev1.ResourceMemory: mustParseQuantity("512Mi"),
										},
									},
								},
							},
							Volumes: []corev1.Volume{
								{
									Name: "trivy-db-cache",
									VolumeSource: corev1.VolumeSource{
										PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
											ClaimName: TrivyDBCachePVCName,
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func mustParseQuantity(s string) resource.Quantity {
	q, err := resource.ParseQuantity(s)
	if err != nil {
		panic(fmt.Sprintf("invalid quantity %q: %v", s, err))
	}
	return q
}
