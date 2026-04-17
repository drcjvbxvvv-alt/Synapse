package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func (e *ToolExecutor) listNodes(clusterID uint) (string, error) {
	lister := e.listerProvider.NodesLister(clusterID)
	if lister == nil {
		return "", fmt.Errorf("叢集 Informer 未就緒")
	}

	nodes, err := lister.List(labels.Everything())
	if err != nil {
		return "", fmt.Errorf("列出 Node 失敗: %w", err)
	}

	type nodeSummary struct {
		Name    string   `json:"name"`
		Status  string   `json:"status"`
		Roles   []string `json:"roles"`
		Version string   `json:"version"`
		CPU     string   `json:"cpu"`
		Memory  string   `json:"memory"`
		Age     string   `json:"age"`
	}

	result := make([]nodeSummary, 0, len(nodes))
	for _, node := range nodes {
		status := "NotReady"
		for _, cond := range node.Status.Conditions {
			if cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionTrue {
				status = "Ready"
				break
			}
		}

		roles := make([]string, 0)
		for label := range node.Labels {
			if strings.HasPrefix(label, "node-role.kubernetes.io/") {
				role := strings.TrimPrefix(label, "node-role.kubernetes.io/")
				if role != "" {
					roles = append(roles, role)
				}
			}
		}
		if len(roles) == 0 {
			roles = append(roles, "<none>")
		}

		result = append(result, nodeSummary{
			Name:    node.Name,
			Status:  status,
			Roles:   roles,
			Version: node.Status.NodeInfo.KubeletVersion,
			CPU:     node.Status.Capacity.Cpu().String(),
			Memory:  node.Status.Capacity.Memory().String(),
			Age:     formatAge(node.CreationTimestamp.Time),
		})
	}

	data, _ := json.Marshal(map[string]interface{}{
		"total": len(result),
		"nodes": result,
	})
	return string(data), nil
}

func (e *ToolExecutor) getNodeDetail(ctx context.Context, clusterID uint, name string) (string, error) {
	clientset, err := e.getClientset(clusterID)
	if err != nil {
		return "", err
	}

	node, err := clientset.CoreV1().Nodes().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("獲取 Node 失敗: %w", err)
	}

	conditions := make([]map[string]string, 0)
	for _, cond := range node.Status.Conditions {
		conditions = append(conditions, map[string]string{
			"type":    string(cond.Type),
			"status":  string(cond.Status),
			"reason":  cond.Reason,
			"message": cond.Message,
		})
	}

	taints := make([]map[string]string, 0)
	for _, taint := range node.Spec.Taints {
		taints = append(taints, map[string]string{
			"key":    taint.Key,
			"value":  taint.Value,
			"effect": string(taint.Effect),
		})
	}

	detail := map[string]interface{}{
		"name":              node.Name,
		"labels":            node.Labels,
		"conditions":        conditions,
		"taints":            taints,
		"unschedulable":     node.Spec.Unschedulable,
		"kubeletVersion":    node.Status.NodeInfo.KubeletVersion,
		"osImage":           node.Status.NodeInfo.OSImage,
		"containerRuntime":  node.Status.NodeInfo.ContainerRuntimeVersion,
		"cpu":               node.Status.Capacity.Cpu().String(),
		"memory":            node.Status.Capacity.Memory().String(),
		"pods":              node.Status.Capacity.Pods().String(),
		"allocatableCPU":    node.Status.Allocatable.Cpu().String(),
		"allocatableMemory": node.Status.Allocatable.Memory().String(),
		"age":               formatAge(node.CreationTimestamp.Time),
	}

	data, _ := json.Marshal(detail)
	return string(data), nil
}

