package services

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
)

// ---------------------------------------------------------------------------
// Pipeline YAML Schema — JSON Schema v1 載入與驗證
//
// 設計原則（CICD_ARCHITECTURE 附錄 A, P3-7）：
//   - Schema 以 JSON Schema Draft 2020-12 定義
//   - 後端於 Pipeline 建立/更新時驗證
//   - 前端 YAML Editor 透過 API 取得同一份 schema 做即時 lint
//   - 嵌入 Go binary，避免運行時檔案依賴
// ---------------------------------------------------------------------------

//go:embed pipeline_schema/v1.json
var pipelineSchemaV1 []byte

// GetPipelineSchemaV1 回傳原始 JSON Schema bytes（供 API 端點回傳）。
func GetPipelineSchemaV1() []byte {
	return pipelineSchemaV1
}

// ---------------------------------------------------------------------------
// Lightweight structural validation（不依賴 JSON Schema library）
//
// 此驗證器檢查結構級別的必填欄位和類型約束。
// 完整 JSON Schema 驗證由前端或 CI pipeline 執行。
// ---------------------------------------------------------------------------

// PipelineYAMLDoc 代表解析後的 Pipeline YAML 文件。
type PipelineYAMLDoc struct {
	APIVersion string                 `json:"apiVersion" yaml:"apiVersion"`
	Kind       string                 `json:"kind" yaml:"kind"`
	Metadata   PipelineYAMLMetadata   `json:"metadata" yaml:"metadata"`
	Spec       PipelineYAMLSpec       `json:"spec" yaml:"spec"`
}

// PipelineYAMLMetadata 文件 metadata。
type PipelineYAMLMetadata struct {
	Name      string `json:"name" yaml:"name"`
	Cluster   string `json:"cluster" yaml:"cluster"`
	Namespace string `json:"namespace" yaml:"namespace"`
}

// PipelineYAMLSpec 文件 spec 區塊。
type PipelineYAMLSpec struct {
	Description      string                    `json:"description,omitempty" yaml:"description,omitempty"`
	Concurrency      *PipelineYAMLConcurrency  `json:"concurrency,omitempty" yaml:"concurrency,omitempty"`
	MaxConcurrentRuns *int                     `json:"max_concurrent_runs,omitempty" yaml:"max_concurrent_runs,omitempty"`
	Env              map[string]string          `json:"env,omitempty" yaml:"env,omitempty"`
	Triggers         []map[string]interface{}   `json:"triggers,omitempty" yaml:"triggers,omitempty"`
	Steps            []PipelineYAMLStep         `json:"steps" yaml:"steps"`
	Notifications    *PipelineYAMLNotifications `json:"notifications,omitempty" yaml:"notifications,omitempty"`
}

// PipelineYAMLConcurrency 並發控制設定。
type PipelineYAMLConcurrency struct {
	Group  string `json:"group,omitempty" yaml:"group,omitempty"`
	Policy string `json:"policy,omitempty" yaml:"policy,omitempty"`
}

// PipelineYAMLStep 步驟定義。
type PipelineYAMLStep struct {
	Name      string                 `json:"name" yaml:"name"`
	Type      string                 `json:"type" yaml:"type"`
	Image     string                 `json:"image,omitempty" yaml:"image,omitempty"`
	DependsOn []string               `json:"depends_on,omitempty" yaml:"depends_on,omitempty"`
	Command   string                 `json:"command,omitempty" yaml:"command,omitempty"`
	Config    map[string]interface{} `json:"config,omitempty" yaml:"config,omitempty"`
	Env       map[string]string      `json:"env,omitempty" yaml:"env,omitempty"`
	OnFailure string                 `json:"on_failure,omitempty" yaml:"on_failure,omitempty"`
	Retry     *PipelineYAMLRetry     `json:"retry,omitempty" yaml:"retry,omitempty"`
	Timeout   string                 `json:"timeout,omitempty" yaml:"timeout,omitempty"`
}

// PipelineYAMLRetry 重試設定。
type PipelineYAMLRetry struct {
	MaxAttempts    int `json:"max_attempts,omitempty" yaml:"max_attempts,omitempty"`
	BackoffSeconds int `json:"backoff_seconds,omitempty" yaml:"backoff_seconds,omitempty"`
}

// PipelineYAMLNotifications 通知設定。
type PipelineYAMLNotifications struct {
	OnSuccess      *PipelineYAMLNotifyTarget `json:"on_success,omitempty" yaml:"on_success,omitempty"`
	OnFailure      *PipelineYAMLNotifyTarget `json:"on_failure,omitempty" yaml:"on_failure,omitempty"`
	OnScanCritical *PipelineYAMLNotifyTarget `json:"on_scan_critical,omitempty" yaml:"on_scan_critical,omitempty"`
}

// PipelineYAMLNotifyTarget 通知目標。
type PipelineYAMLNotifyTarget struct {
	Channels []uint `json:"channels,omitempty" yaml:"channels,omitempty"`
}

