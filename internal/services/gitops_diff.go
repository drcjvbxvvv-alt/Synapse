package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/pkg/logger"
	"gorm.io/gorm"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ---------------------------------------------------------------------------
// GitOps Diff Engine — Drift Detection（CICD_ARCHITECTURE §12.3, P1-6）
//
// 設計原則：
//   - 比較 Git Repo（desired state）與 K8s API（actual state）
//   - 支援 raw YAML manifest 的差異比對
//   - Strategic Merge Diff 層級的差異偵測
//   - Drift 通知（整合 NotifyChannel）
//   - Diff timeout 30s，避免大 chart 卡住 worker
// ---------------------------------------------------------------------------

// GitOpsDiffEngine 執行 GitOps 差異比對。
type GitOpsDiffEngine struct {
	db        *gorm.DB
	gitopsSvc *GitOpsService
}

// NewGitOpsDiffEngine 建立 GitOpsDiffEngine。
func NewGitOpsDiffEngine(db *gorm.DB, gitopsSvc *GitOpsService) *GitOpsDiffEngine {
	return &GitOpsDiffEngine{db: db, gitopsSvc: gitopsSvc}
}

// ---------------------------------------------------------------------------
// Diff Types
// ---------------------------------------------------------------------------

// DiffResult 代表一次 Diff 的結果。
type DiffResult struct {
	AppID       uint            `json:"app_id"`
	AppName     string          `json:"app_name"`
	Status      string          `json:"status"` // synced / drifted / error
	DiffItems   []DiffItem      `json:"diff_items,omitempty"`
	DiffSummary string          `json:"diff_summary"`
	ComputedAt  time.Time       `json:"computed_at"`
	Duration    time.Duration   `json:"duration_ms"`
	Error       string          `json:"error,omitempty"`
}

// DiffItem 代表單一資源的差異。
type DiffItem struct {
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	Namespace  string `json:"namespace"`
	DiffType   string `json:"diff_type"` // added / removed / modified / unchanged
	FieldDiffs []FieldDiff `json:"field_diffs,omitempty"`
}

// FieldDiff 代表資源中單一欄位的差異。
type FieldDiff struct {
	Path     string `json:"path"`     // e.g., "spec.replicas"
	Expected string `json:"expected"` // from Git
	Actual   string `json:"actual"`   // from K8s
}

// ---------------------------------------------------------------------------
// Diff 計算
// ---------------------------------------------------------------------------

// ComputeDiff 計算指定 GitOps 應用的 desired vs actual 差異。
// desiredResources 是從 Git Repo 渲染出的資源列表。
func (e *GitOpsDiffEngine) ComputeDiff(
	ctx context.Context,
	app *models.GitOpsApp,
	desiredResources []ResourceManifest,
	dynClient dynamic.Interface,
) (*DiffResult, error) {
	start := time.Now()

	result := &DiffResult{
		AppID:      app.ID,
		AppName:    app.Name,
		ComputedAt: start,
	}

	var diffs []DiffItem

	for _, desired := range desiredResources {
		gvr, err := resolveGVR(desired.APIVersion, desired.Kind)
		if err != nil {
			result.Error = fmt.Sprintf("resolve GVR for %s/%s: %v", desired.APIVersion, desired.Kind, err)
			result.Status = models.GitOpsStatusError
			result.Duration = time.Since(start)
			return result, nil
		}

		ns := desired.Namespace
		if ns == "" {
			ns = app.Namespace
		}

		// 取得 K8s 中的實際資源
		actual, err := dynClient.Resource(gvr).Namespace(ns).Get(ctx, desired.Name, metav1.GetOptions{})
		if err != nil {
			// 資源不存在 → 需要新增
			diffs = append(diffs, DiffItem{
				Kind:      desired.Kind,
				Name:      desired.Name,
				Namespace: ns,
				DiffType:  "added",
			})
			continue
		}

		// 比對差異
		fieldDiffs := compareResource(desired, actual)
		if len(fieldDiffs) > 0 {
			diffs = append(diffs, DiffItem{
				Kind:       desired.Kind,
				Name:       desired.Name,
				Namespace:  ns,
				DiffType:   "modified",
				FieldDiffs: fieldDiffs,
			})
		} else {
			diffs = append(diffs, DiffItem{
				Kind:      desired.Kind,
				Name:      desired.Name,
				Namespace: ns,
				DiffType:  "unchanged",
			})
		}
	}

	result.DiffItems = diffs
	result.Duration = time.Since(start)

	// 判斷狀態
	hasDrift := false
	for _, d := range diffs {
		if d.DiffType == "added" || d.DiffType == "modified" {
			hasDrift = true
			break
		}
	}

	if hasDrift {
		result.Status = models.GitOpsStatusDrifted
		result.DiffSummary = buildDiffSummary(diffs)
	} else {
		result.Status = models.GitOpsStatusSynced
		result.DiffSummary = "all resources in sync"
	}

	return result, nil
}

