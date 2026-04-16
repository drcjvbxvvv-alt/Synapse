package services

import (
	"testing"
)

// TestIsDeployStepType verifies the deploy vs non-deploy categorisation used
// by the rollback scheduler to decide which steps to skip.
func TestIsDeployStepType(t *testing.T) {
	deployTypes := []string{
		"deploy", "deploy-helm", "deploy-argocd-sync", "deploy-rollout",
		"rollout-promote", "rollout-abort", "rollout-status", "gitops-sync",
	}
	for _, st := range deployTypes {
		if !IsDeployStepType(st) {
			t.Errorf("IsDeployStepType(%q) = false, want true", st)
		}
	}

	nonDeployTypes := []string{
		"build-image", "build-jar", "trivy-scan", "push-image",
		"run-script", "shell", "approval", "notify", "smoke-test", "custom",
	}
	for _, st := range nonDeployTypes {
		if IsDeployStepType(st) {
			t.Errorf("IsDeployStepType(%q) = true, want false", st)
		}
	}
}
