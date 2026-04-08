package services

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/yaml"
)

// GVR 定義
var (
	GatewayClassGVR = schema.GroupVersionResource{
		Group:    "gateway.networking.k8s.io",
		Version:  "v1",
		Resource: "gatewayclasses",
	}
	GatewayGVR = schema.GroupVersionResource{
		Group:    "gateway.networking.k8s.io",
		Version:  "v1",
		Resource: "gateways",
	}
	HTTPRouteGVR = schema.GroupVersionResource{
		Group:    "gateway.networking.k8s.io",
		Version:  "v1",
		Resource: "httproutes",
	}
	GRPCRouteGVR = schema.GroupVersionResource{
		Group:    "gateway.networking.k8s.io",
		Version:  "v1",
		Resource: "grpcroutes",
	}
	ReferenceGrantGVR = schema.GroupVersionResource{
		Group:    "gateway.networking.k8s.io",
		Version:  "v1beta1",
		Resource: "referencegrants",
	}
)

// --- DTO 定義 ---

type GatewayK8sCondition struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

type GatewayClassItem struct {
	Name           string `json:"name"`
	Controller     string `json:"controller"`
	Description    string `json:"description"`
	AcceptedStatus string `json:"acceptedStatus"` // "Accepted" | "Unknown" | reason string
	CreatedAt      string `json:"createdAt"`
}

type GatewayListener struct {
	Name     string `json:"name"`
	Port     int64  `json:"port"`
	Protocol string `json:"protocol"`
	Hostname string `json:"hostname"`
	TLSMode  string `json:"tlsMode"`
	Status   string `json:"status"` // "Ready" | "Detached" | "Conflicted" | "Unknown"
}

type GatewayAddress struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type GatewayItem struct {
	Name         string                `json:"name"`
	Namespace    string                `json:"namespace"`
	GatewayClass string                `json:"gatewayClass"`
	Listeners    []GatewayListener     `json:"listeners"`
	Addresses    []GatewayAddress      `json:"addresses"`
	Conditions   []GatewayK8sCondition `json:"conditions"`
	CreatedAt    string                `json:"createdAt"`
}

type HTTPRouteBackend struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Port      int64  `json:"port"`
	Weight    int64  `json:"weight"`
}

type HTTPRouteRule struct {
	Matches  []map[string]interface{} `json:"matches"`
	Filters  []map[string]interface{} `json:"filters"`
	Backends []HTTPRouteBackend       `json:"backends"`
}

type HTTPRouteParentRef struct {
	GatewayNamespace string                `json:"gatewayNamespace"`
	GatewayName      string                `json:"gatewayName"`
	SectionName      string                `json:"sectionName"`
	Conditions       []GatewayK8sCondition `json:"conditions"`
}

type HTTPRouteItem struct {
	Name       string                `json:"name"`
	Namespace  string                `json:"namespace"`
	Hostnames  []string              `json:"hostnames"`
	ParentRefs []HTTPRouteParentRef  `json:"parentRefs"`
	Rules      []HTTPRouteRule       `json:"rules"`
	Conditions []GatewayK8sCondition `json:"conditions"`
	CreatedAt  string                `json:"createdAt"`
}

// --- GRPCRoute DTO ---

type GRPCRouteMethod struct {
	Service string `json:"service"`
	Method  string `json:"method,omitempty"`
}

type GRPCRouteMatch struct {
	Method *GRPCRouteMethod `json:"method,omitempty"`
}

type GRPCRouteBackend struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Port      int64  `json:"port"`
	Weight    int64  `json:"weight"`
}

type GRPCRouteRule struct {
	Matches  []GRPCRouteMatch  `json:"matches"`
	Backends []GRPCRouteBackend `json:"backends"`
}

type GRPCRouteItem struct {
	Name       string                `json:"name"`
	Namespace  string                `json:"namespace"`
	Hostnames  []string              `json:"hostnames"`
	ParentRefs []HTTPRouteParentRef  `json:"parentRefs"`
	Rules      []GRPCRouteRule       `json:"rules"`
	Conditions []GatewayK8sCondition `json:"conditions"`
	CreatedAt  string                `json:"createdAt"`
}

// --- ReferenceGrant DTO ---

type ReferenceGrantPeer struct {
	Group     string `json:"group"`
	Kind      string `json:"kind"`
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name,omitempty"`
}

type ReferenceGrantItem struct {
	Name      string               `json:"name"`
	Namespace string               `json:"namespace"`
	From      []ReferenceGrantPeer `json:"from"`
	To        []ReferenceGrantPeer `json:"to"`
	CreatedAt string               `json:"createdAt"`
}

// --- Topology DTO ---

