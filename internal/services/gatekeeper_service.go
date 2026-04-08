package services

import (
	"context"
	"fmt"
	"strings"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// GatekeeperViolation represents a single OPA/Gatekeeper policy violation.
type GatekeeperViolation struct {
	ConstraintKind string `json:"constraint_kind"`
	ConstraintName string `json:"constraint_name"`
	Resource       string `json:"resource"`
	Namespace      string `json:"namespace"`
	Message        string `json:"message"`
}

// GatekeeperSummary groups violations by constraint type.
type GatekeeperSummary struct {
	Installed       bool                  `json:"installed"`
	TotalViolations int                   `json:"total_violations"`
	Constraints     []ConstraintSummary   `json:"constraints"`
}

type ConstraintSummary struct {
	Kind           string                `json:"kind"`
	Name           string                `json:"name"`
	ViolationCount int                   `json:"violation_count"`
	Violations     []GatekeeperViolation `json:"violations"`
}

var constraintTemplateGVR = schema.GroupVersionResource{
	Group:    "templates.gatekeeper.sh",
	Version:  "v1beta1",
	Resource: "constrainttemplates",
}

// GetGatekeeperViolations queries all Gatekeeper constraint CRDs and collects violations.
func GetGatekeeperViolations(k8sClient *K8sClient) (*GatekeeperSummary, error) {
	restConfig := k8sClient.GetRestConfig()
	dynClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	ctx := context.Background()

	// List all ConstraintTemplates to discover constraint kinds
	templateList, err := dynClient.Resource(constraintTemplateGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		// Gatekeeper CRDs not present — treat as not installed, not an error
		if k8serrors.IsNotFound(err) || isNoMatchError(err) {
			return &GatekeeperSummary{Installed: false, Constraints: []ConstraintSummary{}}, nil
		}
		return nil, fmt.Errorf("failed to list ConstraintTemplates: %w", err)
	}

	summary := &GatekeeperSummary{Installed: true}

	for _, template := range templateList.Items {
		// The constraint kind is the CRD name from spec.crd.spec.names.kind
		spec, ok := template.Object["spec"].(map[string]interface{})
		if !ok {
			continue
		}
		crd, ok := spec["crd"].(map[string]interface{})
		if !ok {
			continue
		}
		crdSpec, ok := crd["spec"].(map[string]interface{})
		if !ok {
			continue
		}
		names, ok := crdSpec["names"].(map[string]interface{})
		if !ok {
			continue
		}
		kind, _ := names["kind"].(string)
		if kind == "" {
			continue
		}

		// Query constraint instances of this kind
		constraintGVR := schema.GroupVersionResource{
			Group:    "constraints.gatekeeper.sh",
			Version:  "v1beta1",
			Resource: resourceNameForKind(kind),
		}

		constraintList, err := dynClient.Resource(constraintGVR).List(ctx, metav1.ListOptions{})
		if err != nil {
			// This constraint kind may not have any instances — skip
			continue
		}

		for _, constraint := range constraintList.Items {
			constraintName := constraint.GetName()
			cs := ConstraintSummary{
				Kind: kind,
				Name: constraintName,
			}

			status, ok := constraint.Object["status"].(map[string]interface{})
			if !ok {
				summary.Constraints = append(summary.Constraints, cs)
				continue
			}

			violations, ok := status["violations"].([]interface{})
			if !ok {
				summary.Constraints = append(summary.Constraints, cs)
				continue
			}

			for _, v := range violations {
				vm, ok := v.(map[string]interface{})
				if !ok {
					continue
				}
				viol := GatekeeperViolation{
					ConstraintKind: kind,
					ConstraintName: constraintName,
				}
				if ns, ok := vm["namespace"].(string); ok {
					viol.Namespace = ns
				}
				if name, ok := vm["name"].(string); ok {
					viol.Resource = name
				}
				if msg, ok := vm["message"].(string); ok {
					viol.Message = msg
				}
				cs.Violations = append(cs.Violations, viol)
			}
			cs.ViolationCount = len(cs.Violations)
			summary.TotalViolations += cs.ViolationCount
			summary.Constraints = append(summary.Constraints, cs)
		}
	}

	return summary, nil
}

// isNoMatchError returns true when the API server doesn't know the resource type
// (e.g. Gatekeeper CRDs not installed).
func isNoMatchError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "no matches for kind") ||
		strings.Contains(msg, "no kind is registered") ||
		strings.Contains(msg, "the server could not find the requested resource")
}

// resourceNameForKind converts a CamelCase Kind to a lowercase plural resource name.
// e.g. K8sRequiredLabels → k8srequiredlabels
func resourceNameForKind(kind string) string {
	result := ""
	for _, ch := range kind {
		if ch >= 'A' && ch <= 'Z' {
			result += string(ch + 32)
		} else {
			result += string(ch)
		}
	}
	return result + "s"
}
