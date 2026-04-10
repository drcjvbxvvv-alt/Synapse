package handlers

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

// convertPodsToInfo 轉換Pod列表為PodInfo
func (h *PodHandler) convertPodsToInfo(pods []corev1.Pod) []PodInfo {
	var podInfos []PodInfo
	for _, pod := range pods {
		podInfos = append(podInfos, h.convertPodToInfo(pod))
	}
	return podInfos
}

// convertPodToInfo 轉換Pod為PodInfo
func (h *PodHandler) convertPodToInfo(pod corev1.Pod) PodInfo {
	// 計算重啟次數
	var restartCount int32
	for _, containerStatus := range pod.Status.ContainerStatuses {
		restartCount += containerStatus.RestartCount
	}

	// 轉換容器資訊
	containers := make([]ContainerInfo, 0, len(pod.Spec.Containers))
	for i, container := range pod.Spec.Containers {
		containerInfo := ContainerInfo{
			Name:  container.Name,
			Image: container.Image,
			Resources: ContainerResource{
				Requests: make(map[string]string),
				Limits:   make(map[string]string),
			},
		}

		// 資源資訊
		if container.Resources.Requests != nil {
			for k, v := range container.Resources.Requests {
				containerInfo.Resources.Requests[string(k)] = v.String()
			}
		}
		if container.Resources.Limits != nil {
			for k, v := range container.Resources.Limits {
				containerInfo.Resources.Limits[string(k)] = v.String()
			}
		}

		// 連接埠資訊
		for _, port := range container.Ports {
			containerInfo.Ports = append(containerInfo.Ports, ContainerPort{
				Name:          port.Name,
				ContainerPort: port.ContainerPort,
				Protocol:      string(port.Protocol),
			})
		}

		// 狀態資訊
		if i < len(pod.Status.ContainerStatuses) {
			status := pod.Status.ContainerStatuses[i]
			containerInfo.Ready = status.Ready
			containerInfo.RestartCount = status.RestartCount

			if status.State.Running != nil {
				containerInfo.State = ContainerState{
					State:     "Running",
					StartedAt: &status.State.Running.StartedAt.Time,
				}
			} else if status.State.Waiting != nil {
				containerInfo.State = ContainerState{
					State:   "Waiting",
					Reason:  status.State.Waiting.Reason,
					Message: status.State.Waiting.Message,
				}
			} else if status.State.Terminated != nil {
				containerInfo.State = ContainerState{
					State:     "Terminated",
					Reason:    status.State.Terminated.Reason,
					Message:   status.State.Terminated.Message,
					StartedAt: &status.State.Terminated.StartedAt.Time,
				}
			}
		}

		containers = append(containers, containerInfo)
	}

	// 轉換Init容器資訊
	initContainers := make([]ContainerInfo, 0, len(pod.Spec.InitContainers))
	for i, container := range pod.Spec.InitContainers {
		containerInfo := ContainerInfo{
			Name:  container.Name,
			Image: container.Image,
			Resources: ContainerResource{
				Requests: make(map[string]string),
				Limits:   make(map[string]string),
			},
		}

		// 狀態資訊
		if i < len(pod.Status.InitContainerStatuses) {
			status := pod.Status.InitContainerStatuses[i]
			containerInfo.Ready = status.Ready
			containerInfo.RestartCount = status.RestartCount
		}

		initContainers = append(initContainers, containerInfo)
	}

	// 轉換條件資訊
	conditions := make([]PodCondition, 0, len(pod.Status.Conditions))
	for _, condition := range pod.Status.Conditions {
		conditions = append(conditions, PodCondition{
			Type:               string(condition.Type),
			Status:             string(condition.Status),
			LastProbeTime:      condition.LastProbeTime.Time,
			LastTransitionTime: condition.LastTransitionTime.Time,
			Reason:             condition.Reason,
			Message:            condition.Message,
		})
	}

	return PodInfo{
		Name:              pod.Name,
		Namespace:         pod.Namespace,
		Status:            h.getPodStatus(pod),
		Phase:             string(pod.Status.Phase),
		NodeName:          pod.Spec.NodeName,
		PodIP:             pod.Status.PodIP,
		HostIP:            pod.Status.HostIP,
		RestartCount:      restartCount,
		CreatedAt:         pod.CreationTimestamp.Time,
		Labels:            pod.Labels,
		Annotations:       pod.Annotations,
		OwnerReferences:   pod.OwnerReferences,
		Containers:        containers,
		InitContainers:    initContainers,
		Conditions:        conditions,
		QOSClass:          string(pod.Status.QOSClass),
		ServiceAccount:    pod.Spec.ServiceAccountName,
		Priority:          pod.Spec.Priority,
		PriorityClassName: pod.Spec.PriorityClassName,
	}
}

// getPodStatus 獲取Pod狀態
func (h *PodHandler) getPodStatus(pod corev1.Pod) string {
	if pod.DeletionTimestamp != nil {
		return "Terminating"
	}

	switch pod.Status.Phase {
	case corev1.PodPending:
		// 檢查是否有容器在等待
		for _, containerStatus := range pod.Status.ContainerStatuses {
			if containerStatus.State.Waiting != nil {
				if containerStatus.State.Waiting.Reason == "ImagePullBackOff" ||
					containerStatus.State.Waiting.Reason == "ErrImagePull" {
					return containerStatus.State.Waiting.Reason
				}
			}
		}
		return "Pending"
	case corev1.PodRunning:
		// 檢查是否所有容器都就緒
		ready := 0
		total := len(pod.Status.ContainerStatuses)
		for _, containerStatus := range pod.Status.ContainerStatuses {
			if containerStatus.Ready {
				ready++
			} else if containerStatus.State.Waiting != nil {
				return containerStatus.State.Waiting.Reason
			} else if containerStatus.State.Terminated != nil {
				return containerStatus.State.Terminated.Reason
			}
		}
		if ready == total {
			return "Running"
		}
		return fmt.Sprintf("NotReady (%d/%d)", ready, total)
	case corev1.PodSucceeded:
		return "Completed"
	case corev1.PodFailed:
		return "Failed"
	default:
		return string(pod.Status.Phase)
	}
}