type TopologyNode struct {
	ID        string   `json:"id"`
	Kind      string   `json:"kind"`                // GatewayClass | Gateway | HTTPRoute | GRPCRoute | Service
	Name      string   `json:"name"`
	Namespace string   `json:"namespace,omitempty"`
	Status    string   `json:"status,omitempty"`
	SubKind   string   `json:"subKind,omitempty"`   // listener summary e.g. "HTTP:80" | service type
	Hostnames []string `json:"hostnames,omitempty"` // HTTPRoute / GRPCRoute hostnames
}

type TopologyEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

type TopologyData struct {
	Nodes []TopologyNode `json:"nodes"`
	Edges []TopologyEdge `json:"edges"`
}

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

func toGatewayClassItem(obj unstructured.Unstructured) GatewayClassItem {
	spec := gwGetMap(obj.Object, "spec")
	controller := gwGetString(spec, "controllerName")
	description := gwGetString(spec, "description")

	acceptedStatus := "Unknown"
	for _, cond := range gwGetConditions(obj.Object, "status", "conditions") {
		if cond.Type == "Accepted" {
			if cond.Status == "True" {
				acceptedStatus = "Accepted"
			} else {
				acceptedStatus = cond.Reason
			}
			break
		}
	}

	return GatewayClassItem{
		Name:           obj.GetName(),
		Controller:     controller,
		Description:    description,
		AcceptedStatus: acceptedStatus,
		CreatedAt:      obj.GetCreationTimestamp().UTC().Format("2006-01-02T15:04:05Z"),
	}
}

func toGatewayItem(obj unstructured.Unstructured) GatewayItem {
	spec := gwGetMap(obj.Object, "spec")
	status := gwGetMap(obj.Object, "status")

	// Listeners
	listenerStatusMap := map[string]string{}
	for _, ls := range gwGetSlice(status, "listeners") {
		lsm, ok := ls.(map[string]interface{})
		if !ok {
			continue
		}
		lname := gwGetString(lsm, "name")
		for _, cond := range gwGetConditions(lsm, "conditions") {
			if cond.Type == "Ready" || cond.Type == "Programmed" {
				if cond.Status == "True" {
					listenerStatusMap[lname] = "Ready"
				} else {
					listenerStatusMap[lname] = cond.Reason
				}
				break
			}
		}
	}

	listeners := make([]GatewayListener, 0)
	for _, l := range gwGetSlice(spec, "listeners") {
		lm, ok := l.(map[string]interface{})
		if !ok {
			continue
		}
		name := gwGetString(lm, "name")
		tlsMode := ""
		if tlsMap := gwGetMap(lm, "tls"); tlsMap != nil {
			tlsMode = gwGetString(tlsMap, "mode")
		}
		lStatus := listenerStatusMap[name]
		if lStatus == "" {
			lStatus = "Unknown"
		}
		listeners = append(listeners, GatewayListener{
			Name:     name,
			Port:     gwGetInt64(lm, "port"),
			Protocol: gwGetString(lm, "protocol"),
			Hostname: gwGetString(lm, "hostname"),
			TLSMode:  tlsMode,
			Status:   lStatus,
		})
	}

	// Addresses
	addresses := make([]GatewayAddress, 0)
	for _, a := range gwGetSlice(status, "addresses") {
		am, ok := a.(map[string]interface{})
		if !ok {
			continue
		}
		addresses = append(addresses, GatewayAddress{
			Type:  gwGetString(am, "type"),
			Value: gwGetString(am, "value"),
		})
	}

	return GatewayItem{
		Name:         obj.GetName(),
		Namespace:    obj.GetNamespace(),
		GatewayClass: gwGetString(spec, "gatewayClassName"),
		Listeners:    listeners,
		Addresses:    addresses,
		Conditions:   gwGetConditions(obj.Object, "status", "conditions"),
		CreatedAt:    obj.GetCreationTimestamp().UTC().Format("2006-01-02T15:04:05Z"),
	}
}

