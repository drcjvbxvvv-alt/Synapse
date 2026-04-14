package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/shaia/Synapse/pkg/logger"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

// ---------------------------------------------------------------------------
// RolloutService — Argo Rollouts 狀態機（CICD_ARCHITECTURE §M13c, P1-5）
//
// 設計原則：
//   - 使用 dynamic client 操作 Rollout CRD（不引入 Argo Rollouts SDK）
//   - Observer Pattern：先偵測 CRD 是否安裝，未安裝則優雅降級
//   - 提供 Rollout 查詢、操作（promote/abort/retry）
//   - 純服務層，不依賴 DB（Rollout 狀態來自 K8s）
// ---------------------------------------------------------------------------

var rolloutGVR = schema.GroupVersionResource{
	Group:    "argoproj.io",
	Version:  "v1alpha1",
	Resource: "rollouts",
}

// RolloutService 管理 Argo Rollouts 操作。
type RolloutService struct{}

// NewRolloutService 建立 RolloutService。
func NewRolloutService() *RolloutService {
	return &RolloutService{}
}

// ---------------------------------------------------------------------------
// CRD Discovery（Observer Pattern）
// ---------------------------------------------------------------------------

// IsArgoRolloutsInstalled 偵測叢集是否安裝 Argo Rollouts CRD。
func (s *RolloutService) IsArgoRolloutsInstalled(ctx context.Context, clientset kubernetes.Interface) bool {
	for _, version := range []string{"v1alpha1"} {
		_, err := clientset.Discovery().ServerResourcesForGroupVersion("argoproj.io/" + version)
		if err == nil {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Rollout 查詢
// ---------------------------------------------------------------------------

// RolloutInfo 代表 Rollout 的摘要資訊。
type RolloutInfo struct {
	Name              string            `json:"name"`
	Namespace         string            `json:"namespace"`
	Strategy          string            `json:"strategy"`           // canary / blueGreen
	Status            string            `json:"status"`             // Healthy / Progressing / Degraded / Paused
	CurrentImage      string            `json:"current_image"`
	DesiredReplicas   int64             `json:"desired_replicas"`
	ReadyReplicas     int64             `json:"ready_replicas"`
	UpdatedReplicas   int64             `json:"updated_replicas"`
	CanaryWeight      int64             `json:"canary_weight"`
	StableRevision    string            `json:"stable_revision"`
	CurrentRevision   string            `json:"current_revision"`
	CreatedAt         time.Time         `json:"created_at"`
	Conditions        []RolloutCondition `json:"conditions,omitempty"`
}

// RolloutCondition 代表 Rollout 的狀態條件。
type RolloutCondition struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

// ListRollouts 列出指定命名空間的所有 Rollout。
func (s *RolloutService) ListRollouts(ctx context.Context, dynClient dynamic.Interface, namespace string) ([]RolloutInfo, error) {
	list, err := dynClient.Resource(rolloutGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list rollouts in %s: %w", namespace, err)
	}

	rollouts := make([]RolloutInfo, 0, len(list.Items))
	for i := range list.Items {
		info := parseRolloutInfo(&list.Items[i])
		rollouts = append(rollouts, info)
	}
	return rollouts, nil
}

// GetRollout 取得指定 Rollout 的詳細資訊。
func (s *RolloutService) GetRollout(ctx context.Context, dynClient dynamic.Interface, namespace, name string) (*RolloutInfo, error) {
	obj, err := dynClient.Resource(rolloutGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get rollout %s/%s: %w", namespace, name, err)
	}

	info := parseRolloutInfo(obj)
	return &info, nil
}

// ---------------------------------------------------------------------------
// Rollout 操作
// ---------------------------------------------------------------------------

// PromoteRollout 晉升 Rollout（推進 canary 到下一步或完全晉升）。
func (s *RolloutService) PromoteRollout(ctx context.Context, dynClient dynamic.Interface, namespace, name string, full bool) error {
	// Argo Rollouts 使用 annotation 觸發 promote
	// 設定 rollout.argoproj.io/promote = "full" 或 "true"
	promoteValue := "true"
	if full {
		promoteValue = "full"
	}

	patch := fmt.Sprintf(`{"metadata":{"annotations":{"rollout.argoproj.io/promote":"%s"}}}`, promoteValue)
	_, err := dynClient.Resource(rolloutGVR).Namespace(namespace).Patch(
		ctx, name, types.MergePatchType, []byte(patch), metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("promote rollout %s/%s: %w", namespace, name, err)
	}

	logger.Info("rollout promoted",
		"namespace", namespace,
		"name", name,
		"full", full,
	)
	return nil
}

// AbortRollout 中止 Rollout（回滾到 stable 版本）。
func (s *RolloutService) AbortRollout(ctx context.Context, dynClient dynamic.Interface, namespace, name string) error {
	// 設定 rollout.argoproj.io/abort = "true"
	patch := `{"metadata":{"annotations":{"rollout.argoproj.io/abort":"true"}}}`
	_, err := dynClient.Resource(rolloutGVR).Namespace(namespace).Patch(
		ctx, name, types.MergePatchType, []byte(patch), metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("abort rollout %s/%s: %w", namespace, name, err)
	}

	logger.Info("rollout aborted",
		"namespace", namespace,
		"name", name,
	)
	return nil
}

// RetryRollout 重試失敗的 Rollout。
func (s *RolloutService) RetryRollout(ctx context.Context, dynClient dynamic.Interface, namespace, name string) error {
	// 移除 abort annotation 並設定 restart
	patch := `{"metadata":{"annotations":{"rollout.argoproj.io/abort":null,"rollout.argoproj.io/restart":"true"}}}`
	_, err := dynClient.Resource(rolloutGVR).Namespace(namespace).Patch(
		ctx, name, types.MergePatchType, []byte(patch), metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("retry rollout %s/%s: %w", namespace, name, err)
	}

	logger.Info("rollout retried",
		"namespace", namespace,
		"name", name,
	)
	return nil
}

// SetRolloutImage 更新 Rollout 的容器映像。
func (s *RolloutService) SetRolloutImage(ctx context.Context, dynClient dynamic.Interface, namespace, name, image string) error {
	// 使用 JSON patch 更新第一個 container 的 image
	patch := fmt.Sprintf(`{"spec":{"template":{"spec":{"containers":[{"name":"*","image":"%s"}]}}}}`, image)
	// 使用 strategic merge patch
	_, err := dynClient.Resource(rolloutGVR).Namespace(namespace).Patch(
		ctx, name, types.MergePatchType, []byte(patch), metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("set rollout image %s/%s to %s: %w", namespace, name, image, err)
	}

	logger.Info("rollout image updated",
		"namespace", namespace,
		"name", name,
		"image", image,
	)
	return nil
}

// ---------------------------------------------------------------------------
// Parsing helpers
// ---------------------------------------------------------------------------

func parseRolloutInfo(obj *unstructured.Unstructured) RolloutInfo {
	info := RolloutInfo{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
		CreatedAt: obj.GetCreationTimestamp().Time,
	}

	// Strategy
	info.Strategy = detectRolloutStrategy(obj)

	// Status phase
	info.Status = nestedString(obj, "status", "phase")
	if info.Status == "" {
		info.Status = "Unknown"
	}

	// Replicas
	info.DesiredReplicas, _, _ = unstructured.NestedInt64(obj.Object, "status", "replicas")
	info.ReadyReplicas, _, _ = unstructured.NestedInt64(obj.Object, "status", "readyReplicas")
	info.UpdatedReplicas, _, _ = unstructured.NestedInt64(obj.Object, "status", "updatedReplicas")

	// Canary weight
	info.CanaryWeight, _, _ = unstructured.NestedInt64(obj.Object, "status", "canary", "weight")

	// Revisions
	info.StableRevision = nestedString(obj, "status", "stableRS")
	info.CurrentRevision = nestedString(obj, "status", "currentPodHash")

	// Current image — first container
	containers, found, _ := unstructured.NestedSlice(obj.Object, "spec", "template", "spec", "containers")
	if found && len(containers) > 0 {
		if c, ok := containers[0].(map[string]interface{}); ok {
			if img, ok := c["image"].(string); ok {
				info.CurrentImage = img
			}
		}
	}

	// Conditions
	conditions, found, _ := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if found {
		for _, cond := range conditions {
			if cm, ok := cond.(map[string]interface{}); ok {
				info.Conditions = append(info.Conditions, RolloutCondition{
					Type:    stringFromMap(cm, "type"),
					Status:  stringFromMap(cm, "status"),
					Reason:  stringFromMap(cm, "reason"),
					Message: stringFromMap(cm, "message"),
				})
			}
		}
	}

	return info
}

func detectRolloutStrategy(obj *unstructured.Unstructured) string {
	if _, found, _ := unstructured.NestedMap(obj.Object, "spec", "strategy", "canary"); found {
		return "canary"
	}
	if _, found, _ := unstructured.NestedMap(obj.Object, "spec", "strategy", "blueGreen"); found {
		return "blueGreen"
	}
	return "unknown"
}

func nestedString(obj *unstructured.Unstructured, fields ...string) string {
	val, found, _ := unstructured.NestedString(obj.Object, fields...)
	if !found {
		return ""
	}
	return val
}

func stringFromMap(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// ---------------------------------------------------------------------------
// Validation helpers
// ---------------------------------------------------------------------------

// ValidateRolloutStatus 驗證期望的 Rollout 狀態值。
func ValidateRolloutStatus(status string) error {
	valid := map[string]bool{
		"healthy":     true,
		"progressing": true,
		"degraded":    true,
		"paused":      true,
	}
	if !valid[strings.ToLower(status)] {
		return fmt.Errorf("invalid rollout status %q, must be healthy|progressing|degraded|paused", status)
	}
	return nil
}

// IsRolloutStatusMatch 檢查 Rollout 當前狀態是否符合期望。
func IsRolloutStatusMatch(current, expected string) bool {
	return strings.EqualFold(current, expected)
}
