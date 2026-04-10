package handlers

import (
	"fmt"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
)

// DeploymentInfo Deployment資訊
type DeploymentInfo struct {
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
	CPULimit          string            `json:"cpuLimit"`
	CPURequest        string            `json:"cpuRequest"`
	MemoryLimit       string            `json:"memoryLimit"`
	MemoryRequest     string            `json:"memoryRequest"`
}

// convertToDeploymentInfo 轉換Deployment到DeploymentInfo
func (h *DeploymentHandler) convertToDeploymentInfo(d *appsv1.Deployment) DeploymentInfo {
	status := "Running"
	if d.Status.Replicas == 0 {
		status = "Stopped"
	} else if d.Status.AvailableReplicas < d.Status.Replicas {
		status = "Degraded"
	}

	// 提取映像列表和資源資訊
	var images []string
	var cpuLimits, cpuRequests []string
	var memoryLimits, memoryRequests []string

	for _, container := range d.Spec.Template.Spec.Containers {
		images = append(images, container.Image)

		// CPU 限制
		if cpu := container.Resources.Limits.Cpu(); cpu != nil && !cpu.IsZero() {
			cpuLimits = append(cpuLimits, cpu.String())
		}

		// CPU 申請
		if cpu := container.Resources.Requests.Cpu(); cpu != nil && !cpu.IsZero() {
			cpuRequests = append(cpuRequests, cpu.String())
		}

		// 記憶體 限制
		if memory := container.Resources.Limits.Memory(); memory != nil && !memory.IsZero() {
			memoryLimits = append(memoryLimits, memory.String())
		}

		// 記憶體 申請
		if memory := container.Resources.Requests.Memory(); memory != nil && !memory.IsZero() {
			memoryRequests = append(memoryRequests, memory.String())
		}
	}

	// 策略
	strategy := string(d.Spec.Strategy.Type)

	// 格式化資源值
	cpuLimit := "-"
	if len(cpuLimits) > 0 {
		cpuLimit = strings.Join(cpuLimits, " + ")
	}

	cpuRequest := "-"
	if len(cpuRequests) > 0 {
		cpuRequest = strings.Join(cpuRequests, " + ")
	}

	memoryLimit := "-"
	if len(memoryLimits) > 0 {
		memoryLimit = strings.Join(memoryLimits, " + ")
	}

	memoryRequest := "-"
	if len(memoryRequests) > 0 {
		memoryRequest = strings.Join(memoryRequests, " + ")
	}

	return DeploymentInfo{
		ID:                fmt.Sprintf("%s/%s", d.Namespace, d.Name),
		Name:              d.Name,
		Namespace:         d.Namespace,
		Type:              "Deployment",
		Status:            status,
		Replicas:          *d.Spec.Replicas,
		ReadyReplicas:     d.Status.ReadyReplicas,
		AvailableReplicas: d.Status.AvailableReplicas,
		UpdatedReplicas:   d.Status.UpdatedReplicas,
		Labels:            d.Labels,
		Annotations:       d.Annotations,
		CreatedAt:         d.CreationTimestamp.Time,
		Images:            images,
		Selector:          d.Spec.Selector.MatchLabels,
		Strategy:          strategy,
		CPULimit:          cpuLimit,
		CPURequest:        cpuRequest,
		MemoryLimit:       memoryLimit,
		MemoryRequest:     memoryRequest,
	}
}