func toHTTPRouteItem(obj unstructured.Unstructured) HTTPRouteItem {
	spec := gwGetMap(obj.Object, "spec")
	status := gwGetMap(obj.Object, "status")

	// hostnames
	hostnames := make([]string, 0)
	for _, h := range gwGetSlice(spec, "hostnames") {
		if hs, ok := h.(string); ok {
			hostnames = append(hostnames, hs)
		}
	}

	// Build status parent map: ns/name -> conditions
	statusCondMap := map[string][]GatewayK8sCondition{}
	for _, p := range gwGetSlice(status, "parents") {
		pm, ok := p.(map[string]interface{})
		if !ok {
			continue
		}
		prm := gwGetMap(pm, "parentRef")
		ns := gwGetString(prm, "namespace")
		if ns == "" {
			ns = obj.GetNamespace()
		}
		key := ns + "/" + gwGetString(prm, "name")
		statusCondMap[key] = gwGetConditions(pm, "conditions")
	}

	// parentRefs
	parentRefs := make([]HTTPRouteParentRef, 0)
	for _, p := range gwGetSlice(spec, "parentRefs") {
		pm, ok := p.(map[string]interface{})
		if !ok {
			continue
		}
		ns := gwGetString(pm, "namespace")
		if ns == "" {
			ns = obj.GetNamespace()
		}
		name := gwGetString(pm, "name")
		parentRefs = append(parentRefs, HTTPRouteParentRef{
			GatewayNamespace: ns,
			GatewayName:      name,
			SectionName:      gwGetString(pm, "sectionName"),
			Conditions:       statusCondMap[ns+"/"+name],
		})
	}

	// rules
	rules := make([]HTTPRouteRule, 0)
	for _, r := range gwGetSlice(spec, "rules") {
		rm, ok := r.(map[string]interface{})
		if !ok {
			continue
		}
		matches := make([]map[string]interface{}, 0)
		for _, m := range gwGetSlice(rm, "matches") {
			if mm, ok := m.(map[string]interface{}); ok {
				matches = append(matches, mm)
			}
		}
		filters := make([]map[string]interface{}, 0)
		for _, f := range gwGetSlice(rm, "filters") {
			if fm, ok := f.(map[string]interface{}); ok {
				filters = append(filters, fm)
			}
		}
		backends := make([]HTTPRouteBackend, 0)
		for _, b := range gwGetSlice(rm, "backendRefs") {
			bm, ok := b.(map[string]interface{})
			if !ok {
				continue
			}
			bns := gwGetString(bm, "namespace")
			if bns == "" {
				bns = obj.GetNamespace()
			}
			weight := gwGetInt64(bm, "weight")
			if weight == 0 {
				weight = 1
			}
			backends = append(backends, HTTPRouteBackend{
				Name:      gwGetString(bm, "name"),
				Namespace: bns,
				Port:      gwGetInt64(bm, "port"),
				Weight:    weight,
			})
		}
		rules = append(rules, HTTPRouteRule{
			Matches:  matches,
			Filters:  filters,
			Backends: backends,
		})
	}

	// Aggregate conditions from all parents
	conditions := make([]GatewayK8sCondition, 0)
	for _, conds := range statusCondMap {
		conditions = append(conditions, conds...)
	}

	return HTTPRouteItem{
		Name:       obj.GetName(),
		Namespace:  obj.GetNamespace(),
		Hostnames:  hostnames,
		ParentRefs: parentRefs,
		Rules:      rules,
		Conditions: conditions,
		CreatedAt:  obj.GetCreationTimestamp().UTC().Format("2006-01-02T15:04:05Z"),
	}
}

// --- CRUD（Phase 2）---

// CreateGateway 從 YAML 建立 Gateway
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

func toGRPCRouteItem(obj unstructured.Unstructured) GRPCRouteItem {
	spec := gwGetMap(obj.Object, "spec")
	status := gwGetMap(obj.Object, "status")

	hostnames := make([]string, 0)
	for _, h := range gwGetSlice(spec, "hostnames") {
		if hs, ok := h.(string); ok {
			hostnames = append(hostnames, hs)
		}
	}

	statusCondMap := map[string][]GatewayK8sCondition{}
	for _, p := range gwGetSlice(status, "parents") {
		pm, ok := p.(map[string]interface{})
		if !ok {
			continue
		}
		prm := gwGetMap(pm, "parentRef")
		ns := gwGetString(prm, "namespace")
		if ns == "" {
			ns = obj.GetNamespace()
		}
		key := ns + "/" + gwGetString(prm, "name")
		statusCondMap[key] = gwGetConditions(pm, "conditions")
	}

	parentRefs := make([]HTTPRouteParentRef, 0)
	for _, p := range gwGetSlice(spec, "parentRefs") {
		pm, ok := p.(map[string]interface{})
		if !ok {
			continue
		}
		ns := gwGetString(pm, "namespace")
		if ns == "" {
			ns = obj.GetNamespace()
		}
		name := gwGetString(pm, "name")
		parentRefs = append(parentRefs, HTTPRouteParentRef{
			GatewayNamespace: ns,
			GatewayName:      name,
			SectionName:      gwGetString(pm, "sectionName"),
			Conditions:       statusCondMap[ns+"/"+name],
		})
	}

	rules := make([]GRPCRouteRule, 0)
	for _, r := range gwGetSlice(spec, "rules") {
		rm, ok := r.(map[string]interface{})
		if !ok {
			continue
		}
		matches := make([]GRPCRouteMatch, 0)
		for _, m := range gwGetSlice(rm, "matches") {
			mm, ok := m.(map[string]interface{})
			if !ok {
				continue
			}
			var method *GRPCRouteMethod
			if methodMap := gwGetMap(mm, "method"); methodMap != nil {
				method = &GRPCRouteMethod{
					Service: gwGetString(methodMap, "service"),
					Method:  gwGetString(methodMap, "method"),
				}
			}
			matches = append(matches, GRPCRouteMatch{Method: method})
		}
		backends := make([]GRPCRouteBackend, 0)
		for _, b := range gwGetSlice(rm, "backendRefs") {
			bm, ok := b.(map[string]interface{})
			if !ok {
				continue
			}
			bns := gwGetString(bm, "namespace")
			if bns == "" {
				bns = obj.GetNamespace()
			}
			weight := gwGetInt64(bm, "weight")
			if weight == 0 {
				weight = 1
			}
			backends = append(backends, GRPCRouteBackend{
				Name:      gwGetString(bm, "name"),
				Namespace: bns,
				Port:      gwGetInt64(bm, "port"),
				Weight:    weight,
			})
		}
		rules = append(rules, GRPCRouteRule{Matches: matches, Backends: backends})
	}

	conditions := make([]GatewayK8sCondition, 0)
	for _, conds := range statusCondMap {
		conditions = append(conditions, conds...)
	}

	return GRPCRouteItem{
		Name:       obj.GetName(),
		Namespace:  obj.GetNamespace(),
		Hostnames:  hostnames,
		ParentRefs: parentRefs,
		Rules:      rules,
		Conditions: conditions,
		CreatedAt:  obj.GetCreationTimestamp().UTC().Format("2006-01-02T15:04:05Z"),
	}
}