func (e *ToolExecutor) listEvents(ctx context.Context, clusterID uint, namespace, resourceName string) (string, error) {
	clientset, err := e.getClientset(clusterID)
	if err != nil {
		return "", err
	}

	var eventList *corev1.EventList
	if namespace != "" {
		eventList, err = clientset.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{})
	} else {
		eventList, err = clientset.CoreV1().Events("").List(ctx, metav1.ListOptions{})
	}
	if err != nil {
		return "", fmt.Errorf("列出事件失敗: %w", err)
	}

	type eventSummary struct {
		Type      string `json:"type"`
		Reason    string `json:"reason"`
		Object    string `json:"object"`
		Message   string `json:"message"`
		Count     int32  `json:"count"`
		Namespace string `json:"namespace"`
		LastSeen  string `json:"lastSeen"`
	}

	result := make([]eventSummary, 0)
	for _, evt := range eventList.Items {
		if resourceName != "" && evt.InvolvedObject.Name != resourceName {
			continue
		}

		lastSeen := ""
		if !evt.LastTimestamp.IsZero() {
			lastSeen = formatAge(evt.LastTimestamp.Time)
		} else if evt.EventTime.Time != (time.Time{}) {
			lastSeen = formatAge(evt.EventTime.Time)
		}

		result = append(result, eventSummary{
			Type:      evt.Type,
			Reason:    evt.Reason,
			Object:    fmt.Sprintf("%s/%s", evt.InvolvedObject.Kind, evt.InvolvedObject.Name),
			Message:   evt.Message,
			Count:     evt.Count,
			Namespace: evt.Namespace,
			LastSeen:  lastSeen,
		})
	}

	// 只返回最近 50 條
	if len(result) > 50 {
		result = result[len(result)-50:]
	}

	data, _ := json.Marshal(map[string]interface{}{
		"total":  len(result),
		"events": result,
	})
	return string(data), nil
}

func (e *ToolExecutor) listServices(clusterID uint, namespace string) (string, error) {
	lister := e.listerProvider.ServicesLister(clusterID)
	if lister == nil {
		return "", fmt.Errorf("叢集 Informer 未就緒")
	}

	type svcSummary struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
		Type      string `json:"type"`
		ClusterIP string `json:"clusterIP"`
		Ports     string `json:"ports"`
		Age       string `json:"age"`
	}

	var svcs []*corev1.Service
	var err error
	if namespace != "" {
		svcs, err = lister.Services(namespace).List(labels.Everything())
	} else {
		svcs, err = lister.List(labels.Everything())
	}
	if err != nil {
		return "", fmt.Errorf("列出 Service 失敗: %w", err)
	}

	result := make([]svcSummary, 0, len(svcs))
	for _, svc := range svcs {
		result = append(result, svcSummary{
			Name:      svc.Name,
			Namespace: svc.Namespace,
			Type:      string(svc.Spec.Type),
			ClusterIP: svc.Spec.ClusterIP,
			Ports:     formatServicePorts(svc.Spec.Ports),
			Age:       formatAge(svc.CreationTimestamp.Time),
		})
	}

	data, _ := json.Marshal(map[string]interface{}{
		"total":    len(result),
		"services": result,
	})
	return string(data), nil
}

func (e *ToolExecutor) listIngresses(ctx context.Context, clusterID uint, namespace string) (string, error) {
	clientset, err := e.getClientset(clusterID)
	if err != nil {
		return "", err
	}

	ingressList, err := clientset.NetworkingV1().Ingresses(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("列出 Ingress 失敗: %w", err)
	}

	type ingressSummary struct {
		Name      string   `json:"name"`
		Namespace string   `json:"namespace"`
		Hosts     []string `json:"hosts"`
		Class     string   `json:"class"`
		Age       string   `json:"age"`
	}

	result := make([]ingressSummary, 0)
	for _, ing := range ingressList.Items {
		hosts := make([]string, 0)
		for _, rule := range ing.Spec.Rules {
			if rule.Host != "" {
				hosts = append(hosts, rule.Host)
			}
		}

		class := ""
		if ing.Spec.IngressClassName != nil {
			class = *ing.Spec.IngressClassName
		}

		result = append(result, ingressSummary{
			Name:      ing.Name,
			Namespace: ing.Namespace,
			Hosts:     hosts,
			Class:     class,
			Age:       formatAge(ing.CreationTimestamp.Time),
		})
	}

	data, _ := json.Marshal(map[string]interface{}{
		"total":     len(result),
		"ingresses": result,
	})
	return string(data), nil
}