// PersistDiffResult 將 Diff 結果寫回 GitOpsApp。
func (e *GitOpsDiffEngine) PersistDiffResult(ctx context.Context, result *DiffResult) error {
	now := time.Now()
	diffJSON, _ := json.Marshal(result.DiffItems)

	updates := map[string]interface{}{
		"status":           result.Status,
		"status_message":   result.DiffSummary,
		"last_diff_at":     now,
		"last_diff_result": string(diffJSON),
	}

	if result.Status == models.GitOpsStatusSynced {
		updates["last_synced_at"] = now
	}

	if err := e.gitopsSvc.UpdateApp(ctx, result.AppID, updates); err != nil {
		return fmt.Errorf("persist diff result for app %d: %w", result.AppID, err)
	}

	logger.Info("gitops diff persisted",
		"app_id", result.AppID,
		"status", result.Status,
		"duration_ms", result.Duration.Milliseconds(),
	)
	return nil
}

// ---------------------------------------------------------------------------
// Resource comparison
// ---------------------------------------------------------------------------

// ResourceManifest 代表從 Git Repo 渲染出的單一 K8s 資源。
type ResourceManifest struct {
	APIVersion string                 `json:"apiVersion"`
	Kind       string                 `json:"kind"`
	Name       string                 `json:"name"`
	Namespace  string                 `json:"namespace,omitempty"`
	Spec       map[string]interface{} `json:"spec,omitempty"`
	Labels     map[string]string      `json:"labels,omitempty"`
	Annotations map[string]string     `json:"annotations,omitempty"`
}

// compareResource 比較 desired（Git）和 actual（K8s）資源的關鍵欄位。
func compareResource(desired ResourceManifest, actual *unstructured.Unstructured) []FieldDiff {
	var diffs []FieldDiff

	// 比較 labels
	actualLabels := actual.GetLabels()
	for k, v := range desired.Labels {
		if av, ok := actualLabels[k]; !ok || av != v {
			diffs = append(diffs, FieldDiff{
				Path:     fmt.Sprintf("metadata.labels.%s", k),
				Expected: v,
				Actual:   actualLabels[k],
			})
		}
	}

	// 比較 annotations（排除 kubectl.kubernetes.io/last-applied-configuration 等系統 annotation）
	actualAnnotations := actual.GetAnnotations()
	for k, v := range desired.Annotations {
		if isSystemAnnotation(k) {
			continue
		}
		if av, ok := actualAnnotations[k]; !ok || av != v {
			diffs = append(diffs, FieldDiff{
				Path:     fmt.Sprintf("metadata.annotations.%s", k),
				Expected: v,
				Actual:   actualAnnotations[k],
			})
		}
	}

	// 比較 spec 中的頂層欄位
	actualSpec, _, _ := unstructured.NestedMap(actual.Object, "spec")
	for key, desiredVal := range desired.Spec {
		desiredStr := fmt.Sprintf("%v", desiredVal)
		actualVal, exists := actualSpec[key]
		actualStr := ""
		if exists {
			actualStr = fmt.Sprintf("%v", actualVal)
		}
		if desiredStr != actualStr {
			diffs = append(diffs, FieldDiff{
				Path:     fmt.Sprintf("spec.%s", key),
				Expected: desiredStr,
				Actual:   actualStr,
			})
		}
	}

	return diffs
}

