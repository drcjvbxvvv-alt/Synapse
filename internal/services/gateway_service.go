package services

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/yaml"
)


// --- Service ---

// GatewayService 管理 Gateway API 資源（動態 CRD 操作）
type GatewayService struct {
	dynClient dynamic.Interface
	k8sClient *K8sClient
}

// NewGatewayService 建立 GatewayService
func NewGatewayService(k8sClient *K8sClient) (*GatewayService, error) {
	dynClient, err := dynamic.NewForConfig(k8sClient.GetRestConfig())
	if err != nil {
		return nil, fmt.Errorf("建立 dynamic client 失敗: %v", err)
	}
	return &GatewayService{
		dynClient: dynClient,
		k8sClient: k8sClient,
	}, nil
}

// IsGatewayAPIAvailable 偵測叢集是否已安裝 Gateway API CRD（v1 → v1beta1 fallback）
func (s *GatewayService) IsGatewayAPIAvailable(ctx context.Context) bool {
	for _, version := range []string{"v1", "v1beta1", "v1alpha2"} {
		_, err := s.k8sClient.GetClientset().Discovery().ServerResourcesForGroupVersion(
			"gateway.networking.k8s.io/" + version,
		)
		if err == nil {
			return true
		}
	}
	return false
}

// --- GatewayClass ---