func toReferenceGrantItem(obj unstructured.Unstructured) ReferenceGrantItem {
	spec := gwGetMap(obj.Object, "spec")

	parsePeers := func(key string) []ReferenceGrantPeer {
		peers := make([]ReferenceGrantPeer, 0)
		for _, p := range gwGetSlice(spec, key) {
			pm, ok := p.(map[string]interface{})
			if !ok {
				continue
			}
			peers = append(peers, ReferenceGrantPeer{
				Group:     gwGetString(pm, "group"),
				Kind:      gwGetString(pm, "kind"),
				Namespace: gwGetString(pm, "namespace"),
				Name:      gwGetString(pm, "name"),
			})
		}
		return peers
	}

	return ReferenceGrantItem{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
		From:      parsePeers("from"),
		To:        parsePeers("to"),
		CreatedAt: obj.GetCreationTimestamp().UTC().Format("2006-01-02T15:04:05Z"),
	}
}

// gwParseYAML 將 YAML 字串解析為 Unstructured 物件
func gwParseYAML(yamlStr string) (*unstructured.Unstructured, error) {
	var data map[string]interface{}
	if err := yaml.Unmarshal([]byte(yamlStr), &data); err != nil {
		return nil, fmt.Errorf("YAML 格式錯誤: %v", err)
	}
	if data == nil {
		return nil, fmt.Errorf("YAML 內容為空")
	}
	return &unstructured.Unstructured{Object: data}, nil
}

// --- 工具函式（gw 前綴避免與其他 service 衝突）---

func gwGetConditions(obj interface{}, path ...string) []GatewayK8sCondition {
	var current interface{} = obj
	for _, key := range path {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil
		}
		current = m[key]
	}
	slice, ok := current.([]interface{})
	if !ok {
		return nil
	}
	result := make([]GatewayK8sCondition, 0, len(slice))
	for _, item := range slice {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		result = append(result, GatewayK8sCondition{
			Type:    gwGetString(m, "type"),
			Status:  gwGetString(m, "status"),
			Reason:  gwGetString(m, "reason"),
			Message: gwGetString(m, "message"),
		})
	}
	return result
}

func gwGetMap(obj map[string]interface{}, key string) map[string]interface{} {
	if obj == nil {
		return nil
	}
	v, _ := obj[key].(map[string]interface{})
	return v
}

func gwGetSlice(obj map[string]interface{}, key string) []interface{} {
	if obj == nil {
		return nil
	}
	v, _ := obj[key].([]interface{})
	return v
}

func gwGetString(obj map[string]interface{}, key string) string {
	if obj == nil {
		return ""
	}
	v, _ := obj[key].(string)
	return v
}

func gwGetInt64(obj map[string]interface{}, key string) int64 {
	if obj == nil {
		return 0
	}
	switch v := obj[key].(type) {
	case int64:
		return v
	case float64:
		return int64(v)
	case int:
		return int64(v)
	}
	return 0
}

func gwCleanMeta(obj *unstructured.Unstructured) {
	if meta, ok := obj.Object["metadata"].(map[string]interface{}); ok {
		delete(meta, "managedFields")
	}
}
