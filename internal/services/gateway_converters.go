package services

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

// ── DTO converters & low-level helpers ─────────────────────────────────────
// Package-level functions that convert unstructured.Unstructured objects to
// typed Gateway API DTOs, plus gw* accessor helpers.
// Extracted from gateway_service.go to reduce file size.

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