func (s *GatewayService) ListGatewayClasses(ctx context.Context) ([]GatewayClassItem, error) {
	list, err := s.dynClient.Resource(GatewayClassGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	result := make([]GatewayClassItem, 0, len(list.Items))
	for _, item := range list.Items {
		result = append(result, toGatewayClassItem(item))
	}
	return result, nil
}

func (s *GatewayService) GetGatewayClass(ctx context.Context, name string) (*GatewayClassItem, error) {
	obj, err := s.dynClient.Resource(GatewayClassGVR).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	item := toGatewayClassItem(*obj)
	return &item, nil
}

// --- Gateway ---

func (s *GatewayService) ListGateways(ctx context.Context, namespace string) ([]GatewayItem, error) {
	var list *unstructured.UnstructuredList
	var err error
	if namespace == "" {
		list, err = s.dynClient.Resource(GatewayGVR).Namespace(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	} else {
		list, err = s.dynClient.Resource(GatewayGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	}
	if err != nil {
		return nil, err
	}
	result := make([]GatewayItem, 0, len(list.Items))
	for _, item := range list.Items {
		result = append(result, toGatewayItem(item))
	}
	return result, nil
}

func (s *GatewayService) GetGateway(ctx context.Context, namespace, name string) (*GatewayItem, error) {
	obj, err := s.dynClient.Resource(GatewayGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	item := toGatewayItem(*obj)
	return &item, nil
}

func (s *GatewayService) GetGatewayYAML(ctx context.Context, namespace, name string) (string, error) {
	obj, err := s.dynClient.Resource(GatewayGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	gwCleanMeta(obj)
	data, err := yaml.Marshal(obj.Object)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// --- HTTPRoute ---

func (s *GatewayService) ListHTTPRoutes(ctx context.Context, namespace string) ([]HTTPRouteItem, error) {
	var list *unstructured.UnstructuredList
	var err error
	if namespace == "" {
		list, err = s.dynClient.Resource(HTTPRouteGVR).Namespace(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	} else {
		list, err = s.dynClient.Resource(HTTPRouteGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	}
	if err != nil {
		return nil, err
	}
	result := make([]HTTPRouteItem, 0, len(list.Items))
	for _, item := range list.Items {
		result = append(result, toHTTPRouteItem(item))
	}
	return result, nil
}

func (s *GatewayService) GetHTTPRoute(ctx context.Context, namespace, name string) (*HTTPRouteItem, error) {
	obj, err := s.dynClient.Resource(HTTPRouteGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	item := toHTTPRouteItem(*obj)
	return &item, nil
}

func (s *GatewayService) GetHTTPRouteYAML(ctx context.Context, namespace, name string) (string, error) {
	obj, err := s.dynClient.Resource(HTTPRouteGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	gwCleanMeta(obj)
	data, err := yaml.Marshal(obj.Object)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// --- 轉換函式 ---

func (s *GatewayService) CreateGateway(ctx context.Context, namespace, yamlStr string) (*GatewayItem, error) {
	obj, err := gwParseYAML(yamlStr)
	if err != nil {
		return nil, err
	}
	if namespace != "" {
		obj.SetNamespace(namespace)
	}
	result, err := s.dynClient.Resource(GatewayGVR).Namespace(obj.GetNamespace()).Create(ctx, obj, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	item := toGatewayItem(*result)
	return &item, nil
}

// UpdateGateway 從 YAML 更新 Gateway
func (s *GatewayService) UpdateGateway(ctx context.Context, namespace, name, yamlStr string) (*GatewayItem, error) {
	// 先取得現有物件以保留 resourceVersion
	existing, err := s.dynClient.Resource(GatewayGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	obj, err := gwParseYAML(yamlStr)
	if err != nil {
		return nil, err
	}
	obj.SetNamespace(namespace)
	obj.SetName(name)
	obj.SetResourceVersion(existing.GetResourceVersion())
	result, err := s.dynClient.Resource(GatewayGVR).Namespace(namespace).Update(ctx, obj, metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}
	item := toGatewayItem(*result)
	return &item, nil
}

// DeleteGateway 刪除 Gateway
func (s *GatewayService) DeleteGateway(ctx context.Context, namespace, name string) error {
	return s.dynClient.Resource(GatewayGVR).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

// CreateHTTPRoute 從 YAML 建立 HTTPRoute
func (s *GatewayService) CreateHTTPRoute(ctx context.Context, namespace, yamlStr string, dryRun bool) (*HTTPRouteItem, error) {
	obj, err := gwParseYAML(yamlStr)
	if err != nil {
		return nil, err
	}
	if namespace != "" {
		obj.SetNamespace(namespace)
	}
	opts := metav1.CreateOptions{}
	if dryRun {
		opts.DryRun = []string{metav1.DryRunAll}
	}
	result, err := s.dynClient.Resource(HTTPRouteGVR).Namespace(obj.GetNamespace()).Create(ctx, obj, opts)
	if err != nil {
		return nil, err
	}
	item := toHTTPRouteItem(*result)
	return &item, nil
}

// UpdateHTTPRoute 從 YAML 更新 HTTPRoute
func (s *GatewayService) UpdateHTTPRoute(ctx context.Context, namespace, name, yamlStr string, dryRun bool) (*HTTPRouteItem, error) {
	existing, err := s.dynClient.Resource(HTTPRouteGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	obj, err := gwParseYAML(yamlStr)
	if err != nil {
		return nil, err
	}
	obj.SetNamespace(namespace)
	obj.SetName(name)
	obj.SetResourceVersion(existing.GetResourceVersion())
	opts := metav1.UpdateOptions{}
	if dryRun {
		opts.DryRun = []string{metav1.DryRunAll}
	}
	result, err := s.dynClient.Resource(HTTPRouteGVR).Namespace(namespace).Update(ctx, obj, opts)
	if err != nil {
		return nil, err
	}
	item := toHTTPRouteItem(*result)
	return &item, nil
}

// DeleteHTTPRoute 刪除 HTTPRoute
func (s *GatewayService) DeleteHTTPRoute(ctx context.Context, namespace, name string) error {
	return s.dynClient.Resource(HTTPRouteGVR).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

// --- GRPCRoute ---

func (s *GatewayService) ListGRPCRoutes(ctx context.Context, namespace string) ([]GRPCRouteItem, error) {
	var list *unstructured.UnstructuredList
	var err error
	if namespace == "" {
		list, err = s.dynClient.Resource(GRPCRouteGVR).Namespace(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	} else {
		list, err = s.dynClient.Resource(GRPCRouteGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	}
	if err != nil {
		return nil, err
	}
	result := make([]GRPCRouteItem, 0, len(list.Items))
	for _, item := range list.Items {
		result = append(result, toGRPCRouteItem(item))
	}
	return result, nil
}

func (s *GatewayService) GetGRPCRoute(ctx context.Context, namespace, name string) (*GRPCRouteItem, error) {
	obj, err := s.dynClient.Resource(GRPCRouteGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	item := toGRPCRouteItem(*obj)
	return &item, nil
}

func (s *GatewayService) GetGRPCRouteYAML(ctx context.Context, namespace, name string) (string, error) {
	obj, err := s.dynClient.Resource(GRPCRouteGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	gwCleanMeta(obj)
	data, err := yaml.Marshal(obj.Object)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (s *GatewayService) CreateGRPCRoute(ctx context.Context, namespace, yamlStr string, dryRun bool) (*GRPCRouteItem, error) {
	obj, err := gwParseYAML(yamlStr)
	if err != nil {
		return nil, err
	}
	if namespace != "" {
		obj.SetNamespace(namespace)
	}
	opts := metav1.CreateOptions{}
	if dryRun {
		opts.DryRun = []string{metav1.DryRunAll}
	}
	result, err := s.dynClient.Resource(GRPCRouteGVR).Namespace(obj.GetNamespace()).Create(ctx, obj, opts)
	if err != nil {
		return nil, err
	}
	item := toGRPCRouteItem(*result)
	return &item, nil
}

func (s *GatewayService) UpdateGRPCRoute(ctx context.Context, namespace, name, yamlStr string, dryRun bool) (*GRPCRouteItem, error) {
	existing, err := s.dynClient.Resource(GRPCRouteGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	obj, err := gwParseYAML(yamlStr)
	if err != nil {
		return nil, err
	}
	obj.SetNamespace(namespace)
	obj.SetName(name)
	obj.SetResourceVersion(existing.GetResourceVersion())
	opts := metav1.UpdateOptions{}
	if dryRun {
		opts.DryRun = []string{metav1.DryRunAll}
	}
	result, err := s.dynClient.Resource(GRPCRouteGVR).Namespace(namespace).Update(ctx, obj, opts)
	if err != nil {
		return nil, err
	}
	item := toGRPCRouteItem(*result)
	return &item, nil
}

func (s *GatewayService) DeleteGRPCRoute(ctx context.Context, namespace, name string) error {
	return s.dynClient.Resource(GRPCRouteGVR).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

// --- ReferenceGrant ---

func (s *GatewayService) ListReferenceGrants(ctx context.Context, namespace string) ([]ReferenceGrantItem, error) {
	var list *unstructured.UnstructuredList
	var err error
	if namespace == "" {
		list, err = s.dynClient.Resource(ReferenceGrantGVR).Namespace(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	} else {
		list, err = s.dynClient.Resource(ReferenceGrantGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	}
	if err != nil {
		return nil, err
	}
	result := make([]ReferenceGrantItem, 0, len(list.Items))
	for _, item := range list.Items {
		result = append(result, toReferenceGrantItem(item))
	}
	return result, nil
}

func (s *GatewayService) GetReferenceGrantYAML(ctx context.Context, namespace, name string) (string, error) {
	obj, err := s.dynClient.Resource(ReferenceGrantGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	gwCleanMeta(obj)
	data, err := yaml.Marshal(obj.Object)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (s *GatewayService) CreateReferenceGrant(ctx context.Context, namespace, yamlStr string, dryRun bool) (*ReferenceGrantItem, error) {
	obj, err := gwParseYAML(yamlStr)
	if err != nil {
		return nil, err
	}
	if namespace != "" {
		obj.SetNamespace(namespace)
	}
	opts := metav1.CreateOptions{}
	if dryRun {
		opts.DryRun = []string{metav1.DryRunAll}
	}
	result, err := s.dynClient.Resource(ReferenceGrantGVR).Namespace(obj.GetNamespace()).Create(ctx, obj, opts)
	if err != nil {
		return nil, err
	}
	item := toReferenceGrantItem(*result)
	return &item, nil
}

func (s *GatewayService) DeleteReferenceGrant(ctx context.Context, namespace, name string) error {
	return s.dynClient.Resource(ReferenceGrantGVR).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

// --- Topology ---

func (s *GatewayService) GetTopology(ctx context.Context) (*TopologyData, error) {
	nodes := make([]TopologyNode, 0)
	edges := make([]TopologyEdge, 0)
	nodeSet := map[string]bool{}

	addNode := func(n TopologyNode) {
		if !nodeSet[n.ID] {
			nodeSet[n.ID] = true
			nodes = append(nodes, n)
		}
	}

	// GatewayClasses
	gcList, _ := s.ListGatewayClasses(ctx)
	for _, gc := range gcList {
		addNode(TopologyNode{ID: "gc:" + gc.Name, Kind: "GatewayClass", Name: gc.Name, Status: gc.AcceptedStatus})
	}

	// Gateways
	gwList, _ := s.ListGateways(ctx, "")
	for _, gw := range gwList {
		gwID := "gw:" + gw.Namespace + "/" + gw.Name
		status := "Unknown"
		for _, c := range gw.Conditions {
			if c.Type == "Programmed" {
				if c.Status == "True" {
					status = "Ready"
				} else {
					status = c.Reason
				}
				break
			}
		}
		// Build listener summary (e.g. "HTTP:80" or "HTTP:80 HTTPS:443")
		listenerSummary := ""
		for _, l := range gw.Listeners {
			part := fmt.Sprintf("%s:%d", l.Protocol, l.Port)
			if listenerSummary == "" {
				listenerSummary = part
			} else {
				listenerSummary += " " + part
			}
		}
		addNode(TopologyNode{ID: gwID, Kind: "Gateway", Name: gw.Name, Namespace: gw.Namespace, Status: status, SubKind: listenerSummary})
		gcID := "gc:" + gw.GatewayClass
		if nodeSet[gcID] {
			edges = append(edges, TopologyEdge{Source: gcID, Target: gwID})
		}
	}

	// HTTPRoutes
	hrList, _ := s.ListHTTPRoutes(ctx, "")
	for _, hr := range hrList {
		hrID := "hr:" + hr.Namespace + "/" + hr.Name
		addNode(TopologyNode{ID: hrID, Kind: "HTTPRoute", Name: hr.Name, Namespace: hr.Namespace, Hostnames: hr.Hostnames})
		for _, pr := range hr.ParentRefs {
			parentID := "gw:" + pr.GatewayNamespace + "/" + pr.GatewayName
			if nodeSet[parentID] {
				edges = append(edges, TopologyEdge{Source: parentID, Target: hrID})
			}
		}
		for _, rule := range hr.Rules {
			for _, b := range rule.Backends {
				svcNS := b.Namespace
				if svcNS == "" {
					svcNS = hr.Namespace
				}
				svcID := "svc:" + svcNS + "/" + b.Name
				addNode(TopologyNode{ID: svcID, Kind: "Service", Name: b.Name, Namespace: svcNS})
				edges = append(edges, TopologyEdge{Source: hrID, Target: svcID})
			}
		}
	}

	// GRPCRoutes
	grList, _ := s.ListGRPCRoutes(ctx, "")
	for _, gr := range grList {
		grID := "gr:" + gr.Namespace + "/" + gr.Name
		addNode(TopologyNode{ID: grID, Kind: "GRPCRoute", Name: gr.Name, Namespace: gr.Namespace, Hostnames: gr.Hostnames})
		for _, pr := range gr.ParentRefs {
			parentID := "gw:" + pr.GatewayNamespace + "/" + pr.GatewayName
			if nodeSet[parentID] {
				edges = append(edges, TopologyEdge{Source: parentID, Target: grID})
			}
		}
		for _, rule := range gr.Rules {
			for _, b := range rule.Backends {
				svcNS := b.Namespace
				if svcNS == "" {
					svcNS = gr.Namespace
				}
				svcID := "svc:" + svcNS + "/" + b.Name
				addNode(TopologyNode{ID: svcID, Kind: "Service", Name: b.Name, Namespace: svcNS})
				edges = append(edges, TopologyEdge{Source: grID, Target: svcID})
			}
		}
	}

	return &TopologyData{Nodes: nodes, Edges: edges}, nil
}

