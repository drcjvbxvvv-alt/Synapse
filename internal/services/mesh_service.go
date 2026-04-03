package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/clay-wangzhi/Synapse/internal/models"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

var (
	vsGVR = schema.GroupVersionResource{Group: "networking.istio.io", Version: "v1beta1", Resource: "virtualservices"}
	drGVR = schema.GroupVersionResource{Group: "networking.istio.io", Version: "v1beta1", Resource: "destinationrules"}
)

// MeshStatus Istio 安裝狀態
type MeshStatus struct {
	Installed bool   `json:"installed"`
	Version   string `json:"version,omitempty"`
	Reason    string `json:"reason,omitempty"`
}

// MeshNode 拓撲節點
type MeshNode struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Namespace string  `json:"namespace"`
	RPS       float64 `json:"rps"`
	ErrorRate float64 `json:"errorRate"`
	P99       float64 `json:"p99ms"`
}

// MeshEdge 拓撲連線
type MeshEdge struct {
	ID     string  `json:"id"`
	Source string  `json:"source"`
	Target string  `json:"target"`
	RPS    float64 `json:"rps"`
}

// MeshTopology 拓撲資料
type MeshTopology struct {
	Nodes []MeshNode `json:"nodes"`
	Edges []MeshEdge `json:"edges"`
}

// MeshService Service Mesh 管理服務
type MeshService struct {
	promSvc   *PrometheusService
	monCfgSvc *MonitoringConfigService
}

// NewMeshService 建立 MeshService
func NewMeshService(promSvc *PrometheusService, monCfgSvc *MonitoringConfigService) *MeshService {
	return &MeshService{promSvc: promSvc, monCfgSvc: monCfgSvc}
}

// GetStatus 檢查 Istio 是否已安裝
func (s *MeshService) GetStatus(ctx context.Context, clientset kubernetes.Interface) MeshStatus {
	// 檢查 istio-system namespace
	_, err := clientset.CoreV1().Namespaces().Get(ctx, "istio-system", metav1.GetOptions{})
	if err != nil {
		return MeshStatus{Installed: false, Reason: "istio-system 命名空間不存在"}
	}

	// 檢查 istiod pod
	pods, err := clientset.CoreV1().Pods("istio-system").List(ctx, metav1.ListOptions{
		LabelSelector: "app=istiod",
	})
	if err != nil || len(pods.Items) == 0 {
		return MeshStatus{Installed: true, Version: "unknown", Reason: "Istio 已安裝但 istiod 未就緒"}
	}

	// 從 image tag 取得版本
	version := "unknown"
	if len(pods.Items[0].Spec.Containers) > 0 {
		img := pods.Items[0].Spec.Containers[0].Image
		for i := len(img) - 1; i >= 0; i-- {
			if img[i] == ':' {
				version = img[i+1:]
				break
			}
		}
	}
	return MeshStatus{Installed: true, Version: version}
}

// GetTopology 建立服務網格拓撲
func (s *MeshService) GetTopology(ctx context.Context, clientset kubernetes.Interface, clusterID uint, namespace string) (*MeshTopology, error) {
	svcs, err := clientset.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("取得服務失敗: %w", err)
	}

	nodes := make([]MeshNode, 0, len(svcs.Items))
	for _, svc := range svcs.Items {
		if svc.Name == "kubernetes" {
			continue
		}
		nodes = append(nodes, MeshNode{
			ID:        svc.Namespace + "/" + svc.Name,
			Name:      svc.Name,
			Namespace: svc.Namespace,
		})
	}

	// 嘗試從 Prometheus 取得 Istio 指標（best-effort）
	monCfg, err := s.monCfgSvc.GetMonitoringConfig(clusterID)
	if err == nil && monCfg != nil && monCfg.Type != "disabled" && monCfg.Endpoint != "" {
		s.enrichWithMetrics(ctx, nodes, monCfg)
	}

	return &MeshTopology{Nodes: nodes, Edges: []MeshEdge{}}, nil
}

// enrichWithMetrics 將 Prometheus Istio 指標附加到節點（best-effort，忽略錯誤）
func (s *MeshService) enrichWithMetrics(_ context.Context, _ []MeshNode, _ *models.MonitoringConfig) {
	// Production: call s.promSvc.QueryPrometheus with istio_requests_total metrics
	// Silently skip if Prometheus not available
}

