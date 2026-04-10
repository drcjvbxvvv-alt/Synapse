package handlers

import (
	"fmt"
	"time"

	rollouts "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
)

// RolloutInfo Rollout資訊
type RolloutInfo struct {
	ID                string            `json:"id"`
	Name              string            `json:"name"`
	Namespace         string            `json:"namespace"`
	Type              string            `json:"type"`
	Status            string            `json:"status"`
	Replicas          int32             `json:"replicas"`
	ReadyReplicas     int32             `json:"readyReplicas"`
	AvailableReplicas int32             `json:"availableReplicas"`
	UpdatedReplicas   int32             `json:"updatedReplicas"`
	Labels            map[string]string `json:"labels"`
	Annotations       map[string]string `json:"annotations"`
	CreatedAt         time.Time         `json:"createdAt"`
	Images            []string          `json:"images"`
	Selector          map[string]string `json:"selector"`
	Strategy          string            `json:"strategy"`
}

// convertToRolloutInfo 轉換 Rollout 到 RolloutInfo
func convertToRolloutInfo(r *rollouts.Rollout) RolloutInfo {
	status := "Healthy"
	if r.Status.Replicas == 0 {
		status = "Stopped"
	} else if r.Status.AvailableReplicas < r.Status.Replicas {
		status = "Degraded"
	}

	// 提取映像列表
	var images []string
	for _, container := range r.Spec.Template.Spec.Containers {
		images = append(images, container.Image)
	}

	// 策略
	strategy := "Canary"
	if r.Spec.Strategy.BlueGreen != nil {
		strategy = "BlueGreen"
	}

	replicas := int32(0)
	if r.Spec.Replicas != nil {
		replicas = *r.Spec.Replicas
	}

	return RolloutInfo{
		ID:                fmt.Sprintf("%s/%s", r.Namespace, r.Name),
		Name:              r.Name,
		Namespace:         r.Namespace,
		Type:              "Rollout",
		Status:            status,
		Replicas:          replicas,
		ReadyReplicas:     r.Status.ReadyReplicas,
		AvailableReplicas: r.Status.AvailableReplicas,
		UpdatedReplicas:   r.Status.UpdatedReplicas,
		Labels:            r.Labels,
		Annotations:       r.Annotations,
		CreatedAt:         r.CreationTimestamp.Time,
		Images:            images,
		Selector:          r.Spec.Selector.MatchLabels,
		Strategy:          strategy,
	}
}
