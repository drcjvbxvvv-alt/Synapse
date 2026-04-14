package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/pkg/logger"
)

// ---------------------------------------------------------------------------
// JobBuilder — 將 StepRun 轉換為 K8s Job spec
//
// 設計原則（CICD_ARCHITECTURE §7.7）：
//   - Pod Security Baseline：runAsNonRoot, drop ALL caps, seccomp RuntimeDefault
//   - automountServiceAccountToken：預設 false，僅 deploy 類型開啟
//   - Resource Limits：強制設定，防止資源逃逸
//   - Labels：統一標記供 Watcher label selector 使用
// ---------------------------------------------------------------------------

// JobBuilder 從 StepRun 產生 K8s Job 物件。
type JobBuilder struct {
	defaultCPURequest    string
	defaultCPULimit      string
	defaultMemoryRequest string
	defaultMemoryLimit   string
	defaultDiskLimit     string
}

// NewJobBuilder 建立 JobBuilder，使用預設資源限制。
func NewJobBuilder() *JobBuilder {
	return &JobBuilder{
		defaultCPURequest:    "100m",
		defaultCPULimit:      "1000m",
		defaultMemoryRequest: "256Mi",
		defaultMemoryLimit:   "2Gi",
		defaultDiskLimit:     "4Gi",
	}
}

// StepConfig 從 StepRun.ConfigJSON 解析的 Step 設定。
type StepConfig struct {
	// Resource overrides
	CPURequest    string `json:"cpu_request"`
	CPULimit      string `json:"cpu_limit"`
	MemoryRequest string `json:"memory_request"`
	MemoryLimit   string `json:"memory_limit"`
	DiskLimit     string `json:"disk_limit"`

	// Security overrides
	ReadOnlyRootFS           *bool  `json:"read_only_root_fs"`
	AutomountServiceAccount  *bool  `json:"automount_service_account"`
	ServiceAccount           string `json:"service_account"`

	// Execution
	Args          []string          `json:"args"`
	Env           map[string]string `json:"env"`
	WorkingDir    string            `json:"working_dir"`
	TimeoutSec    int               `json:"timeout_seconds"`

	// Istio ambient
	DisableIstioRedirection *bool `json:"disable_istio_redirection"`
}

// BuildJobInput 建構 Job 所需的全部輸入。
type BuildJobInput struct {
	Run         *models.PipelineRun
	StepRun     *models.StepRun
	Namespace   string            // Job 部署到的 namespace
	Secrets     map[string]string // 已解析的 secret key→value
	SecretName  string            // K8s Secret 名稱（由 EnsureRunSecret 建立），用 envFrom 掛載
}

