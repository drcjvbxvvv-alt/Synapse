package services

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// BuildTrivyScanJob
// ---------------------------------------------------------------------------

func TestBuildTrivyScanJob_BasicFields(t *testing.T) {
	cfg := TrivyJobConfig{
		Image:     "nginx:1.25",
		ClusterID: 1,
		Namespace: "synapse-system",
	}
	job := BuildTrivyScanJob(42, cfg)

	if job.Name != "synapse-trivy-scan-42" {
		t.Errorf("expected job name synapse-trivy-scan-42, got %s", job.Name)
	}
	if job.Namespace != "synapse-system" {
		t.Errorf("expected namespace synapse-system, got %s", job.Namespace)
	}
	if job.Labels["app.kubernetes.io/managed-by"] != "synapse" {
		t.Error("missing managed-by label")
	}
	if job.Labels["synapse.io/scan-record-id"] != "42" {
		t.Errorf("expected scan-record-id=42, got %s", job.Labels["synapse.io/scan-record-id"])
	}
}

func TestBuildTrivyScanJob_UsesProvidedTrivyImage(t *testing.T) {
	// BuildTrivyScanJob uses cfg.TrivyImage directly; defaults are applied in TriggerJobScan.
	cfg := TrivyJobConfig{
		Image:      "nginx:1.25",
		TrivyImage: TrivyDefaultImage,
	}
	job := BuildTrivyScanJob(1, cfg)

	container := job.Spec.Template.Spec.Containers[0]
	if container.Image != TrivyDefaultImage {
		t.Errorf("expected trivy image %s, got %s", TrivyDefaultImage, container.Image)
	}
}

func TestBuildTrivyScanJob_CustomTrivyImage(t *testing.T) {
	cfg := TrivyJobConfig{
		Image:      "nginx:1.25",
		TrivyImage: "aquasec/trivy:0.60.0",
	}
	job := BuildTrivyScanJob(1, cfg)

	container := job.Spec.Template.Spec.Containers[0]
	if container.Image != "aquasec/trivy:0.60.0" {
		t.Errorf("expected custom trivy image, got %s", container.Image)
	}
}

func TestBuildTrivyScanJob_ContainerArgs(t *testing.T) {
	cfg := TrivyJobConfig{
		Image:      "myapp:v1",
		TrivyImage: TrivyDefaultImage,
	}
	job := BuildTrivyScanJob(1, cfg)

	container := job.Spec.Template.Spec.Containers[0]
	if container.Name != "trivy" {
		t.Errorf("expected container name trivy, got %s", container.Name)
	}

	args := strings.Join(container.Args, " ")
	if !strings.Contains(args, "image") {
		t.Error("args should contain 'image' subcommand")
	}
	if !strings.Contains(args, "--format json") {
		t.Error("args should contain --format json")
	}
	if !strings.Contains(args, "--quiet") {
		t.Error("args should contain --quiet")
	}
	if !strings.Contains(args, "myapp:v1") {
		t.Errorf("args should contain target image, got: %s", args)
	}
}

func TestBuildTrivyScanJob_SeverityFilter(t *testing.T) {
	cfg := TrivyJobConfig{
		Image:      "nginx:1.25",
		TrivyImage: TrivyDefaultImage,
		Severity:   "HIGH,CRITICAL",
	}
	job := BuildTrivyScanJob(1, cfg)

	args := strings.Join(job.Spec.Template.Spec.Containers[0].Args, " ")
	if !strings.Contains(args, "--severity=HIGH,CRITICAL") {
		t.Errorf("args should contain severity filter, got: %s", args)
	}
}

func TestBuildTrivyScanJob_NoSeverityFilter(t *testing.T) {
	cfg := TrivyJobConfig{
		Image:      "nginx:1.25",
		TrivyImage: TrivyDefaultImage,
		Severity:   "",
	}
	job := BuildTrivyScanJob(1, cfg)

	args := strings.Join(job.Spec.Template.Spec.Containers[0].Args, " ")
	if strings.Contains(args, "--severity") {
		t.Error("args should not contain severity filter when empty")
	}
}

func TestBuildTrivyScanJob_TTLAndBackoff(t *testing.T) {
	cfg := TrivyJobConfig{
		Image:      "nginx:1.25",
		TrivyImage: TrivyDefaultImage,
	}
	job := BuildTrivyScanJob(1, cfg)

	if *job.Spec.BackoffLimit != int32(TrivyJobBackoffLimit) {
		t.Errorf("expected backoff limit %d, got %d", TrivyJobBackoffLimit, *job.Spec.BackoffLimit)
	}
	if *job.Spec.TTLSecondsAfterFinished != int32(TrivyJobTTLSeconds) {
		t.Errorf("expected TTL %d, got %d", TrivyJobTTLSeconds, *job.Spec.TTLSecondsAfterFinished)
	}
}

