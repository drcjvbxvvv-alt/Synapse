package services

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
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
