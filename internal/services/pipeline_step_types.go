package services

import (
	"encoding/json"
	"fmt"
)

// ---------------------------------------------------------------------------
// Step Type Registry — 定義支援的 Step 類型、預設 Image、Command 模板
//
// 設計原則（CICD_ARCHITECTURE §8.1, P1-2）：
//   - 每個 Step 類型有預設 Image（使用者可覆蓋）
//   - Command 模板依類型產生（Kaniko 用 /kaniko/executor，deploy 用 kubectl）
//   - Validation 確保必要欄位存在
// ---------------------------------------------------------------------------

// StepTypeInfo 描述一個 Step 類型的元資料。
type StepTypeInfo struct {
	Name            string // 類型名稱（如 "build-image"）
	DefaultImage    string // 預設容器映像
	RequiresCommand bool   // 是否需要使用者提供 command
	Description     string // 類型描述
}

// stepTypeRegistry 已註冊的 Step 類型。
var stepTypeRegistry = map[string]StepTypeInfo{
	// ── M13a 基本類型 ──────────────────────────────────────────────────
	"build-image": {
		Name:            "build-image",
		DefaultImage:    "gcr.io/kaniko-project/executor:v1.23.2",
		RequiresCommand: false, // command 由 BuildImageConfig 產生
		Description:     "Build container image using Kaniko (rootless, no Docker daemon)",
	},
	"deploy": {
		Name:            "deploy",
		DefaultImage:    "bitnami/kubectl:1.30",
		RequiresCommand: false, // command 由 DeployConfig 產生
		Description:     "Deploy to Kubernetes via kubectl apply",
	},
	"run-script": {
		Name:            "run-script",
		DefaultImage:    "alpine:3.20",
		RequiresCommand: true, // 使用者必須提供 command
		Description:     "Run custom shell script",
	},

	// ── M13b 進階類型（預定義，暫不實作 command 模板） ──────────────────
	"build-jar":          {Name: "build-jar", DefaultImage: "maven:3.9-eclipse-temurin-17", RequiresCommand: true, Description: "Build Java artifact with Maven/Gradle"},
	"trivy-scan":         {Name: "trivy-scan", DefaultImage: "aquasec/trivy:0.58.0", RequiresCommand: false, Description: "Run Trivy vulnerability scan"},
	"push-image":         {Name: "push-image", DefaultImage: "gcr.io/go-containerregistry/crane:latest", RequiresCommand: false, Description: "Push/retag container image"},
	"deploy-helm":        {Name: "deploy-helm", DefaultImage: "alpine/helm:3.16", RequiresCommand: false, Description: "Deploy via Helm chart"},
	"deploy-argocd-sync": {Name: "deploy-argocd-sync", DefaultImage: "argoproj/argocd:v2.13.0", RequiresCommand: false, Description: "Trigger ArgoCD sync"},
	"deploy-rollout":     {Name: "deploy-rollout", DefaultImage: "argoproj/argo-rollouts:v1.7.2", RequiresCommand: false, Description: "Trigger Argo Rollout canary/blue-green"},
	"rollout-promote":    {Name: "rollout-promote", DefaultImage: "argoproj/argo-rollouts:v1.7.2", RequiresCommand: false, Description: "Promote Argo Rollout to next step"},
	"rollout-abort":      {Name: "rollout-abort", DefaultImage: "argoproj/argo-rollouts:v1.7.2", RequiresCommand: false, Description: "Abort Argo Rollout"},
	"rollout-status":     {Name: "rollout-status", DefaultImage: "argoproj/argo-rollouts:v1.7.2", RequiresCommand: false, Description: "Wait for Argo Rollout status"},
	"gitops-sync":        {Name: "gitops-sync", DefaultImage: "bitnami/git:2.47", RequiresCommand: false, Description: "Git commit + push for GitOps"},
	"shell":              {Name: "shell", DefaultImage: "alpine:3.20", RequiresCommand: true, Description: "Run shell commands (alias for run-script)"},
	"custom":             {Name: "custom", DefaultImage: "", RequiresCommand: true, Description: "Custom step with user-provided image and command"},
	"approval":           {Name: "approval", DefaultImage: "", RequiresCommand: false, Description: "Manual approval gate — pauses pipeline until approved or rejected"},
}

// ---------------------------------------------------------------------------
// Type-specific config structures
// ---------------------------------------------------------------------------

// BuildImageConfig build-image Step 的類型特定設定。
type BuildImageConfig struct {
	Context     string `json:"context"`     // Build context 路徑（預設 "."）
	Dockerfile  string `json:"dockerfile"`  // Dockerfile 路徑（預設 "Dockerfile"）
	Destination string `json:"destination"` // 目標 image（必填）
	Cache       bool   `json:"cache"`       // 是否啟用 Kaniko layer cache
	CacheRepo   string `json:"cache_repo"`  // Cache repo（如 harbor.example.com/cache）
	BuildArgs   map[string]string `json:"build_args"` // Docker build args
}

// DeployConfig deploy Step 的類型特定設定。
type DeployConfig struct {
	Manifest  string `json:"manifest"`  // YAML 檔案路徑（必填）
	Namespace string `json:"namespace"` // 部署目標 namespace
	DryRun    bool   `json:"dry_run"`   // 是否 dry-run
}

// ---------------------------------------------------------------------------
// Validation
// ---------------------------------------------------------------------------