// isSystemAnnotation 判斷是否為系統管理的 annotation（不參與 drift 比對）。
func isSystemAnnotation(key string) bool {
	systemPrefixes := []string{
		"kubectl.kubernetes.io/",
		"deployment.kubernetes.io/",
		"kubernetes.io/",
		"control-plane.alpha.kubernetes.io/",
		"meta.helm.sh/",
	}
	for _, prefix := range systemPrefixes {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// GVR resolution
// ---------------------------------------------------------------------------

// commonGVRMap 常用資源的 GVR 映射。
var commonGVRMap = map[string]schema.GroupVersionResource{
	"v1/ConfigMap":                {Group: "", Version: "v1", Resource: "configmaps"},
	"v1/Secret":                   {Group: "", Version: "v1", Resource: "secrets"},
	"v1/Service":                  {Group: "", Version: "v1", Resource: "services"},
	"v1/ServiceAccount":           {Group: "", Version: "v1", Resource: "serviceaccounts"},
	"apps/v1/Deployment":          {Group: "apps", Version: "v1", Resource: "deployments"},
	"apps/v1/StatefulSet":         {Group: "apps", Version: "v1", Resource: "statefulsets"},
	"apps/v1/DaemonSet":           {Group: "apps", Version: "v1", Resource: "daemonsets"},
	"batch/v1/Job":                {Group: "batch", Version: "v1", Resource: "jobs"},
	"batch/v1/CronJob":            {Group: "batch", Version: "v1", Resource: "cronjobs"},
	"networking.k8s.io/v1/Ingress": {Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"},
	"rbac.authorization.k8s.io/v1/Role":        {Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "roles"},
	"rbac.authorization.k8s.io/v1/RoleBinding":  {Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "rolebindings"},
	"autoscaling/v2/HorizontalPodAutoscaler":    {Group: "autoscaling", Version: "v2", Resource: "horizontalpodautoscalers"},
	"policy/v1/PodDisruptionBudget":             {Group: "policy", Version: "v1", Resource: "poddisruptionbudgets"},
}

// resolveGVR 從 apiVersion + Kind 解析出 GroupVersionResource。
func resolveGVR(apiVersion, kind string) (schema.GroupVersionResource, error) {
	key := apiVersion + "/" + kind
	if gvr, ok := commonGVRMap[key]; ok {
		return gvr, nil
	}

	// 嘗試從 apiVersion 拆解 group/version
	group, version := parseAPIVersion(apiVersion)
	resource := strings.ToLower(kind) + "s" // 簡易複數化

	return schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: resource,
	}, nil
}

// parseAPIVersion 拆解 apiVersion 為 group 和 version。
func parseAPIVersion(apiVersion string) (group, version string) {
	parts := strings.SplitN(apiVersion, "/", 2)
	if len(parts) == 1 {
		return "", parts[0] // core API: "v1" → group="", version="v1"
	}
	return parts[0], parts[1] // "apps/v1" → group="apps", version="v1"
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func buildDiffSummary(diffs []DiffItem) string {
	added, modified, unchanged := 0, 0, 0
	for _, d := range diffs {
		switch d.DiffType {
		case "added":
			added++
		case "modified":
			modified++
		case "unchanged":
			unchanged++
		}
	}

	parts := make([]string, 0, 3)
	if added > 0 {
		parts = append(parts, fmt.Sprintf("%d to add", added))
	}
	if modified > 0 {
		parts = append(parts, fmt.Sprintf("%d drifted", modified))
	}
	if unchanged > 0 {
		parts = append(parts, fmt.Sprintf("%d in sync", unchanged))
	}

	if len(parts) == 0 {
		return "no resources"
	}
	return strings.Join(parts, ", ")
}
