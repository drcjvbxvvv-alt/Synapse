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

	// ── M13b 進階類型 ────────────────────────────────────────────────────
	"build-jar":          {Name: "build-jar", DefaultImage: "maven:3.9-eclipse-temurin-17", RequiresCommand: false, Description: "Build Java artifact with Maven/Gradle"},
	"trivy-scan":         {Name: "trivy-scan", DefaultImage: "aquasec/trivy:0.58.0", RequiresCommand: false, Description: "Run Trivy vulnerability scan on container image"},
	"push-image":         {Name: "push-image", DefaultImage: "gcr.io/go-containerregistry/crane:latest", RequiresCommand: false, Description: "Push/retag container image via crane"},
	"deploy-helm":        {Name: "deploy-helm", DefaultImage: "alpine/helm:3.16", RequiresCommand: false, Description: "Deploy via Helm upgrade --install"},
	"deploy-argocd-sync": {Name: "deploy-argocd-sync", DefaultImage: "quay.io/argoproj/argocd:v2.13.0", RequiresCommand: false, Description: "Trigger ArgoCD application sync"},
	"notify":             {Name: "notify", DefaultImage: "curlimages/curl:8.11.0", RequiresCommand: false, Description: "Send webhook notification (Slack, Teams, generic)"},
	"deploy-rollout":     {Name: "deploy-rollout", DefaultImage: "argoproj/argo-rollouts:v1.7.2", RequiresCommand: false, Description: "Trigger Argo Rollout canary/blue-green"},
	"rollout-promote":    {Name: "rollout-promote", DefaultImage: "argoproj/argo-rollouts:v1.7.2", RequiresCommand: false, Description: "Promote Argo Rollout to next step"},
	"rollout-abort":      {Name: "rollout-abort", DefaultImage: "argoproj/argo-rollouts:v1.7.2", RequiresCommand: false, Description: "Abort Argo Rollout"},
	"rollout-status":     {Name: "rollout-status", DefaultImage: "argoproj/argo-rollouts:v1.7.2", RequiresCommand: false, Description: "Wait for Argo Rollout status"},
	"gitops-sync":        {Name: "gitops-sync", DefaultImage: "bitnami/git:2.47", RequiresCommand: false, Description: "Git commit + push for GitOps"},
	"shell":              {Name: "shell", DefaultImage: "alpine:3.20", RequiresCommand: true, Description: "Run shell commands (alias for run-script)"},
	"custom":             {Name: "custom", DefaultImage: "", RequiresCommand: true, Description: "Custom step with user-provided image and command"},
	"approval":           {Name: "approval", DefaultImage: "", RequiresCommand: false, Description: "Manual approval gate — pauses pipeline until approved or rejected"},
	"smoke-test":         {Name: "smoke-test", DefaultImage: "curlimages/curl:8.11.0", RequiresCommand: false, Description: "HTTP smoke test — verify endpoint returns expected status after deployment"},
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

// TrivyScanConfig trivy-scan Step 的類型特定設定。
type TrivyScanConfig struct {
	Image      string `json:"image"`       // 掃描目標 image（必填）
	Severity   string `json:"severity"`    // 篩選嚴重程度（如 "HIGH,CRITICAL"，預設全部）
	ExitCode   int    `json:"exit_code"`   // 發現漏洞時的退出碼（預設 1）
	IgnoreFile string `json:"ignore_file"` // .trivyignore 檔案路徑
	Format     string `json:"format"`      // 輸出格式（table/json/sarif，預設 table）
}

// PushImageConfig push-image Step 的類型特定設定。
type PushImageConfig struct {
	Source      string `json:"source"`      // 來源 image（必填）
	Destination string `json:"destination"` // 目標 image（必填）
	Registry    string `json:"registry"`    // Registry 名稱（可選，自動注入認證）
}

// HelmDeployConfig deploy-helm Step 的類型特定設定。
type HelmDeployConfig struct {
	Release   string            `json:"release"`   // Release 名稱（必填）
	Chart     string            `json:"chart"`     // Chart 路徑或 repo/chart（必填）
	Namespace string            `json:"namespace"` // 部署目標 namespace
	Values    string            `json:"values"`    // values.yaml 檔案路徑
	SetValues map[string]string `json:"set"`       // --set key=value 參數
	Version   string            `json:"version"`   // Chart 版本
	Wait      bool              `json:"wait"`      // 等待 rollout 完成
	Timeout   string            `json:"timeout"`   // helm timeout（如 "5m"）
	DryRun    bool              `json:"dry_run"`   // 是否 dry-run
}