// ValidateStepDef 驗證 StepDef 的完整性。
func ValidateStepDef(step *StepDef) error {
	if step.Name == "" {
		return fmt.Errorf("step name is required")
	}
	if step.Type == "" {
		return fmt.Errorf("step %q: type is required", step.Name)
	}

	info, ok := stepTypeRegistry[step.Type]
	if !ok {
		return fmt.Errorf("step %q: unknown type %q", step.Name, step.Type)
	}

	// approval 類型不需要 image 和 command
	if step.Type == "approval" {
		return nil
	}

	// 如果使用者沒給 image，用預設值
	if step.Image == "" && info.DefaultImage == "" {
		return fmt.Errorf("step %q: image is required for type %q", step.Name, step.Type)
	}

	// 需要 command 的類型必須提供 command
	if info.RequiresCommand && step.Command == "" {
		return fmt.Errorf("step %q: command is required for type %q", step.Name, step.Type)
	}

	// 類型特定驗證
	switch step.Type {
	case "build-image":
		return validateBuildImageStep(step)
	case "deploy":
		return validateDeployStep(step)
	}

	return nil
}

func validateBuildImageStep(step *StepDef) error {
	if step.Config == "" {
		return fmt.Errorf("step %q (build-image): config with destination is required", step.Name)
	}
	var cfg BuildImageConfig
	if err := parseJSON(step.Config, &cfg); err != nil {
		return fmt.Errorf("step %q (build-image): invalid config: %w", step.Name, err)
	}
	if cfg.Destination == "" {
		return fmt.Errorf("step %q (build-image): config.destination is required", step.Name)
	}
	return nil
}

func validateDeployStep(step *StepDef) error {
	if step.Config == "" {
		// deploy 可以直接用 command 模式（如自訂 kubectl 指令）
		if step.Command != "" {
			return nil
		}
		return fmt.Errorf("step %q (deploy): config with manifest or command is required", step.Name)
	}
	var cfg DeployConfig
	if err := parseJSON(step.Config, &cfg); err != nil {
		return fmt.Errorf("step %q (deploy): invalid config: %w", step.Name, err)
	}
	if cfg.Manifest == "" && step.Command == "" {
		return fmt.Errorf("step %q (deploy): config.manifest or command is required", step.Name)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Command generation
// ---------------------------------------------------------------------------

// GenerateCommand 為特定 Step 類型產生容器執行指令。
// 若使用者已提供 command，則直接使用。
func GenerateCommand(step *StepDef) (command []string, args []string) {
	// 使用者自訂 command 優先
	if step.Command != "" {
		return []string{"/bin/sh", "-c", step.Command}, nil
	}

	switch step.Type {
	case "build-image":
		return generateBuildImageCommand(step)
	case "deploy":
		return generateDeployCommand(step)
	case "run-script", "shell":
		// run-script 必須有 command（已在 validation 檢查）
		return []string{"/bin/sh", "-c", "echo 'no command provided'"}, nil
	default:
		return nil, nil
	}
}

func generateBuildImageCommand(step *StepDef) ([]string, []string) {
	var cfg BuildImageConfig
	if err := parseJSON(step.Config, &cfg); err != nil {
		return []string{"/kaniko/executor"}, []string{"--help"}
	}

	args := []string{
		"--context=" + defaultString(cfg.Context, "/workspace"),
		"--dockerfile=" + defaultString(cfg.Dockerfile, "Dockerfile"),
		"--destination=" + cfg.Destination,
		"--snapshot-mode=redo",
		"--push-retry=3",
	}

	if cfg.Cache {
		args = append(args, "--cache=true")
		if cfg.CacheRepo != "" {
			args = append(args, "--cache-repo="+cfg.CacheRepo)
		}
	}

	for k, v := range cfg.BuildArgs {
		args = append(args, fmt.Sprintf("--build-arg=%s=%s", k, v))
	}

	// Kaniko 使用自己的 entrypoint，不需要 /bin/sh -c
	return []string{"/kaniko/executor"}, args
}

func generateDeployCommand(step *StepDef) ([]string, []string) {
	var cfg DeployConfig
	if err := parseJSON(step.Config, &cfg); err != nil {
		return []string{"/bin/sh", "-c", "kubectl apply -f /workspace"}, nil
	}

	manifest := defaultString(cfg.Manifest, "/workspace/deployment.yaml")
	cmd := "kubectl apply -f " + manifest

	if cfg.Namespace != "" {
		cmd += " -n " + cfg.Namespace
	}
	if cfg.DryRun {
		cmd += " --dry-run=server"
	}

	return []string{"/bin/sh", "-c", cmd}, nil
}

// ResolveImage 解析 Step 的 image：使用者指定優先，否則使用預設。
func ResolveImage(step *StepDef) string {
	if step.Image != "" {
		return step.Image
	}
	if info, ok := stepTypeRegistry[step.Type]; ok && info.DefaultImage != "" {
		return info.DefaultImage
	}
	return ""
}

// GetStepTypeInfo 取得 Step 類型資訊（供 API 回傳可用類型清單）。
func GetStepTypeInfo(stepType string) (StepTypeInfo, bool) {
	info, ok := stepTypeRegistry[stepType]
	return info, ok
}

// ListStepTypes 列出所有已註冊的 Step 類型。
func ListStepTypes() []StepTypeInfo {
	types := make([]StepTypeInfo, 0, len(stepTypeRegistry))
	for _, info := range stepTypeRegistry {
		types = append(types, info)
	}
	return types
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func defaultString(val, def string) string {
	if val == "" {
		return def
	}
	return val
}

func parseJSON(raw string, out interface{}) error {
	if raw == "" {
		return fmt.Errorf("empty JSON")
	}
	return json.Unmarshal([]byte(raw), out)
}