func TestBuildTrivyScanJob_RestartPolicyNever(t *testing.T) {
	cfg := TrivyJobConfig{
		Image:      "nginx:1.25",
		TrivyImage: TrivyDefaultImage,
	}
	job := BuildTrivyScanJob(1, cfg)

	if job.Spec.Template.Spec.RestartPolicy != "Never" {
		t.Errorf("expected RestartPolicy Never, got %s", job.Spec.Template.Spec.RestartPolicy)
	}
}

func TestBuildTrivyScanJob_ResourceRequests(t *testing.T) {
	cfg := TrivyJobConfig{
		Image:      "nginx:1.25",
		TrivyImage: TrivyDefaultImage,
	}
	job := BuildTrivyScanJob(1, cfg)

	container := job.Spec.Template.Spec.Containers[0]
	cpuReq := container.Resources.Requests["cpu"]
	memReq := container.Resources.Requests["memory"]
	if cpuReq.String() != "100m" {
		t.Errorf("expected CPU request 100m, got %s", cpuReq.String())
	}
	if memReq.String() != "256Mi" {
		t.Errorf("expected memory request 256Mi, got %s", memReq.String())
	}
}

// ---------------------------------------------------------------------------
// BuildTrivyScanJob with DB cache (Phase 4)
// ---------------------------------------------------------------------------

func TestBuildTrivyScanJob_WithDBCache(t *testing.T) {
	cfg := TrivyJobConfig{
		Image:      "nginx:1.25",
		TrivyImage: TrivyDefaultImage,
		UseDBCache: true,
	}
	job := BuildTrivyScanJob(1, cfg)

	// Check volume
	if len(job.Spec.Template.Spec.Volumes) != 1 {
		t.Fatalf("expected 1 volume, got %d", len(job.Spec.Template.Spec.Volumes))
	}
	vol := job.Spec.Template.Spec.Volumes[0]
	if vol.Name != "trivy-db-cache" {
		t.Errorf("expected volume name trivy-db-cache, got %s", vol.Name)
	}
	if vol.PersistentVolumeClaim.ClaimName != TrivyDBCachePVCName {
		t.Errorf("expected PVC claim %s, got %s", TrivyDBCachePVCName, vol.PersistentVolumeClaim.ClaimName)
	}

	// Check volume mount
	container := job.Spec.Template.Spec.Containers[0]
	if len(container.VolumeMounts) != 1 {
		t.Fatalf("expected 1 volume mount, got %d", len(container.VolumeMounts))
	}
	if container.VolumeMounts[0].MountPath != TrivyDBCacheMountPath {
		t.Errorf("expected mount path %s, got %s", TrivyDBCacheMountPath, container.VolumeMounts[0].MountPath)
	}

	// Check TRIVY_SKIP_DB_UPDATE env
	foundEnv := false
	for _, env := range container.Env {
		if env.Name == "TRIVY_SKIP_DB_UPDATE" && env.Value == "true" {
			foundEnv = true
		}
	}
	if !foundEnv {
		t.Error("expected TRIVY_SKIP_DB_UPDATE=true env var")
	}
}

func TestBuildTrivyScanJob_WithoutDBCache(t *testing.T) {
	cfg := TrivyJobConfig{
		Image:      "nginx:1.25",
		TrivyImage: TrivyDefaultImage,
		UseDBCache: false,
	}
	job := BuildTrivyScanJob(1, cfg)

	if len(job.Spec.Template.Spec.Volumes) != 0 {
		t.Errorf("expected no volumes, got %d", len(job.Spec.Template.Spec.Volumes))
	}
	container := job.Spec.Template.Spec.Containers[0]
	if len(container.VolumeMounts) != 0 {
		t.Errorf("expected no volume mounts, got %d", len(container.VolumeMounts))
	}
}

// ---------------------------------------------------------------------------
// BuildTrivyDBCachePVC
// ---------------------------------------------------------------------------

func TestBuildTrivyDBCachePVC(t *testing.T) {
	pvc := BuildTrivyDBCachePVC("synapse-system")

	if pvc.Name != TrivyDBCachePVCName {
		t.Errorf("expected PVC name %s, got %s", TrivyDBCachePVCName, pvc.Name)
	}
	if pvc.Namespace != "synapse-system" {
		t.Errorf("expected namespace synapse-system, got %s", pvc.Namespace)
	}
	if pvc.Labels["app.kubernetes.io/component"] != "trivy-db-cache" {
		t.Error("missing component label")
	}

	storageReq := pvc.Spec.Resources.Requests["storage"]
	if storageReq.String() != "2Gi" {
		t.Errorf("expected storage 2Gi, got %s", storageReq.String())
	}
}