// ArgoCDSyncConfig deploy-argocd-sync Step 的類型特定設定。
type ArgoCDSyncConfig struct {
	AppName   string `json:"app_name"`   // ArgoCD 應用名稱（必填）
	Server    string `json:"server"`     // ArgoCD server URL（預設 argocd-server.argocd.svc）
	Revision  string `json:"revision"`   // 同步到指定 revision
	Prune     bool   `json:"prune"`      // 是否清除多餘資源
	DryRun    bool   `json:"dry_run"`    // 是否 dry-run
	Wait      bool   `json:"wait"`       // 等待同步完成
	Timeout   string `json:"timeout"`    // 等待超時（如 "5m"）
	Insecure  bool   `json:"insecure"`   // 跳過 TLS 驗證（內部叢集用）
}

// DeployRolloutConfig deploy-rollout Step 的類型特定設定。
type DeployRolloutConfig struct {
	RolloutName  string `json:"rollout_name"`    // Argo Rollout 名稱（必填）
	Namespace    string `json:"namespace"`        // Rollout 所在 namespace（必填）
	Image        string `json:"image"`            // 更新的 image（必填）
	WaitForReady bool   `json:"wait_for_ready"`   // 等待 Rollout 達到 Healthy/Paused
	Timeout      string `json:"timeout"`          // 等待超時（如 "30m"）
}

// RolloutPromoteConfig rollout-promote Step 的類型特定設定。
type RolloutPromoteConfig struct {
	RolloutName string `json:"rollout_name"` // Argo Rollout 名稱（必填）
	Namespace   string `json:"namespace"`    // Rollout 所在 namespace（必填）
	Full        bool   `json:"full"`         // 是否全量 promote（跳過所有剩餘步驟）
}

// RolloutAbortConfig rollout-abort Step 的類型特定設定。
type RolloutAbortConfig struct {
	RolloutName string `json:"rollout_name"` // Argo Rollout 名稱（必填）
	Namespace   string `json:"namespace"`    // Rollout 所在 namespace（必填）
}

// RolloutStatusConfig rollout-status Step 的類型特定設定。
type RolloutStatusConfig struct {
	RolloutName    string `json:"rollout_name"`    // Argo Rollout 名稱（必填）
	Namespace      string `json:"namespace"`        // Rollout 所在 namespace（必填）
	ExpectedStatus string `json:"expected_status"`  // 期望狀態：healthy / paused / progressing
	Timeout        string `json:"timeout"`          // 等待超時（如 "30m"）
	OnTimeout      string `json:"on_timeout"`       // timeout 行為：abort / fail（預設 fail）
}

// BuildJarConfig build-jar Step 的類型特定設定。
type BuildJarConfig struct {
	BuildTool string            `json:"build_tool"` // "maven" (default) | "gradle"
	Goals     string            `json:"goals"`      // Maven goals（如 "clean package"，預設 "clean package -DskipTests"）
	Tasks     string            `json:"tasks"`      // Gradle tasks（如 "clean build"，預設 "clean build -x test"）
	PomFile   string            `json:"pom_file"`   // Maven POM 路徑（預設 "pom.xml"）
	BuildFile string            `json:"build_file"` // Gradle build 檔路徑（預設 auto-detect）
	Profiles  []string          `json:"profiles"`   // Maven profiles（如 ["prod", "docker"]）
	Properties map[string]string `json:"properties"` // -D properties
	JavaVersion string          `json:"java_version"` // 若指定，覆蓋 image（如 "21" → maven:3.9-eclipse-temurin-21）
	CacheDir  string            `json:"cache_dir"`  // Maven/Gradle cache 目錄（預設 /workspace/.m2 或 /workspace/.gradle）
}

