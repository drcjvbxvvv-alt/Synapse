package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

// K8sListerProvider 提供 Informer Lister 的介面（避免迴圈依賴 k8s 包）
type K8sListerProvider interface {
	PodsLister(clusterID uint) corev1listers.PodLister
	NodesLister(clusterID uint) corev1listers.NodeLister
	ServicesLister(clusterID uint) corev1listers.ServiceLister
	DeploymentsLister(clusterID uint) appsv1listers.DeploymentLister
	GetK8sClientByID(clusterID uint) *K8sClient
}

// ToolExecutor K8s 工具執行器
type ToolExecutor struct {
	listerProvider   K8sListerProvider
	clusterService   *ClusterService
}

// NewToolExecutor 建立工具執行器
func NewToolExecutor(listerProvider K8sListerProvider, clusterSvc *ClusterService) *ToolExecutor {
	return &ToolExecutor{
		listerProvider: listerProvider,
		clusterService: clusterSvc,
	}
}

// GetToolDefinitions 返回所有可用工具定義（用於 OpenAI Function Calling）
func GetToolDefinitions() []ToolDefinition {
	return []ToolDefinition{
		{
			Type: "function",
			Function: FunctionDefinition{
				Name:        "list_pods",
				Description: "列出指定命名空間（或所有命名空間）的 Pod，包含狀態、重啟次數等資訊",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"namespace": map[string]interface{}{
							"type":        "string",
							"description": "命名空間名稱，為空則列出所有命名空間的 Pod",
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDefinition{
				Name:        "get_pod_detail",
				Description: "獲取指定 Pod 的詳細資訊，包含容器狀態、事件等",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"namespace": map[string]interface{}{
							"type":        "string",
							"description": "Pod 所在命名空間",
						},
						"name": map[string]interface{}{
							"type":        "string",
							"description": "Pod 名稱",
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
				Description: "獲取指定 Pod 的最近日誌（最多100行）",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"namespace": map[string]interface{}{
							"type":        "string",
							"description": "Pod 所在命名空間",
						},
						"name": map[string]interface{}{
							"type":        "string",
							"description": "Pod 名稱",
						},
						"container": map[string]interface{}{
							"type":        "string",
							"description": "容器名稱（可選，多容器 Pod 時指定）",
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
				Description: "列出指定命名空間（或所有命名空間）的 Deployment，包含副本數等資訊",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"namespace": map[string]interface{}{
							"type":        "string",
							"description": "命名空間名稱，為空則列出所有命名空間",
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDefinition{
				Name:        "get_deployment_detail",
				Description: "獲取指定 Deployment 的詳細資訊",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"namespace": map[string]interface{}{
							"type":        "string",
							"description": "Deployment 所在命名空間",
						},
						"name": map[string]interface{}{
							"type":        "string",
							"description": "Deployment 名稱",
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
				Description: "列出叢集所有節點，包含狀態、角色、資源資訊",
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
				Description: "獲取指定節點的詳細資訊，包含資源分配、條件等",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"name": map[string]interface{}{
							"type":        "string",
							"description": "節點名稱",
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
				Description: "列出指定命名空間的 K8s 事件（最近 50 條），可過濾特定資源",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"namespace": map[string]interface{}{
							"type":        "string",
							"description": "命名空間名稱，為空則列出所有命名空間",
						},
						"resource_name": map[string]interface{}{
							"type":        "string",
							"description": "按涉及的資源名稱過濾（可選）",
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDefinition{
				Name:        "list_services",
				Description: "列出指定命名空間（或所有命名空間）的 Service",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"namespace": map[string]interface{}{
							"type":        "string",
							"description": "命名空間名稱，為空則列出所有命名空間",
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDefinition{
				Name:        "list_ingresses",
				Description: "列出指定命名空間（或所有命名空間）的 Ingress",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"namespace": map[string]interface{}{
							"type":        "string",
							"description": "命名空間名稱，為空則列出所有命名空間",
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionDefinition{
				Name:        "scale_deployment",
				Description: "擴縮容 Deployment（寫操作，需要使用者確認）",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"namespace": map[string]interface{}{
							"type":        "string",
							"description": "Deployment 所在命名空間",
						},
						"name": map[string]interface{}{
							"type":        "string",
							"description": "Deployment 名稱",
						},
						"replicas": map[string]interface{}{
							"type":        "integer",
							"description": "目標副本數",
						},
						"confirmed": map[string]interface{}{
							"type":        "boolean",
							"description": "使用者是否已確認執行（首次呼叫應為 false，要求使用者確認）",
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
				Description: "重啟 Deployment（寫操作，透過 rollout restart 實現，需要使用者確認）",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"namespace": map[string]interface{}{
							"type":        "string",
							"description": "Deployment 所在命名空間",
						},
						"name": map[string]interface{}{
							"type":        "string",
							"description": "Deployment 名稱",
						},
						"confirmed": map[string]interface{}{
							"type":        "boolean",
							"description": "使用者是否已確認執行",
						},
					},
					"required": []string{"namespace", "name"},
				},
			},
		},
	}
}

// ExecuteTool 執行指定工具
func (e *ToolExecutor) ExecuteTool(ctx context.Context, clusterID uint, toolName string, argsJSON string) (string, error) {
	var args map[string]interface{}
	if argsJSON != "" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("解析工具參數失敗: %w", err)
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
		return nil, fmt.Errorf("叢集 %d 未初始化", clusterID)
	}
	return kc.GetClientset(), nil
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