// ---------------------------------------------------------------------------
// BuildTrivyDBUpdateCronJob
// ---------------------------------------------------------------------------

func TestBuildTrivyDBUpdateCronJob_BasicFields(t *testing.T) {
	cj := BuildTrivyDBUpdateCronJob("synapse-system", "")

	if cj.Name != "synapse-trivy-db-update" {
		t.Errorf("expected name synapse-trivy-db-update, got %s", cj.Name)
	}
	if cj.Namespace != "synapse-system" {
		t.Errorf("expected namespace synapse-system, got %s", cj.Namespace)
	}
	if cj.Spec.Schedule != "0 3 * * *" {
		t.Errorf("expected daily schedule, got %s", cj.Spec.Schedule)
	}
}

func TestBuildTrivyDBUpdateCronJob_DefaultImage(t *testing.T) {
	cj := BuildTrivyDBUpdateCronJob("synapse-system", "")

	container := cj.Spec.JobTemplate.Spec.Template.Spec.Containers[0]
	if container.Image != TrivyDefaultImage {
		t.Errorf("expected default image %s, got %s", TrivyDefaultImage, container.Image)
	}
}

func TestBuildTrivyDBUpdateCronJob_CustomImage(t *testing.T) {
	cj := BuildTrivyDBUpdateCronJob("synapse-system", "aquasec/trivy:0.60.0")

	container := cj.Spec.JobTemplate.Spec.Template.Spec.Containers[0]
	if container.Image != "aquasec/trivy:0.60.0" {
		t.Errorf("expected custom image, got %s", container.Image)
	}
}

func TestBuildTrivyDBUpdateCronJob_DownloadDBOnly(t *testing.T) {
	cj := BuildTrivyDBUpdateCronJob("synapse-system", "")

	container := cj.Spec.JobTemplate.Spec.Template.Spec.Containers[0]
	args := strings.Join(container.Args, " ")
	if !strings.Contains(args, "--download-db-only") {
		t.Errorf("expected --download-db-only flag, got: %s", args)
	}
}

func TestBuildTrivyDBUpdateCronJob_MountsDBCache(t *testing.T) {
	cj := BuildTrivyDBUpdateCronJob("synapse-system", "")

	volumes := cj.Spec.JobTemplate.Spec.Template.Spec.Volumes
	if len(volumes) != 1 || volumes[0].Name != "trivy-db-cache" {
		t.Error("expected trivy-db-cache volume")
	}

	container := cj.Spec.JobTemplate.Spec.Template.Spec.Containers[0]
	if len(container.VolumeMounts) != 1 {
		t.Fatalf("expected 1 volume mount, got %d", len(container.VolumeMounts))
	}
	if container.VolumeMounts[0].MountPath != TrivyDBCacheMountPath {
		t.Errorf("expected mount path %s, got %s", TrivyDBCacheMountPath, container.VolumeMounts[0].MountPath)
	}
	// CronJob should mount read-write (not read-only) since it updates the DB
	if container.VolumeMounts[0].ReadOnly {
		t.Error("CronJob mount should be read-write, not read-only")
	}
}

// ---------------------------------------------------------------------------
// NewTrivyJobScanner
// ---------------------------------------------------------------------------

func TestNewTrivyJobScanner(t *testing.T) {
	scanner := NewTrivyJobScanner(nil)
	if scanner == nil {
		t.Fatal("expected non-nil scanner")
	}
}

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

func TestTrivyConstants(t *testing.T) {
	if TrivyDefaultImage != "aquasec/trivy:0.58.0" {
		t.Errorf("unexpected default image: %s", TrivyDefaultImage)
	}
	if TrivyJobNamePrefix != "synapse-trivy-scan-" {
		t.Errorf("unexpected job prefix: %s", TrivyJobNamePrefix)
	}
	if TrivyScanSourceJob != "job" {
		t.Errorf("unexpected scan source job: %s", TrivyScanSourceJob)
	}
	if TrivyScanSourcePipeline != "pipeline" {
		t.Errorf("unexpected scan source pipeline: %s", TrivyScanSourcePipeline)
	}
	if TrivyDBCachePVCName != "trivy-db-cache" {
		t.Errorf("unexpected PVC name: %s", TrivyDBCachePVCName)
	}
}