// NotifyConfig notify Step 的類型特定設定。
type NotifyConfig struct {
	URL     string            `json:"url"`     // Webhook URL（必填）
	Method  string            `json:"method"`  // HTTP method（預設 POST）
	Headers map[string]string `json:"headers"` // 自訂 headers
	Body    string            `json:"body"`    // 自訂 body template（JSON 字串）
}

// SmokeTestConfig smoke-test Step 的類型特定設定。
type SmokeTestConfig struct {
	URL            string            `json:"url"`             // 目標 URL（必填）
	Method         string            `json:"method"`          // HTTP method（預設 GET）
	ExpectedStatus int               `json:"expected_status"` // 期望的 HTTP 狀態碼（預設 200）
	Headers        map[string]string `json:"headers"`         // 自訂 headers
	Body           string            `json:"body"`            // 請求 body（POST/PUT 用）
	Timeout        int               `json:"timeout"`         // 單次請求超時秒數（預設 10）
	Retries        int               `json:"retries"`         // 重試次數（預設 3）
	RetryInterval  int               `json:"retry_interval"`  // 重試間隔秒數（預設 5）
	Insecure       bool              `json:"insecure"`        // 跳過 TLS 驗證
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
	case "trivy-scan":
		return validateTrivyScanStep(step)
	case "push-image":
		return validatePushImageStep(step)
	case "deploy-helm":
		return validateHelmDeployStep(step)
	case "deploy-argocd-sync":
		return validateArgoCDSyncStep(step)
	case "build-jar":
		return validateBuildJarStep(step)
	case "notify":
		return validateNotifyStep(step)
	case "deploy-rollout":
		return validateDeployRolloutStep(step)
	case "rollout-promote":
		return validateRolloutPromoteStep(step)
	case "rollout-abort":
		return validateRolloutAbortStep(step)
	case "rollout-status":
		return validateRolloutStatusStep(step)
	case "smoke-test":
		return validateSmokeTestStep(step)
	}

	return nil
}

