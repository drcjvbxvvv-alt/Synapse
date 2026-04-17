package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func (e *ToolExecutor) listPods(_ context.Context, clusterID uint, namespace string) (string, error) {
	lister := e.listerProvider.PodsLister(clusterID)
	if lister == nil {
		return "", fmt.Errorf("叢集 Informer 未就緒")
	}

	var podList []*corev1.Pod
	var err error
	if namespace != "" {
		podList, err = lister.Pods(namespace).List(labels.Everything())
	} else {
		podList, err = lister.List(labels.Everything())
	}
	if err != nil {
		return "", fmt.Errorf("列出 Pod 失敗: %w", err)
	}

	type podSummary struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
		Status    string `json:"status"`
		Node      string `json:"node"`
		Restarts  int32  `json:"restarts"`
		Ready     string `json:"ready"`
		Age       string `json:"age"`
	}

	result := make([]podSummary, 0, len(podList))
	for _, pod := range podList {
		var restarts int32
		readyCount := 0
		totalCount := len(pod.Status.ContainerStatuses)
		for _, cs := range pod.Status.ContainerStatuses {
			restarts += cs.RestartCount
			if cs.Ready {
				readyCount++
			}
		}

		result = append(result, podSummary{
			Name:      pod.Name,
			Namespace: pod.Namespace,
			Status:    string(pod.Status.Phase),
			Node:      pod.Spec.NodeName,
			Restarts:  restarts,
			Ready:     fmt.Sprintf("%d/%d", readyCount, totalCount),
			Age:       formatAge(pod.CreationTimestamp.Time),
		})
	}

	data, _ := json.Marshal(map[string]interface{}{
		"total": len(result),
		"pods":  result,
	})
	return string(data), nil
}

func (e *ToolExecutor) getPodDetail(ctx context.Context, clusterID uint, namespace, name string) (string, error) {
	clientset, err := e.getClientset(clusterID)
	if err != nil {
		return "", err
	}

	pod, err := clientset.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("獲取 Pod 失敗: %w", err)
	}

	containers := make([]map[string]interface{}, 0)
	for _, cs := range pod.Status.ContainerStatuses {
		c := map[string]interface{}{
			"name":         cs.Name,
			"image":        cs.Image,
			"ready":        cs.Ready,
			"restartCount": cs.RestartCount,
		}
		if cs.State.Waiting != nil {
			c["state"] = "Waiting"
			c["reason"] = cs.State.Waiting.Reason
			c["message"] = cs.State.Waiting.Message
		} else if cs.State.Running != nil {
			c["state"] = "Running"
			c["startedAt"] = cs.State.Running.StartedAt.Format(time.RFC3339)
		} else if cs.State.Terminated != nil {
			c["state"] = "Terminated"
			c["reason"] = cs.State.Terminated.Reason
			c["exitCode"] = cs.State.Terminated.ExitCode
		}
		containers = append(containers, c)
	}

	detail := map[string]interface{}{
		"name":       pod.Name,
		"namespace":  pod.Namespace,
		"status":     string(pod.Status.Phase),
		"node":       pod.Spec.NodeName,
		"ip":         pod.Status.PodIP,
		"hostIP":     pod.Status.HostIP,
		"startTime":  pod.Status.StartTime,
		"labels":     pod.Labels,
		"containers": containers,
		"age":        formatAge(pod.CreationTimestamp.Time),
	}

	conditions := make([]map[string]string, 0)
	for _, cond := range pod.Status.Conditions {
		conditions = append(conditions, map[string]string{
			"type":    string(cond.Type),
			"status":  string(cond.Status),
			"reason":  cond.Reason,
			"message": cond.Message,
		})
	}
	detail["conditions"] = conditions

	data, _ := json.Marshal(detail)
	return string(data), nil
}

func (e *ToolExecutor) getPodLogs(ctx context.Context, clusterID uint, namespace, name, container string) (string, error) {
	clientset, err := e.getClientset(clusterID)
	if err != nil {
		return "", err
	}

	tailLines := int64(100)
	opts := &corev1.PodLogOptions{
		TailLines: &tailLines,
	}
	if container != "" {
		opts.Container = container
	}

	req := clientset.CoreV1().Pods(namespace).GetLogs(name, opts)
	stream, err := req.Stream(ctx)
	if err != nil {
		return "", fmt.Errorf("獲取日誌失敗: %w", err)
	}
	defer stream.Close()

	logBytes, err := io.ReadAll(io.LimitReader(stream, 64*1024))
	if err != nil {
		return "", fmt.Errorf("讀取日誌失敗: %w", err)
	}

	return string(logBytes), nil
}
