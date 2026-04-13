package models

import "encoding/json"

// Feature key constants — used in FeaturePolicy JSON and frontend hasFeature() calls.
const (
	FeatureWorkloadView   = "workload:view"
	FeatureWorkloadWrite  = "workload:write"
	FeatureWorkloadDelete = "workload:delete"
	FeatureNetworkView    = "network:view"
	FeatureNetworkWrite   = "network:write"
	FeatureNetworkDelete  = "network:delete"
	FeatureStorageView    = "storage:view"
	FeatureStorageWrite   = "storage:write"
	FeatureStorageDelete  = "storage:delete"
	FeatureNodeView       = "node:view"
	FeatureNodeManage     = "node:manage"
	FeatureNamespaceView   = "namespace:view"
	FeatureNamespaceWrite  = "namespace:write"
	FeatureNamespaceDelete = "namespace:delete"
	FeatureConfigView     = "config:view"
	FeatureConfigWrite    = "config:write"
	FeatureConfigDelete   = "config:delete"
	FeatureTerminalPod    = "terminal:pod"
	FeatureTerminalNode   = "terminal:node"
	FeatureLogsView       = "logs:view"
	FeatureMonitoringView  = "monitoring:view"
	FeatureAlertsView      = "alerts:view"
	FeatureEventAlertsView = "event_alerts:view"
	FeatureCostView        = "cost:view"
	FeatureSecurityView    = "security:view"
	FeatureCertificatesView = "certificates:view"
	FeatureSLOView         = "slo:view"
	FeatureChaosView       = "chaos:view"
	FeatureComplianceView  = "compliance:view"
	FeatureHelmView        = "helm:view"
	FeatureHelmWrite       = "helm:write"
	FeatureExport          = "export"
	FeatureAIAssistant     = "ai_assistant"
)

// FeatureCeilings defines the maximum set of feature keys each permission_type
// can ever have enabled. Feature policy can only restrict within this ceiling —
// it cannot grant features that exceed the type's ceiling.
//
// For non-readonly roles the ceiling is the full feature set. The actual access
// boundary is enforced by the permission_type check in route/handler middleware
// (e.g. ops cannot exec into nodes even if terminal:node is in their ceiling).
// Feature policy is an admin-configurable restriction layer on top of RBAC.
var FeatureCeilings = map[string][]string{
	// admin — all features enabled by default, fully configurable
	PermissionTypeAdmin: allFeatureKeys(),
	// ops — all features in ceiling; RBAC still gates backend operations
	PermissionTypeOps: allFeatureKeys(),
	// dev — all features in ceiling; RBAC still gates backend operations
	PermissionTypeDev: allFeatureKeys(),
	// readonly — hard-limited to read-only views, no writes/terminal/export
	PermissionTypeReadonly: {
		FeatureWorkloadView,
		FeatureNetworkView,
		FeatureStorageView,
		FeatureNodeView,
		FeatureNamespaceView,
		FeatureConfigView,
		FeatureLogsView, FeatureMonitoringView,
		FeatureHelmView,
	},
	// custom — all features; RBAC controls access, not feature policy
	PermissionTypeCustom: allFeatureKeys(),
}

// allFeatureKeys returns every defined feature key.
func allFeatureKeys() []string {
	return []string{
		FeatureWorkloadView, FeatureWorkloadWrite, FeatureWorkloadDelete,
		FeatureNetworkView, FeatureNetworkWrite, FeatureNetworkDelete,
		FeatureStorageView, FeatureStorageWrite, FeatureStorageDelete,
		FeatureNodeView, FeatureNodeManage,
		FeatureNamespaceView, FeatureNamespaceWrite, FeatureNamespaceDelete,
		FeatureConfigView, FeatureConfigWrite, FeatureConfigDelete,
		FeatureTerminalPod, FeatureTerminalNode,
		FeatureLogsView, FeatureMonitoringView,
		FeatureAlertsView, FeatureEventAlertsView,
		FeatureCostView, FeatureSecurityView, FeatureCertificatesView,
		FeatureSLOView, FeatureChaosView, FeatureComplianceView,
		FeatureHelmView, FeatureHelmWrite,
		FeatureExport, FeatureAIAssistant,
	}
}

// ComputeAllowedFeatures returns the effective feature set for a permission record.
// It intersects the permission type's ceiling with the feature policy overrides.
// Keys absent from the policy map default to allowed (within the ceiling).
func ComputeAllowedFeatures(permType string, featurePolicyJSON string) []string {
	ceiling, ok := FeatureCeilings[permType]
	if !ok {
		return []string{}
	}

	// Parse the policy overrides (only explicit false entries matter).
	policy := map[string]bool{}
	if featurePolicyJSON != "" {
		_ = json.Unmarshal([]byte(featurePolicyJSON), &policy)
	}

	result := make([]string, 0, len(ceiling))
	for _, key := range ceiling {
		// If the policy explicitly disables this key, exclude it.
		if v, exists := policy[key]; exists && !v {
			continue
		}
		result = append(result, key)
	}
	return result
}

// GetFeaturePolicyMap returns the decoded feature_policy JSON as a map.
// Returns an empty map when the field is empty or invalid JSON.
func (cp *ClusterPermission) GetFeaturePolicyMap() map[string]bool {
	m := map[string]bool{}
	if cp.FeaturePolicy != "" {
		_ = json.Unmarshal([]byte(cp.FeaturePolicy), &m)
	}
	return m
}

// SetFeaturePolicyMap encodes the map into the FeaturePolicy JSON field.
func (cp *ClusterPermission) SetFeaturePolicyMap(m map[string]bool) error {
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	cp.FeaturePolicy = string(data)
	return nil
}
