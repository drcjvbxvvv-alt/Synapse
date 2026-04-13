package models

import "encoding/json"

// Feature key constants — used in FeaturePolicy JSON and frontend hasFeature() calls.
const (
	FeatureWorkloadView  = "workload:view"
	FeatureWorkloadWrite = "workload:write"
	FeatureNetworkView   = "network:view"
	FeatureNetworkWrite  = "network:write"
	FeatureStorageView   = "storage:view"
	FeatureStorageWrite  = "storage:write"
	FeatureNodeView      = "node:view"
	FeatureNodeManage    = "node:manage"
	FeatureConfigView    = "config:view"
	FeatureConfigWrite   = "config:write"
	FeatureTerminalPod   = "terminal:pod"
	FeatureTerminalNode  = "terminal:node"
	FeatureLogsView      = "logs:view"
	FeatureMonitoringView = "monitoring:view"
	FeatureHelmView      = "helm:view"
	FeatureHelmWrite     = "helm:write"
	FeatureExport        = "export"
	FeatureAIAssistant   = "ai_assistant"
)

// FeatureCeilings defines the maximum set of feature keys each permission_type
// can ever have enabled. Feature policy can only restrict within this ceiling —
// it cannot grant features that exceed the type's ceiling.
var FeatureCeilings = map[string][]string{
	PermissionTypeAdmin: {
		FeatureWorkloadView, FeatureWorkloadWrite,
		FeatureNetworkView, FeatureNetworkWrite,
		FeatureStorageView, FeatureStorageWrite,
		FeatureNodeView, FeatureNodeManage,
		FeatureConfigView, FeatureConfigWrite,
		FeatureTerminalPod, FeatureTerminalNode,
		FeatureLogsView, FeatureMonitoringView,
		FeatureHelmView, FeatureHelmWrite,
		FeatureExport, FeatureAIAssistant,
	},
	PermissionTypeOps: {
		FeatureWorkloadView, FeatureWorkloadWrite,
		FeatureNetworkView, FeatureNetworkWrite,
		FeatureStorageView, // storage:write excluded
		FeatureNodeView,    // node:manage excluded
		FeatureConfigView, FeatureConfigWrite,
		FeatureTerminalPod, FeatureTerminalNode,
		FeatureLogsView, FeatureMonitoringView,
		FeatureHelmView, FeatureHelmWrite,
		FeatureExport, FeatureAIAssistant,
	},
	PermissionTypeDev: {
		FeatureWorkloadView, FeatureWorkloadWrite,
		FeatureNetworkView, FeatureNetworkWrite,
		FeatureConfigView, FeatureConfigWrite,
		FeatureTerminalPod, // terminal:node excluded
		FeatureLogsView, FeatureMonitoringView,
		FeatureExport, FeatureAIAssistant,
		// storage, node, helm excluded
	},
	PermissionTypeReadonly: {
		FeatureWorkloadView,
		FeatureNetworkView,
		FeatureStorageView,
		FeatureNodeView,
		FeatureConfigView,
		FeatureLogsView, FeatureMonitoringView,
		FeatureHelmView,
		// no :write, no terminal, no export, no ai_assistant
	},
	// custom: all features — RBAC controls access, not feature policy
	PermissionTypeCustom: allFeatureKeys(),
}

// allFeatureKeys returns every defined feature key.
func allFeatureKeys() []string {
	return []string{
		FeatureWorkloadView, FeatureWorkloadWrite,
		FeatureNetworkView, FeatureNetworkWrite,
		FeatureStorageView, FeatureStorageWrite,
		FeatureNodeView, FeatureNodeManage,
		FeatureConfigView, FeatureConfigWrite,
		FeatureTerminalPod, FeatureTerminalNode,
		FeatureLogsView, FeatureMonitoringView,
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