// BuildJob 將 StepRun 轉換為 K8s Job。
func (b *JobBuilder) BuildJob(input *BuildJobInput) (*batchv1.Job, error) {
	cfg := b.parseConfig(input.StepRun.ConfigJSON)

	jobName := fmt.Sprintf("pr-%d-step-%d-%s",
		input.Run.ID, input.StepRun.ID,
		sanitizeK8sName(input.StepRun.StepName))
	if len(jobName) > 63 {
		jobName = jobName[:63]
	}

	labels := map[string]string{
		"synapse.io/pipeline-run-id":  fmt.Sprintf("%d", input.Run.ID),
		"synapse.io/step-run-id":      fmt.Sprintf("%d", input.StepRun.ID),
		"synapse.io/pipeline-step":    "true",
		"synapse.io/step-type":        input.StepRun.StepType,
	}

	annotations := make(map[string]string)

	// Istio ambient：外部連線類 Step 自動加 annotation
	if b.shouldDisableIstioRedirection(input.StepRun.StepType, cfg) {
		annotations["ambient.istio.io/redirection"] = "disabled"
	}

	// Security context
	runAsNonRoot := true
	var runAsUser, runAsGroup, fsGroup int64 = 1000, 1000, 1000
	readOnlyRootFS := true
	// Kaniko (build-image) 需要寫入 root filesystem → 自動設為 false
	if input.StepRun.StepType == "build-image" {
		readOnlyRootFS = false
	}
	if cfg.ReadOnlyRootFS != nil {
		readOnlyRootFS = *cfg.ReadOnlyRootFS // 使用者明確設定覆蓋預設
	}
	allowPrivilegeEscalation := false

	// automountServiceAccountToken：僅 deploy 類開啟
	automount := false
	if b.isDeployStepType(input.StepRun.StepType) && cfg.ServiceAccount != "" {
		automount = true
	}
	if cfg.AutomountServiceAccount != nil {
		automount = *cfg.AutomountServiceAccount
	}

	// 環境變數：config env（secrets 透過 envFrom K8s Secret 注入）
	envVars := b.buildEnvVars(cfg, nil)

	// envFrom：掛載 K8s Secret（如有）
	var envFrom []corev1.EnvFromSource
	if input.SecretName != "" {
		envFrom = append(envFrom, corev1.EnvFromSource{
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: input.SecretName},
			},
		})
	}

	// Command：使用 Step 類型特定邏輯產生
	stepDef := &StepDef{
		Name:    input.StepRun.StepName,
		Type:    input.StepRun.StepType,
		Image:   input.StepRun.Image,
		Command: input.StepRun.Command,
		Config:  input.StepRun.ConfigJSON,
	}
	command, generatedArgs := GenerateCommand(stepDef)
	// 合併：GenerateCommand 產生的 args + StepConfig 的 args
	if len(generatedArgs) > 0 {
		cfg.Args = append(generatedArgs, cfg.Args...)
	}

	// Resource limits
	resources := b.buildResources(cfg)

	// vSphere CSI 拓撲約束
	var nodeSelector map[string]string
	if input.Run.BoundNodeName != "" {
		nodeSelector = map[string]string{
			"kubernetes.io/hostname": input.Run.BoundNodeName,
		}
	}

	// ServiceAccount
	saName := cfg.ServiceAccount

	// imagePullPolicy：latest tag → Always，其餘 → IfNotPresent
	imagePullPolicy := corev1.PullIfNotPresent
	if strings.HasSuffix(input.StepRun.Image, ":latest") || !strings.Contains(input.StepRun.Image, ":") {
		imagePullPolicy = corev1.PullAlways
	}

	// terminationGracePeriodSeconds
	var terminationGracePeriod int64 = 30
	if cfg.TimeoutSec > 0 && cfg.TimeoutSec < 30 {
		terminationGracePeriod = int64(cfg.TimeoutSec)
	}

	// backoffLimit = maxRetries（K8s Job 內部重試）
	backoffLimit := int32(input.StepRun.MaxRetries)

	// TTL: auto-cleanup completed Jobs (10 min)
	ttlAfterFinished := int32(600)

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:        jobName,
			Namespace:   input.Namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoffLimit,
			TTLSecondsAfterFinished: &ttlAfterFinished,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labels,
					Annotations: annotations,
				},
				Spec: corev1.PodSpec{
					RestartPolicy:                    corev1.RestartPolicyNever,
					AutomountServiceAccountToken:     &automount,
					ServiceAccountName:               saName,
					NodeSelector:                     nodeSelector,
					TerminationGracePeriodSeconds:    &terminationGracePeriod,
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: &runAsNonRoot,
						RunAsUser:    &runAsUser,
						RunAsGroup:   &runAsGroup,
						FSGroup:      &fsGroup,
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
						},
					},
					Containers: []corev1.Container{
						{
							Name:            "step",
							Image:           input.StepRun.Image,
							ImagePullPolicy: imagePullPolicy,
							Command:         command,
							Args:            cfg.Args,
							WorkingDir:      cfg.WorkingDir,
							Env:             envVars,
							EnvFrom:         envFrom,
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: &allowPrivilegeEscalation,
								ReadOnlyRootFilesystem:   &readOnlyRootFS,
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{"ALL"},
								},
							},
							Resources: resources,
							VolumeMounts: []corev1.VolumeMount{
								{Name: "workspace", MountPath: "/workspace"},
								{Name: "tmp", MountPath: "/tmp"},
							},
						},
					},
					Volumes: []corev1.Volume{
						{Name: "workspace", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
						{Name: "tmp", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
					},
				},
			},
		},
	}

	logger.Debug("job built",
		"job_name", jobName,
		"step_type", input.StepRun.StepType,
		"image", input.StepRun.Image,
		"automount_sa", automount,
		"read_only_root_fs", readOnlyRootFS,
	)

	return job, nil
}