func validateBuildImageStep(step *StepDef) error {
	if len(step.Config) == 0 {
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
	if len(step.Config) == 0 {
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

func validateTrivyScanStep(step *StepDef) error {
	if len(step.Config) == 0 {
		return fmt.Errorf("step %q (trivy-scan): config with image is required", step.Name)
	}
	var cfg TrivyScanConfig
	if err := parseJSON(step.Config, &cfg); err != nil {
		return fmt.Errorf("step %q (trivy-scan): invalid config: %w", step.Name, err)
	}
	if cfg.Image == "" {
		return fmt.Errorf("step %q (trivy-scan): config.image is required", step.Name)
	}
	return nil
}

func validatePushImageStep(step *StepDef) error {
	if len(step.Config) == 0 {
		return fmt.Errorf("step %q (push-image): config with source and destination is required", step.Name)
	}
	var cfg PushImageConfig
	if err := parseJSON(step.Config, &cfg); err != nil {
		return fmt.Errorf("step %q (push-image): invalid config: %w", step.Name, err)
	}
	if cfg.Source == "" {
		return fmt.Errorf("step %q (push-image): config.source is required", step.Name)
	}
	if cfg.Destination == "" {
		return fmt.Errorf("step %q (push-image): config.destination is required", step.Name)
	}
	return nil
}

func validateHelmDeployStep(step *StepDef) error {
	if len(step.Config) == 0 {
		if step.Command != "" {
			return nil
		}
		return fmt.Errorf("step %q (deploy-helm): config with release and chart is required", step.Name)
	}
	var cfg HelmDeployConfig
	if err := parseJSON(step.Config, &cfg); err != nil {
		return fmt.Errorf("step %q (deploy-helm): invalid config: %w", step.Name, err)
	}
	if cfg.Release == "" {
		return fmt.Errorf("step %q (deploy-helm): config.release is required", step.Name)
	}
	if cfg.Chart == "" {
		return fmt.Errorf("step %q (deploy-helm): config.chart is required", step.Name)
	}
	return nil
}

func validateArgoCDSyncStep(step *StepDef) error {
	if len(step.Config) == 0 {
		if step.Command != "" {
			return nil
		}
		return fmt.Errorf("step %q (deploy-argocd-sync): config with app_name is required", step.Name)
	}
	var cfg ArgoCDSyncConfig
	if err := parseJSON(step.Config, &cfg); err != nil {
		return fmt.Errorf("step %q (deploy-argocd-sync): invalid config: %w", step.Name, err)
	}
	if cfg.AppName == "" {
		return fmt.Errorf("step %q (deploy-argocd-sync): config.app_name is required", step.Name)
	}
	return nil
}

func validateNotifyStep(step *StepDef) error {
	if len(step.Config) == 0 {
		return fmt.Errorf("step %q (notify): config with url is required", step.Name)
	}
	var cfg NotifyConfig
	if err := parseJSON(step.Config, &cfg); err != nil {
		return fmt.Errorf("step %q (notify): invalid config: %w", step.Name, err)
	}
	if cfg.URL == "" {
		return fmt.Errorf("step %q (notify): config.url is required", step.Name)
	}
	return nil
}

// ---------------------------------------------------------------------------
// build-jar validation
// ---------------------------------------------------------------------------

func validateBuildJarStep(step *StepDef) error {
	if len(step.Config) == 0 {
		// 無 config 時使用 Maven 預設值，合法
		return nil
	}
	var cfg BuildJarConfig
	if err := parseJSON(step.Config, &cfg); err != nil {
		return fmt.Errorf("step %q (build-jar): invalid config: %w", step.Name, err)
	}
	if cfg.BuildTool != "" && cfg.BuildTool != "maven" && cfg.BuildTool != "gradle" {
		return fmt.Errorf("step %q (build-jar): build_tool must be maven|gradle, got %q", step.Name, cfg.BuildTool)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Rollout validation
// ---------------------------------------------------------------------------

func validateDeployRolloutStep(step *StepDef) error {
	if len(step.Config) == 0 {
		return fmt.Errorf("step %q (deploy-rollout): config with rollout_name, namespace, image is required", step.Name)
	}
	var cfg DeployRolloutConfig
	if err := parseJSON(step.Config, &cfg); err != nil {
		return fmt.Errorf("step %q (deploy-rollout): invalid config: %w", step.Name, err)
	}
	if cfg.RolloutName == "" {
		return fmt.Errorf("step %q (deploy-rollout): config.rollout_name is required", step.Name)
	}
	if cfg.Namespace == "" {
		return fmt.Errorf("step %q (deploy-rollout): config.namespace is required", step.Name)
	}
	if cfg.Image == "" {
		return fmt.Errorf("step %q (deploy-rollout): config.image is required", step.Name)
	}
	return nil
}

func validateRolloutPromoteStep(step *StepDef) error {
	if len(step.Config) == 0 {
		return fmt.Errorf("step %q (rollout-promote): config with rollout_name and namespace is required", step.Name)
	}
	var cfg RolloutPromoteConfig
	if err := parseJSON(step.Config, &cfg); err != nil {
		return fmt.Errorf("step %q (rollout-promote): invalid config: %w", step.Name, err)
	}
	if cfg.RolloutName == "" {
		return fmt.Errorf("step %q (rollout-promote): config.rollout_name is required", step.Name)
	}
	if cfg.Namespace == "" {
		return fmt.Errorf("step %q (rollout-promote): config.namespace is required", step.Name)
	}
	return nil
}

func validateRolloutAbortStep(step *StepDef) error {
	if len(step.Config) == 0 {
		return fmt.Errorf("step %q (rollout-abort): config with rollout_name and namespace is required", step.Name)
	}
	var cfg RolloutAbortConfig
	if err := parseJSON(step.Config, &cfg); err != nil {
		return fmt.Errorf("step %q (rollout-abort): invalid config: %w", step.Name, err)
	}
	if cfg.RolloutName == "" {
		return fmt.Errorf("step %q (rollout-abort): config.rollout_name is required", step.Name)
	}
	if cfg.Namespace == "" {
		return fmt.Errorf("step %q (rollout-abort): config.namespace is required", step.Name)
	}
	return nil
}

func validateRolloutStatusStep(step *StepDef) error {
	if len(step.Config) == 0 {
		return fmt.Errorf("step %q (rollout-status): config with rollout_name and namespace is required", step.Name)
	}
	var cfg RolloutStatusConfig
	if err := parseJSON(step.Config, &cfg); err != nil {
		return fmt.Errorf("step %q (rollout-status): invalid config: %w", step.Name, err)
	}
	if cfg.RolloutName == "" {
		return fmt.Errorf("step %q (rollout-status): config.rollout_name is required", step.Name)
	}
	if cfg.Namespace == "" {
		return fmt.Errorf("step %q (rollout-status): config.namespace is required", step.Name)
	}
	if cfg.ExpectedStatus != "" {
		valid := map[string]bool{"healthy": true, "paused": true, "progressing": true}
		if !valid[cfg.ExpectedStatus] {
			return fmt.Errorf("step %q (rollout-status): config.expected_status must be healthy|paused|progressing, got %q", step.Name, cfg.ExpectedStatus)
		}
	}
	if cfg.OnTimeout != "" {
		valid := map[string]bool{"abort": true, "fail": true}
		if !valid[cfg.OnTimeout] {
			return fmt.Errorf("step %q (rollout-status): config.on_timeout must be abort|fail, got %q", step.Name, cfg.OnTimeout)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// smoke-test Step（M17）
// ---------------------------------------------------------------------------

func validateSmokeTestStep(step *StepDef) error {
	if len(step.Config) == 0 {
		return fmt.Errorf("step %q (smoke-test): config with url is required", step.Name)
	}
	var cfg SmokeTestConfig
	if err := parseJSON(step.Config, &cfg); err != nil {
		return fmt.Errorf("step %q (smoke-test): invalid config: %w", step.Name, err)
	}
	if cfg.URL == "" {
		return fmt.Errorf("step %q (smoke-test): config.url is required", step.Name)
	}
	if cfg.ExpectedStatus < 0 || cfg.ExpectedStatus > 599 {
		return fmt.Errorf("step %q (smoke-test): config.expected_status must be 100-599, got %d", step.Name, cfg.ExpectedStatus)
	}
	if cfg.Retries < 0 || cfg.Retries > 30 {
		return fmt.Errorf("step %q (smoke-test): config.retries must be 0-30, got %d", step.Name, cfg.Retries)
	}
	if cfg.Timeout < 0 || cfg.Timeout > 300 {
		return fmt.Errorf("step %q (smoke-test): config.timeout must be 0-300, got %d", step.Name, cfg.Timeout)
	}
	return nil
}

// ResolveImage 解析 Step 的 image：使用者指定優先，否則使用預設。
// build-jar 類型支援 java_version 覆蓋（如 "21" → maven:3.9-eclipse-temurin-21）。
func ResolveImage(step *StepDef) string {
	if step.Image != "" {
		return step.Image
	}

	// build-jar: 根據 config 中的 java_version 和 build_tool 選擇 image
	if step.Type == "build-jar" && len(step.Config) > 0 {
		var cfg BuildJarConfig
		if err := parseJSON(step.Config, &cfg); err == nil {
			javaVer := defaultString(cfg.JavaVersion, "17")
			if cfg.BuildTool == "gradle" {
				return "gradle:8.10-jdk" + javaVer
			}
			return "maven:3.9-eclipse-temurin-" + javaVer
		}
	}

	if info, ok := stepTypeRegistry[step.Type]; ok && info.DefaultImage != "" {
		return info.DefaultImage
	}
	return ""
}

// IsDeployStepType 判斷 stepType 是否為需要叢集 API 存取的 deploy 類型。
// Deploy 類型在回滾執行時會被保留（非 deploy 類型在回滾時跳過）。
func IsDeployStepType(stepType string) bool {
	switch stepType {
	case "deploy", "deploy-helm", "deploy-argocd-sync", "deploy-rollout",
		"rollout-promote", "rollout-abort", "rollout-status", "gitops-sync":
		return true
	}
	return false
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

func parseJSON(raw json.RawMessage, out interface{}) error {
	if len(raw) == 0 {
		return fmt.Errorf("empty JSON")
	}
	// If the value is a JSON string (e.g. "{\"key\":\"val\"}"), unwrap it first.
	if raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return err
		}
		return json.Unmarshal([]byte(s), out)
	}
	// Otherwise it's a direct JSON object/array — unmarshal as-is.
	return json.Unmarshal(raw, out)
}
