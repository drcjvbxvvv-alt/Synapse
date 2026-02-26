package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/clay-wangzhi/KubePolaris/pkg/logger"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

// K8sListerProvider 提供 Informer Lister 的接口（避免循环依赖 k8s 包）
type K8sListerProvider interface {
	PodsLister(clusterID uint) corev1listers.PodLister
	NodesLister(clusterID uint) corev1listers.NodeLister
	ServicesLister(clusterID uint) corev1listers.ServiceLister
	DeploymentsLister(clusterID uint) appsv1listers.DeploymentLister
	GetK8sClientByID(clusterID uint) *K8sClient
}

// ToolExecutor K8s 工具执行器
type ToolExecutor struct {
	listerProvider   K8sListerProvider
	clusterService   *ClusterService
}

// NewToolExecutor 创建工具执行器
func NewToolExecutor(listerProvider K8sListerProvider, clusterSvc *ClusterService) *ToolExecutor {
	return &ToolExecutor{
		listerProvider: listerProvider,
		clusterService: clusterSvc,
	}
}

// GetToolDefinitions 返回所有可用工具定义（用于 OpenAI Function Calling）
func GetToolDefinitions() []ToolDefinition {
	return []ToolDefinition{
		{
			Type: "function",
			Function: FunctionDefinition{
				Name:        "list_pods",
				Description: "列出指定命名空间（或所有命名空间）的 Pod，包含状态、重启次数等信息",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"namespace": map[string]interface{}{
							"type":        "string",
							"description": "命名空间名称，为空则列出所有命名空间的 Pod",
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDefinition{
				Name:        "get_pod_detail",
				Description: "获取指定 Pod 的详细信息，包含容器状态、事件等",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"namespace": map[string]interface{}{
							"type":        "string",
							"description": "Pod 所在命名空间",
						},
						"name": map[string]interface{}{
							"type":        "string",
							"description": "Pod 名称",
						},
					},
					"required": []string{"namespace", "name"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDefinition{
				Name:        "get_pod_logs",
				Description: "获取指定 Pod 的最近日志（最多100行）",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"namespace": map[string]interface{}{
							"type":        "string",
							"description": "Pod 所在命名空间",
						},
						"name": map[string]interface{}{
							"type":        "string",
							"description": "Pod 名称",
						},
						"container": map[string]interface{}{
							"type":        "string",
							"description": "容器名称（可选，多容器 Pod 时指定）",
						},
					},
					"required": []string{"namespace", "name"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDefinition{
				Name:        "list_deployments",
				Description: "列出指定命名空间（或所有命名空间）的 Deployment，包含副本数等信息",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"namespace": map[string]interface{}{
							"type":        "string",
							"description": "命名空间名称，为空则列出所有命名空间",
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDefinition{
				Name:        "get_deployment_detail",
				Description: "获取指定 Deployment 的详细信息",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"namespace": map[string]interface{}{
							"type":        "string",
							"description": "Deployment 所在命名空间",
						},
						"name": map[string]interface{}{
							"type":        "string",
							"description": "Deployment 名称",
						},
					},
					"required": []string{"namespace", "name"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDefinition{
				Name:        "list_nodes",
				Description: "列出集群所有节点，包含状态、角色、资源信息",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDefinition{
				Name:        "get_node_detail",
				Description: "获取指定节点的详细信息，包含资源分配、条件等",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"name": map[string]interface{}{
							"type":        "string",
							"description": "节点名称",
						},
					},
					"required": []string{"name"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDefinition{
				Name:        "list_events",
				Description: "列出指定命名空间的 K8s 事件（最近 50 条），可过滤特定资源",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"namespace": map[string]interface{}{
							"type":        "string",
							"description": "命名空间名称，为空则列出所有命名空间",
						},
						"resource_name": map[string]interface{}{
							"type":        "string",
							"description": "按涉及的资源名称过滤（可选）",
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDefinition{
				Name:        "list_services",
				Description: "列出指定命名空间（或所有命名空间）的 Service",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"namespace": map[string]interface{}{
							"type":        "string",
							"description": "命名空间名称，为空则列出所有命名空间",
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDefinition{
				Name:        "list_ingresses",
				Description: "列出指定命名空间（或所有命名空间）的 Ingress",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"namespace": map[string]interface{}{
							"type":        "string",
							"description": "命名空间名称，为空则列出所有命名空间",
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDefinition{
				Name:        "scale_deployment",
				Description: "扩缩容 Deployment（写操作，需要用户确认）",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"namespace": map[string]interface{}{
							"type":        "string",
							"description": "Deployment 所在命名空间",
						},
						"name": map[string]interface{}{
							"type":        "string",
							"description": "Deployment 名称",
						},
						"replicas": map[string]interface{}{
							"type":        "integer",
							"description": "目标副本数",
						},
						"confirmed": map[string]interface{}{
							"type":        "boolean",
							"description": "用户是否已确认执行（首次调用应为 false，要求用户确认）",
						},
					},
					"required": []string{"namespace", "name", "replicas"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDefinition{
				Name:        "restart_deployment",
				Description: "重启 Deployment（写操作，通过 rollout restart 实现，需要用户确认）",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"namespace": map[string]interface{}{
							"type":        "string",
							"description": "Deployment 所在命名空间",
						},
						"name": map[string]interface{}{
							"type":        "string",
							"description": "Deployment 名称",
						},
						"confirmed": map[string]interface{}{
							"type":        "boolean",
							"description": "用户是否已确认执行",
						},
					},
					"required": []string{"namespace", "name"},
				},
			},
		},
	}
}

// ExecuteTool 执行指定工具
func (e *ToolExecutor) ExecuteTool(ctx context.Context, clusterID uint, toolName string, argsJSON string) (string, error) {
	var args map[string]interface{}
	if argsJSON != "" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("解析工具参数失败: %w", err)
		}
	}
	if args == nil {
		args = make(map[string]interface{})
	}

	getStr := func(key string) string {
		if v, ok := args[key]; ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
		return ""
	}

	switch toolName {
	case "list_pods":
		return e.listPods(ctx, clusterID, getStr("namespace"))
	case "get_pod_detail":
		return e.getPodDetail(ctx, clusterID, getStr("namespace"), getStr("name"))
	case "get_pod_logs":
		return e.getPodLogs(ctx, clusterID, getStr("namespace"), getStr("name"), getStr("container"))
	case "list_deployments":
		return e.listDeployments(clusterID, getStr("namespace"))
	case "get_deployment_detail":
		return e.getDeploymentDetail(ctx, clusterID, getStr("namespace"), getStr("name"))
	case "list_nodes":
		return e.listNodes(clusterID)
	case "get_node_detail":
		return e.getNodeDetail(ctx, clusterID, getStr("name"))
	case "list_events":
		return e.listEvents(ctx, clusterID, getStr("namespace"), getStr("resource_name"))
	case "list_services":
		return e.listServices(clusterID, getStr("namespace"))
	case "list_ingresses":
		return e.listIngresses(ctx, clusterID, getStr("namespace"))
	case "scale_deployment":
		replicas := 0
		if v, ok := args["replicas"]; ok {
			if f, ok := v.(float64); ok {
				replicas = int(f)
			}
		}
		confirmed := false
		if v, ok := args["confirmed"]; ok {
			if b, ok := v.(bool); ok {
				confirmed = b
			}
		}
		return e.scaleDeployment(ctx, clusterID, getStr("namespace"), getStr("name"), replicas, confirmed)
	case "restart_deployment":
		confirmed := false
		if v, ok := args["confirmed"]; ok {
			if b, ok := v.(bool); ok {
				confirmed = b
			}
		}
		return e.restartDeployment(ctx, clusterID, getStr("namespace"), getStr("name"), confirmed)
	default:
		return "", fmt.Errorf("未知工具: %s", toolName)
	}
}

func (e *ToolExecutor) getClientset(clusterID uint) (*kubernetes.Clientset, error) {
	kc := e.listerProvider.GetK8sClientByID(clusterID)
	if kc == nil {
		return nil, fmt.Errorf("集群 %d 未初始化", clusterID)
	}
	return kc.GetClientset(), nil
}

func (e *ToolExecutor) listPods(_ context.Context, clusterID uint, namespace string) (string, error) {
	lister := e.listerProvider.PodsLister(clusterID)
	if lister == nil {
		return "", fmt.Errorf("集群 Informer 未就绪")
	}

	var podList []*corev1.Pod
	var err error
	if namespace != "" {
		podList, err = lister.Pods(namespace).List(labels.Everything())
	} else {
		podList, err = lister.List(labels.Everything())
	}
	if err != nil {
		return "", fmt.Errorf("列出 Pod 失败: %w", err)
	}

	type podSummary struct {
		Name       string `json:"name"`
		Namespace  string `json:"namespace"`
		Status     string `json:"status"`
		Node       string `json:"node"`
		Restarts   int32  `json:"restarts"`
		Ready      string `json:"ready"`
		Age        string `json:"age"`
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
		return "", fmt.Errorf("获取 Pod 失败: %w", err)
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
		return "", fmt.Errorf("获取日志失败: %w", err)
	}
	defer stream.Close()

	logBytes, err := io.ReadAll(io.LimitReader(stream, 64*1024))
	if err != nil {
		return "", fmt.Errorf("读取日志失败: %w", err)
	}

	return string(logBytes), nil
}

func (e *ToolExecutor) listDeployments(clusterID uint, namespace string) (string, error) {
	lister := e.listerProvider.DeploymentsLister(clusterID)
	if lister == nil {
		return "", fmt.Errorf("集群 Informer 未就绪")
	}

	type deploySummary struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
		Replicas  int32  `json:"replicas"`
		Ready     int32  `json:"ready"`
		Available int32  `json:"available"`
		Age       string `json:"age"`
		Images    string `json:"images"`
	}

	var result []deploySummary

	if namespace != "" {
		deploys, err := lister.Deployments(namespace).List(labels.Everything())
		if err != nil {
			return "", fmt.Errorf("列出 Deployment 失败: %w", err)
		}
		for _, d := range deploys {
			images := getContainerImages(d.Spec.Template.Spec.Containers)
			result = append(result, deploySummary{
				Name:      d.Name,
				Namespace: d.Namespace,
				Replicas:  *d.Spec.Replicas,
				Ready:     d.Status.ReadyReplicas,
				Available: d.Status.AvailableReplicas,
				Age:       formatAge(d.CreationTimestamp.Time),
				Images:    images,
			})
		}
	} else {
		deploys, err := lister.List(labels.Everything())
		if err != nil {
			return "", fmt.Errorf("列出 Deployment 失败: %w", err)
		}
		for _, d := range deploys {
			images := getContainerImages(d.Spec.Template.Spec.Containers)
			result = append(result, deploySummary{
				Name:      d.Name,
				Namespace: d.Namespace,
				Replicas:  *d.Spec.Replicas,
				Ready:     d.Status.ReadyReplicas,
				Available: d.Status.AvailableReplicas,
				Age:       formatAge(d.CreationTimestamp.Time),
				Images:    images,
			})
		}
	}

	data, _ := json.Marshal(map[string]interface{}{
		"total":       len(result),
		"deployments": result,
	})
	return string(data), nil
}

func (e *ToolExecutor) getDeploymentDetail(ctx context.Context, clusterID uint, namespace, name string) (string, error) {
	clientset, err := e.getClientset(clusterID)
	if err != nil {
		return "", err
	}

	deploy, err := clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("获取 Deployment 失败: %w", err)
	}

	conditions := make([]map[string]string, 0)
	for _, cond := range deploy.Status.Conditions {
		conditions = append(conditions, map[string]string{
			"type":    string(cond.Type),
			"status":  string(cond.Status),
			"reason":  cond.Reason,
			"message": cond.Message,
		})
	}

	detail := map[string]interface{}{
		"name":               deploy.Name,
		"namespace":          deploy.Namespace,
		"replicas":           deploy.Spec.Replicas,
		"readyReplicas":      deploy.Status.ReadyReplicas,
		"availableReplicas":  deploy.Status.AvailableReplicas,
		"updatedReplicas":    deploy.Status.UpdatedReplicas,
		"strategy":           deploy.Spec.Strategy.Type,
		"labels":             deploy.Labels,
		"images":             getContainerImages(deploy.Spec.Template.Spec.Containers),
		"conditions":         conditions,
		"age":                formatAge(deploy.CreationTimestamp.Time),
	}

	data, _ := json.Marshal(detail)
	return string(data), nil
}

func (e *ToolExecutor) listNodes(clusterID uint) (string, error) {
	lister := e.listerProvider.NodesLister(clusterID)
	if lister == nil {
		return "", fmt.Errorf("集群 Informer 未就绪")
	}

	nodes, err := lister.List(labels.Everything())
	if err != nil {
		return "", fmt.Errorf("列出 Node 失败: %w", err)
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
		return "", fmt.Errorf("获取 Node 失败: %w", err)
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
		"name":             node.Name,
		"labels":           node.Labels,
		"conditions":       conditions,
		"taints":           taints,
		"unschedulable":    node.Spec.Unschedulable,
		"kubeletVersion":   node.Status.NodeInfo.KubeletVersion,
		"osImage":          node.Status.NodeInfo.OSImage,
		"containerRuntime": node.Status.NodeInfo.ContainerRuntimeVersion,
		"cpu":              node.Status.Capacity.Cpu().String(),
		"memory":           node.Status.Capacity.Memory().String(),
		"pods":             node.Status.Capacity.Pods().String(),
		"allocatableCPU":   node.Status.Allocatable.Cpu().String(),
		"allocatableMemory": node.Status.Allocatable.Memory().String(),
		"age":              formatAge(node.CreationTimestamp.Time),
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
		return "", fmt.Errorf("列出事件失败: %w", err)
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

	// 只返回最近 50 条
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
		return "", fmt.Errorf("集群 Informer 未就绪")
	}

	type svcSummary struct {
		Name       string `json:"name"`
		Namespace  string `json:"namespace"`
		Type       string `json:"type"`
		ClusterIP  string `json:"clusterIP"`
		Ports      string `json:"ports"`
		Age        string `json:"age"`
	}

	var result []svcSummary

	if namespace != "" {
		svcs, err := lister.Services(namespace).List(labels.Everything())
		if err != nil {
			return "", fmt.Errorf("列出 Service 失败: %w", err)
		}
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
	} else {
		svcs, err := lister.List(labels.Everything())
		if err != nil {
			return "", fmt.Errorf("列出 Service 失败: %w", err)
		}
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

	var ns string
	if namespace != "" {
		ns = namespace
	}

	ingressList, err := clientset.NetworkingV1().Ingresses(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("列出 Ingress 失败: %w", err)
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

func (e *ToolExecutor) scaleDeployment(ctx context.Context, clusterID uint, namespace, name string, replicas int, confirmed bool) (string, error) {
	if !confirmed {
		return fmt.Sprintf(`{"action":"scale_deployment","namespace":"%s","name":"%s","target_replicas":%d,"status":"awaiting_confirmation","message":"请确认是否将 %s/%s 的副本数调整为 %d？"}`,
			namespace, name, replicas, namespace, name, replicas), nil
	}

	clientset, err := e.getClientset(clusterID)
	if err != nil {
		return "", err
	}

	scale, err := clientset.AppsV1().Deployments(namespace).GetScale(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("获取 Deployment scale 失败: %w", err)
	}

	scale.Spec.Replicas = int32(replicas)
	_, err = clientset.AppsV1().Deployments(namespace).UpdateScale(ctx, name, scale, metav1.UpdateOptions{})
	if err != nil {
		return "", fmt.Errorf("扩缩容失败: %w", err)
	}

	logger.Info("AI 工具执行扩缩容", "deployment", fmt.Sprintf("%s/%s", namespace, name), "replicas", replicas)
	return fmt.Sprintf(`{"status":"success","message":"已将 %s/%s 的副本数调整为 %d"}`, namespace, name, replicas), nil
}

func (e *ToolExecutor) restartDeployment(ctx context.Context, clusterID uint, namespace, name string, confirmed bool) (string, error) {
	if !confirmed {
		return fmt.Sprintf(`{"action":"restart_deployment","namespace":"%s","name":"%s","status":"awaiting_confirmation","message":"请确认是否重启 %s/%s？"}`,
			namespace, name, namespace, name), nil
	}

	clientset, err := e.getClientset(clusterID)
	if err != nil {
		return "", err
	}

	deploy, err := clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("获取 Deployment 失败: %w", err)
	}

	if deploy.Spec.Template.Annotations == nil {
		deploy.Spec.Template.Annotations = make(map[string]string)
	}
	deploy.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().Format(time.RFC3339)

	_, err = clientset.AppsV1().Deployments(namespace).Update(ctx, deploy, metav1.UpdateOptions{})
	if err != nil {
		return "", fmt.Errorf("重启 Deployment 失败: %w", err)
	}

	logger.Info("AI 工具执行重启", "deployment", fmt.Sprintf("%s/%s", namespace, name))
	return fmt.Sprintf(`{"status":"success","message":"已触发 %s/%s 滚动重启"}`, namespace, name), nil
}

func formatAge(t time.Time) string {
	d := time.Since(t)
	if d.Hours() >= 24*365 {
		return fmt.Sprintf("%.0fy", d.Hours()/(24*365))
	}
	if d.Hours() >= 24 {
		return fmt.Sprintf("%.0fd", d.Hours()/24)
	}
	if d.Hours() >= 1 {
		return fmt.Sprintf("%.0fh", d.Hours())
	}
	if d.Minutes() >= 1 {
		return fmt.Sprintf("%.0fm", d.Minutes())
	}
	return fmt.Sprintf("%.0fs", d.Seconds())
}

func getContainerImages(containers []corev1.Container) string {
	imgs := make([]string, 0, len(containers))
	for _, c := range containers {
		imgs = append(imgs, c.Image)
	}
	return strings.Join(imgs, ", ")
}

func formatServicePorts(ports []corev1.ServicePort) string {
	parts := make([]string, 0, len(ports))
	for _, p := range ports {
		s := fmt.Sprintf("%d/%s", p.Port, p.Protocol)
		if p.NodePort > 0 {
			s = fmt.Sprintf("%d:%d/%s", p.Port, p.NodePort, p.Protocol)
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, ", ")
}