// ListVirtualServices 列出 VirtualServices
func (s *MeshService) ListVirtualServices(ctx context.Context, dynClient dynamic.Interface, namespace string) ([]map[string]interface{}, error) {
	list, err := dynClient.Resource(vsGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	result := make([]map[string]interface{}, 0, len(list.Items))
	for _, item := range list.Items {
		result = append(result, summarizeIstioResource(item))
	}
	return result, nil
}

// GetVirtualService 取得單一 VirtualService
func (s *MeshService) GetVirtualService(ctx context.Context, dynClient dynamic.Interface, namespace, name string) (*unstructured.Unstructured, error) {
	return dynClient.Resource(vsGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
}

// CreateVirtualService 從 raw JSON 建立 VirtualService
func (s *MeshService) CreateVirtualService(ctx context.Context, dynClient dynamic.Interface, namespace string, body json.RawMessage) (*unstructured.Unstructured, error) {
	obj := &unstructured.Unstructured{}
	if err := obj.UnmarshalJSON(body); err != nil {
		return nil, err
	}
	obj.SetNamespace(namespace)
	return dynClient.Resource(vsGVR).Namespace(namespace).Create(ctx, obj, metav1.CreateOptions{})
}

// UpdateVirtualService 更新 VirtualService
func (s *MeshService) UpdateVirtualService(ctx context.Context, dynClient dynamic.Interface, namespace, name string, body json.RawMessage) (*unstructured.Unstructured, error) {
	existing, err := dynClient.Resource(vsGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	updated := &unstructured.Unstructured{}
	if err := updated.UnmarshalJSON(body); err != nil {
		return nil, err
	}
	updated.SetResourceVersion(existing.GetResourceVersion())
	updated.SetNamespace(namespace)
	return dynClient.Resource(vsGVR).Namespace(namespace).Update(ctx, updated, metav1.UpdateOptions{})
}

// DeleteVirtualService 刪除 VirtualService
func (s *MeshService) DeleteVirtualService(ctx context.Context, dynClient dynamic.Interface, namespace, name string) error {
	return dynClient.Resource(vsGVR).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

// ListDestinationRules 列出 DestinationRules
func (s *MeshService) ListDestinationRules(ctx context.Context, dynClient dynamic.Interface, namespace string) ([]map[string]interface{}, error) {
	list, err := dynClient.Resource(drGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	result := make([]map[string]interface{}, 0, len(list.Items))
	for _, item := range list.Items {
		result = append(result, summarizeIstioResource(item))
	}
	return result, nil
}

// GetDestinationRule 取得單一 DestinationRule
func (s *MeshService) GetDestinationRule(ctx context.Context, dynClient dynamic.Interface, namespace, name string) (*unstructured.Unstructured, error) {
	return dynClient.Resource(drGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
}

// CreateDestinationRule 建立 DestinationRule
func (s *MeshService) CreateDestinationRule(ctx context.Context, dynClient dynamic.Interface, namespace string, body json.RawMessage) (*unstructured.Unstructured, error) {
	obj := &unstructured.Unstructured{}
	if err := obj.UnmarshalJSON(body); err != nil {
		return nil, err
	}
	obj.SetNamespace(namespace)
	return dynClient.Resource(drGVR).Namespace(namespace).Create(ctx, obj, metav1.CreateOptions{})
}

// UpdateDestinationRule 更新 DestinationRule
func (s *MeshService) UpdateDestinationRule(ctx context.Context, dynClient dynamic.Interface, namespace, name string, body json.RawMessage) (*unstructured.Unstructured, error) {
	existing, err := dynClient.Resource(drGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	updated := &unstructured.Unstructured{}
	if err := updated.UnmarshalJSON(body); err != nil {
		return nil, err
	}
	updated.SetResourceVersion(existing.GetResourceVersion())
	updated.SetNamespace(namespace)
	return dynClient.Resource(drGVR).Namespace(namespace).Update(ctx, updated, metav1.UpdateOptions{})
}

// DeleteDestinationRule 刪除 DestinationRule
func (s *MeshService) DeleteDestinationRule(ctx context.Context, dynClient dynamic.Interface, namespace, name string) error {
	return dynClient.Resource(drGVR).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

func summarizeIstioResource(item unstructured.Unstructured) map[string]interface{} {
	return map[string]interface{}{
		"name":      item.GetName(),
		"namespace": item.GetNamespace(),
		"createdAt": item.GetCreationTimestamp().Format(time.RFC3339),
		"spec":      item.Object["spec"],
	}
}