// ValidatePipelineYAML 對 Pipeline YAML 文件進行結構驗證。
// 回傳錯誤訊息列表（空表示通過）。
func ValidatePipelineYAML(doc *PipelineYAMLDoc) []string {
	var errs []string

	// apiVersion / kind
	if doc.APIVersion != "synapse.io/v1" {
		errs = append(errs, fmt.Sprintf("apiVersion must be 'synapse.io/v1', got %q", doc.APIVersion))
	}
	if doc.Kind != "Pipeline" {
		errs = append(errs, fmt.Sprintf("kind must be 'Pipeline', got %q", doc.Kind))
	}

	// metadata
	if doc.Metadata.Name == "" {
		errs = append(errs, "metadata.name is required")
	} else if len(doc.Metadata.Name) > 255 {
		errs = append(errs, "metadata.name exceeds 255 characters")
	}
	if doc.Metadata.Cluster == "" {
		errs = append(errs, "metadata.cluster is required")
	}
	if doc.Metadata.Namespace == "" {
		errs = append(errs, "metadata.namespace is required")
	}

	// spec.steps
	if len(doc.Spec.Steps) == 0 {
		errs = append(errs, "spec.steps must contain at least one step")
	}
	if len(doc.Spec.Steps) > 50 {
		errs = append(errs, "spec.steps cannot exceed 50 steps")
	}

	// concurrency policy
	if doc.Spec.Concurrency != nil && doc.Spec.Concurrency.Policy != "" {
		validPolicies := map[string]bool{"cancel_previous": true, "queue": true, "reject": true}
		if !validPolicies[doc.Spec.Concurrency.Policy] {
			errs = append(errs, fmt.Sprintf("spec.concurrency.policy must be cancel_previous|queue|reject, got %q", doc.Spec.Concurrency.Policy))
		}
	}

	// max_concurrent_runs
	if doc.Spec.MaxConcurrentRuns != nil {
		v := *doc.Spec.MaxConcurrentRuns
		if v < 1 || v > 50 {
			errs = append(errs, fmt.Sprintf("spec.max_concurrent_runs must be 1-50, got %d", v))
		}
	}

	// steps validation
	stepNames := make(map[string]bool)
	for i, step := range doc.Spec.Steps {
		prefix := fmt.Sprintf("spec.steps[%d]", i)

		if step.Name == "" {
			errs = append(errs, fmt.Sprintf("%s.name is required", prefix))
		} else if stepNames[step.Name] {
			errs = append(errs, fmt.Sprintf("%s.name %q is duplicated", prefix, step.Name))
		} else {
			stepNames[step.Name] = true
		}

		if step.Type == "" {
			errs = append(errs, fmt.Sprintf("%s.type is required", prefix))
		} else if _, ok := stepTypeRegistry[step.Type]; !ok {
			errs = append(errs, fmt.Sprintf("%s.type %q is not a valid step type", prefix, step.Type))
		}

		// Validate depends_on references
		for _, dep := range step.DependsOn {
			if dep == step.Name {
				errs = append(errs, fmt.Sprintf("%s.depends_on: step cannot depend on itself", prefix))
			}
		}

		// on_failure
		if step.OnFailure != "" {
			validOnFailure := map[string]bool{"abort": true, "continue": true, "ignore": true}
			if !validOnFailure[step.OnFailure] {
				errs = append(errs, fmt.Sprintf("%s.on_failure must be abort|continue|ignore, got %q", prefix, step.OnFailure))
			}
		}

		// retry
		if step.Retry != nil {
			if step.Retry.MaxAttempts < 0 || step.Retry.MaxAttempts > 10 {
				errs = append(errs, fmt.Sprintf("%s.retry.max_attempts must be 0-10, got %d", prefix, step.Retry.MaxAttempts))
			}
		}
	}

	// Validate depends_on references exist (second pass)
	for i, step := range doc.Spec.Steps {
		for _, dep := range step.DependsOn {
			if !stepNames[dep] {
				errs = append(errs, fmt.Sprintf("spec.steps[%d].depends_on: step %q does not exist", i, dep))
			}
		}
	}

	// DAG cycle detection
	if cycleErr := detectStepCycle(doc.Spec.Steps); cycleErr != "" {
		errs = append(errs, cycleErr)
	}

	// triggers validation (reuse existing engine)
	if len(doc.Spec.Triggers) > 0 {
		triggerRules, err := parseTriggerMaps(doc.Spec.Triggers)
		if err != nil {
			errs = append(errs, fmt.Sprintf("spec.triggers: %v", err))
		} else {
			triggerErrs := ValidateTriggerRules(triggerRules)
			errs = append(errs, triggerErrs...)
		}
	}

	return errs
}

// ValidatePipelineYAMLFromJSON 從 JSON bytes 解析並驗證 Pipeline YAML。
func ValidatePipelineYAMLFromJSON(data []byte) ([]string, error) {
	var doc PipelineYAMLDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse pipeline document: %w", err)
	}
	return ValidatePipelineYAML(&doc), nil
}

// --- DAG cycle detection ---

func detectStepCycle(steps []PipelineYAMLStep) string {
	// Build adjacency list
	adj := make(map[string][]string)
	for _, s := range steps {
		adj[s.Name] = s.DependsOn
	}

	visited := make(map[string]int) // 0=unvisited, 1=in-stack, 2=done
	var cyclePath []string

	var dfs func(node string) bool
	dfs = func(node string) bool {
		visited[node] = 1
		cyclePath = append(cyclePath, node)

		for _, dep := range adj[node] {
			if visited[dep] == 1 {
				cyclePath = append(cyclePath, dep)
				return true
			}
			if visited[dep] == 0 {
				if dfs(dep) {
					return true
				}
			}
		}

		cyclePath = cyclePath[:len(cyclePath)-1]
		visited[node] = 2
		return false
	}

	for _, s := range steps {
		if visited[s.Name] == 0 {
			if dfs(s.Name) {
				return fmt.Sprintf("spec.steps: dependency cycle detected: %s", strings.Join(cyclePath, " → "))
			}
		}
	}
	return ""
}

func parseTriggerMaps(triggers []map[string]interface{}) ([]TriggerRule, error) {
	data, err := json.Marshal(triggers)
	if err != nil {
		return nil, err
	}
	var rules []TriggerRule
	if err := json.Unmarshal(data, &rules); err != nil {
		return nil, err
	}
	return rules, nil
}
