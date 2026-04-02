package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"gorm.io/gorm"

	"github.com/clay-wangzhi/Synapse/internal/models"
	"github.com/clay-wangzhi/Synapse/pkg/logger"
)

const (
	benchNamespace  = "kube-system"
	benchJobPrefix  = "kubebench-"
	benchImage      = "aquasec/kube-bench:latest"
)

// BenchService manages kube-bench CIS benchmark Jobs.
type BenchService struct {
	db     *gorm.DB
	k8sMgr BenchK8sClientManager
}

// BenchK8sClientManager is a minimal interface for getting K8s clients.
type BenchK8sClientManager interface {
	GetK8sClientByID(clusterID uint) *K8sClient
}

func NewBenchService(db *gorm.DB, k8sMgr BenchK8sClientManager) *BenchService {
	return &BenchService{db: db, k8sMgr: k8sMgr}
}

// benchReport represents the top-level kube-bench JSON output.
type benchReport struct {
	Controls []benchControl `json:"Controls"`
}

type benchControl struct {
	Tests []benchTest `json:"tests"`
}

type benchTest struct {
	Results []benchTestResult `json:"results"`
}

type benchTestResult struct {
	Status string `json:"status"` // PASS, FAIL, WARN, INFO
}

// TriggerBenchmark creates a BenchResult record and launches a K8s Job.
func (s *BenchService) TriggerBenchmark(clusterID uint) (*models.BenchResult, error) {
	// Only allow one active run per cluster
	var active models.BenchResult
	if err := s.db.Where("cluster_id = ? AND status IN ?", clusterID, []string{"pending", "running"}).
		First(&active).Error; err == nil {
		return &active, nil
	}

	record := &models.BenchResult{
		ClusterID: clusterID,
		Status:    "pending",
	}
	if err := s.db.Create(record).Error; err != nil {
		return nil, err
	}

	go s.runBenchJob(record.ID, clusterID)
	return record, nil
}

func (s *BenchService) runBenchJob(recordID, clusterID uint) {
	k8sClient := s.k8sMgr.GetK8sClientByID(clusterID)
	if k8sClient == nil {
		s.failRecord(recordID, "cluster client not available")
		return
	}
	clientset := k8sClient.GetClientset()

	jobName := fmt.Sprintf("%s%d-%d", benchJobPrefix, clusterID, recordID)
	s.db.Model(&models.BenchResult{}).Where("id = ?", recordID).
		Updates(map[string]interface{}{"status": "running", "job_name": jobName})

	privileged := true
	hostPID := true
	ttl := int32(300) // clean up after 5m

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: benchNamespace,
			Labels:    map[string]string{"app": "kubebench", "cluster-id": fmt.Sprintf("%d", clusterID)},
		},
		Spec: batchv1.JobSpec{
			TTLSecondsAfterFinished: &ttl,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					HostPID:       hostPID,
					RestartPolicy: corev1.RestartPolicyNever,
					Volumes: []corev1.Volume{
						{Name: "var-lib-etcd", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/var/lib/etcd"}}},
						{Name: "var-lib-kubelet", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/var/lib/kubelet"}}},
						{Name: "etc-systemd", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/etc/systemd"}}},
						{Name: "etc-kubernetes", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/etc/kubernetes"}}},
						{Name: "usr-bin", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/usr/bin"}}},
					},
					Containers: []corev1.Container{
						{
							Name:  "kube-bench",
							Image: benchImage,
							Command: []string{"kube-bench", "--json"},
							SecurityContext: &corev1.SecurityContext{
								Privileged: &privileged,
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "var-lib-etcd", MountPath: "/var/lib/etcd"},
								{Name: "var-lib-kubelet", MountPath: "/var/lib/kubelet"},
								{Name: "etc-systemd", MountPath: "/etc/systemd"},
								{Name: "etc-kubernetes", MountPath: "/etc/kubernetes"},
								{Name: "usr-bin", MountPath: "/usr/local/mount-from-host/bin"},
							},
						},
					},
				},
			},
		},
	}

	ctx := context.Background()
	createdJob, err := clientset.BatchV1().Jobs(benchNamespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		s.failRecord(recordID, fmt.Sprintf("failed to create Job: %v", err))
		return
	}
	_ = createdJob

	// Poll for completion (max 10 minutes)
	deadline := time.Now().Add(10 * time.Minute)
	for time.Now().Before(deadline) {
		time.Sleep(10 * time.Second)

		j, err := clientset.BatchV1().Jobs(benchNamespace).Get(ctx, jobName, metav1.GetOptions{})
		if err != nil {
			continue
		}
		if j.Status.Succeeded > 0 {
			break
		}
		if j.Status.Failed > 0 {
			s.failRecord(recordID, "kube-bench Job failed")
			return
		}
	}

	// Collect Pod logs
	podList, err := clientset.CoreV1().Pods(benchNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("job-name=%s", jobName),
	})
	if err != nil || len(podList.Items) == 0 {
		s.failRecord(recordID, "could not find benchmark Pod")
		return
	}

	podName := podList.Items[0].Name
	req := clientset.CoreV1().Pods(benchNamespace).GetLogs(podName, &corev1.PodLogOptions{})
	logBytes, err := req.DoRaw(ctx)
	if err != nil {
		s.failRecord(recordID, fmt.Sprintf("failed to get Pod logs: %v", err))
		return
	}

	pass, fail, warn, info := parseBenchOutput(logBytes)
	score := 0.0
	if pass+fail > 0 {
		score = float64(pass) / float64(pass+fail) * 100
	}

	now := time.Now()
	s.db.Model(&models.BenchResult{}).Where("id = ?", recordID).Updates(map[string]interface{}{
		"status":      "completed",
		"pass":        pass,
		"fail":        fail,
		"warn":        warn,
		"info":        info,
		"score":       score,
		"result_json": string(logBytes),
		"run_at":      &now,
	})
	logger.Info("kube-bench completed", "cluster_id", clusterID, "pass", pass, "fail", fail, "score", score)
}

func parseBenchOutput(data []byte) (pass, fail, warn, info int) {
	var report benchReport
	if err := json.Unmarshal(data, &report); err != nil {
		return
	}
	for _, ctrl := range report.Controls {
		for _, t := range ctrl.Tests {
			for _, r := range t.Results {
				switch r.Status {
				case "PASS":
					pass++
				case "FAIL":
					fail++
				case "WARN":
					warn++
				case "INFO":
					info++
				}
			}
		}
	}
	return
}

func (s *BenchService) failRecord(id uint, msg string) {
	s.db.Model(&models.BenchResult{}).Where("id = ?", id).
		Updates(map[string]interface{}{"status": "failed", "error": msg})
	logger.Warn("bench job failed", "record_id", id, "error", msg)
}

// GetBenchResults returns benchmark history for a cluster.
func (s *BenchService) GetBenchResults(clusterID uint) ([]models.BenchResult, error) {
	var records []models.BenchResult
	if err := s.db.Where("cluster_id = ?", clusterID).
		Select("id, cluster_id, status, pass, fail, warn, info, score, error, job_name, run_at, created_at, updated_at").
		Order("created_at DESC").Limit(20).Find(&records).Error; err != nil {
		return nil, err
	}
	return records, nil
}

// GetBenchDetail returns the full JSON output for a single benchmark run.
func (s *BenchService) GetBenchDetail(id uint) (*models.BenchResult, error) {
	var record models.BenchResult
	if err := s.db.First(&record, id).Error; err != nil {
		return nil, err
	}
	return &record, nil
}
