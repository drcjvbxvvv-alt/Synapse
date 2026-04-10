package handlers

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PodInfo Pod資訊
type PodInfo struct {
	Name              string                  `json:"name"`
	Namespace         string                  `json:"namespace"`
	Status            string                  `json:"status"`
	Phase             string                  `json:"phase"`
	NodeName          string                  `json:"nodeName"`
	PodIP             string                  `json:"podIP"`
	HostIP            string                  `json:"hostIP"`
	RestartCount      int32                   `json:"restartCount"`
	CreatedAt         time.Time               `json:"createdAt"`
	Labels            map[string]string       `json:"labels"`
	Annotations       map[string]string       `json:"annotations"`
	OwnerReferences   []metav1.OwnerReference `json:"ownerReferences"`
	Containers        []ContainerInfo         `json:"containers"`
	InitContainers    []ContainerInfo         `json:"initContainers"`
	Conditions        []PodCondition          `json:"conditions"`
	QOSClass          string                  `json:"qosClass"`
	ServiceAccount    string                  `json:"serviceAccount"`
	Priority          *int32                  `json:"priority,omitempty"`
	PriorityClassName string                  `json:"priorityClassName,omitempty"`
}

// ContainerInfo 容器資訊
type ContainerInfo struct {
	Name         string            `json:"name"`
	Image        string            `json:"image"`
	Ready        bool              `json:"ready"`
	RestartCount int32             `json:"restartCount"`
	State        ContainerState    `json:"state"`
	Resources    ContainerResource `json:"resources"`
	Ports        []ContainerPort   `json:"ports"`
}

// ContainerState 容器狀態
type ContainerState struct {
	State     string     `json:"state"`
	Reason    string     `json:"reason,omitempty"`
	Message   string     `json:"message,omitempty"`
	StartedAt *time.Time `json:"startedAt,omitempty"`
}

// ContainerResource 容器資源
type ContainerResource struct {
	Requests map[string]string `json:"requests"`
	Limits   map[string]string `json:"limits"`
}

// ContainerPort 容器連接埠
type ContainerPort struct {
	Name          string `json:"name,omitempty"`
	ContainerPort int32  `json:"containerPort"`
	Protocol      string `json:"protocol"`
}

// PodCondition Pod條件
type PodCondition struct {
	Type               string    `json:"type"`
	Status             string    `json:"status"`
	LastProbeTime      time.Time `json:"lastProbeTime,omitempty"`
	LastTransitionTime time.Time `json:"lastTransitionTime"`
	Reason             string    `json:"reason,omitempty"`
	Message            string    `json:"message,omitempty"`
}