// SubmitJob 建構並提交 K8s Job，回寫 JobName/JobNamespace 到 StepRun。
// 流程：建立 K8s Secret（如有 secrets）→ 建立 Job → 設定 Secret ownerRef → Job GC 時自動清理。
func (b *JobBuilder) SubmitJob(ctx context.Context, k8sClient *K8sClient, input *BuildJobInput) error {
	// Step 1: 建立 K8s Secret（如有 secrets）
	secretName, err := b.EnsureRunSecret(ctx, k8sClient,
		input.Run, input.StepRun, input.Namespace, input.Secrets)
	if err != nil {
		return fmt.Errorf("ensure run secret for step %s: %w", input.StepRun.StepName, err)
	}
	input.SecretName = secretName

	// Step 2: 建構並建立 Job
	job, err := b.BuildJob(input)
	if err != nil {
		return fmt.Errorf("build job for step %s: %w", input.StepRun.StepName, err)
	}

	created, err := k8sClient.GetClientset().BatchV1().
		Jobs(job.Namespace).
		Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("submit k8s job %s: %w", job.Name, err)
	}

	// Step 3: 設定 Secret ownerRef → Job（自動 GC）
	if err := b.SetSecretOwnerRef(ctx, k8sClient, input.Namespace, secretName, created); err != nil {
		// ownerRef 設定失敗不阻塞 pipeline，但記錄警告
		logger.Warn("failed to set secret ownerRef, secret may leak",
			"secret_name", secretName,
			"job_name", created.Name,
			"error", err,
		)
	}

	// 回寫 Job 資訊到 StepRun（呼叫者負責持久化）
	input.StepRun.JobName = created.Name
	input.StepRun.JobNamespace = created.Namespace

	logger.Info("k8s job submitted",
		"job_name", created.Name,
		"namespace", created.Namespace,
		"secret_name", secretName,
		"step_run_id", input.StepRun.ID,
		"pipeline_run_id", input.Run.ID,
	)
	return nil
}

// EnsureRunSecret 建立 K8s Secret 存放已解析的 pipeline secrets。
// 回傳 Secret 名稱，供 BuildJobInput.SecretName 使用。
// 若 secrets 為空，回傳空字串（不建立 Secret）。
func (b *JobBuilder) EnsureRunSecret(
	ctx context.Context,
	k8sClient *K8sClient,
	run *models.PipelineRun,
	stepRun *models.StepRun,
	namespace string,
	secrets map[string]string,
) (string, error) {
	if len(secrets) == 0 {
		return "", nil
	}

	secretName := fmt.Sprintf("pr-%d-step-%d-secrets",
		run.ID, stepRun.ID)
	if len(secretName) > 63 {
		secretName = secretName[:63]
	}

	stringData := make(map[string]string, len(secrets))
	for k, v := range secrets {
		stringData[k] = v
	}

	k8sSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
			Labels: map[string]string{
				"synapse.io/pipeline-run-id": fmt.Sprintf("%d", run.ID),
				"synapse.io/step-run-id":     fmt.Sprintf("%d", stepRun.ID),
				"synapse.io/managed-by":      "synapse-pipeline",
			},
		},
		Type:       corev1.SecretTypeOpaque,
		StringData: stringData,
	}

	created, err := k8sClient.GetClientset().CoreV1().
		Secrets(namespace).
		Create(ctx, k8sSecret, metav1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("create k8s secret %s: %w", secretName, err)
	}

	logger.Info("k8s secret created for step",
		"secret_name", created.Name,
		"namespace", namespace,
		"step_run_id", stepRun.ID,
		"pipeline_run_id", run.ID,
	)
	return created.Name, nil
}

// SetSecretOwnerRef 在 Job 建立後，將 K8s Secret 的 ownerRef 設為該 Job，
// 使 Job 刪除時 Secret 自動 GC。
func (b *JobBuilder) SetSecretOwnerRef(
	ctx context.Context,
	k8sClient *K8sClient,
	namespace, secretName string,
	job *batchv1.Job,
) error {
	if secretName == "" {
		return nil
	}

	secret, err := k8sClient.GetClientset().CoreV1().
		Secrets(namespace).
		Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get secret %s for ownerRef: %w", secretName, err)
	}

	isController := true
	secret.OwnerReferences = []metav1.OwnerReference{
		{
			APIVersion: "batch/v1",
			Kind:       "Job",
			Name:       job.Name,
			UID:        job.UID,
			Controller: &isController,
		},
	}

	_, err = k8sClient.GetClientset().CoreV1().
		Secrets(namespace).
		Update(ctx, secret, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("set ownerRef on secret %s: %w", secretName, err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

func (b *JobBuilder) parseConfig(configJSON string) StepConfig {
	var cfg StepConfig
	if configJSON != "" {
		if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
			logger.Warn("failed to parse step config JSON, using defaults", "error", err)
		}
	}
	return cfg
}

func (b *JobBuilder) buildResources(cfg StepConfig) corev1.ResourceRequirements {
	cpuReq := b.defaultCPURequest
	cpuLim := b.defaultCPULimit
	memReq := b.defaultMemoryRequest
	memLim := b.defaultMemoryLimit
	diskLim := b.defaultDiskLimit

	if cfg.CPURequest != "" {
		cpuReq = cfg.CPURequest
	}
	if cfg.CPULimit != "" {
		cpuLim = cfg.CPULimit
	}
	if cfg.MemoryRequest != "" {
		memReq = cfg.MemoryRequest
	}
	if cfg.MemoryLimit != "" {
		memLim = cfg.MemoryLimit
	}
	if cfg.DiskLimit != "" {
		diskLim = cfg.DiskLimit
	}

	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(cpuReq),
			corev1.ResourceMemory: resource.MustParse(memReq),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:              resource.MustParse(cpuLim),
			corev1.ResourceMemory:           resource.MustParse(memLim),
			corev1.ResourceEphemeralStorage: resource.MustParse(diskLim),
		},
	}
}

func (b *JobBuilder) buildEnvVars(cfg StepConfig, secrets map[string]string) []corev1.EnvVar {
	envVars := make([]corev1.EnvVar, 0, len(cfg.Env)+len(secrets))

	for k, v := range cfg.Env {
		envVars = append(envVars, corev1.EnvVar{Name: k, Value: v})
	}
	for k, v := range secrets {
		envVars = append(envVars, corev1.EnvVar{Name: k, Value: v})
	}
	return envVars
}

// shouldDisableIstioRedirection 判斷是否需要停用 Istio ambient 重導向。
// build-image、push-image、trivy-scan 需要外部連線，自動停用。
func (b *JobBuilder) shouldDisableIstioRedirection(stepType string, cfg StepConfig) bool {
	if cfg.DisableIstioRedirection != nil {
		return *cfg.DisableIstioRedirection
	}
	switch stepType {
	case "build-image", "push-image", "trivy-scan", "build-jar":
		return true
	default:
		return false
	}
}

// isDeployStepType 判斷是否為需要叢集 API 存取的 deploy 類型。
func (b *JobBuilder) isDeployStepType(stepType string) bool {
	switch stepType {
	case "deploy", "deploy-helm", "deploy-argocd-sync", "deploy-rollout",
		"rollout-promote", "rollout-abort", "rollout-status", "gitops-sync":
		return true
	default:
		return false
	}
}

// sanitizeK8sName 將名稱轉為合法的 K8s 資源名稱片段。
func sanitizeK8sName(name string) string {
	name = strings.ToLower(name)
	name = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		return '-'
	}, name)
	// trim leading/trailing hyphens
	name = strings.Trim(name, "-")
	if len(name) > 30 {
		name = name[:30]
	}
	return name
}
