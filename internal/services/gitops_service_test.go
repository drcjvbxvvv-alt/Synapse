package services

import (
	"strings"
	"testing"

	"github.com/shaia/Synapse/internal/models"
)

// ---------------------------------------------------------------------------
// validateGitOpsApp
// ---------------------------------------------------------------------------

func TestValidateGitOpsApp_ValidNative(t *testing.T) {
	app := &models.GitOpsApp{
		Name:         "my-app",
		Source:       "native",
		ClusterID:    1,
		Namespace:    "default",
		RepoURL:      "https://github.com/org/repo",
		Branch:       "main",
		RenderType:   "raw",
		SyncPolicy:   "manual",
		SyncInterval: 300,
	}
	if err := validateGitOpsApp(app); err != nil {
		t.Errorf("expected valid: %v", err)
	}
}

func TestValidateGitOpsApp_ValidArgoCD(t *testing.T) {
	app := &models.GitOpsApp{
		Name:         "argo-app",
		Source:       "argocd",
		ClusterID:    1,
		Namespace:    "production",
		RenderType:   "helm",
		SyncPolicy:   "auto",
		SyncInterval: 60,
	}
	if err := validateGitOpsApp(app); err != nil {
		t.Errorf("expected valid: %v", err)
	}
}

func TestValidateGitOpsApp_MissingName(t *testing.T) {
	app := &models.GitOpsApp{
		Source:       "native",
		ClusterID:    1,
		Namespace:    "default",
		RepoURL:      "https://github.com/org/repo",
		Branch:       "main",
		RenderType:   "raw",
		SyncPolicy:   "manual",
		SyncInterval: 300,
	}
	if err := validateGitOpsApp(app); err == nil {
		t.Error("expected error for missing name")
	}
}

func TestValidateGitOpsApp_InvalidSource(t *testing.T) {
	app := &models.GitOpsApp{
		Name:         "app",
		Source:       "flux",
		ClusterID:    1,
		Namespace:    "default",
		RenderType:   "raw",
		SyncPolicy:   "manual",
		SyncInterval: 300,
	}
	err := validateGitOpsApp(app)
	if err == nil {
		t.Error("expected error for invalid source")
	}
	if !strings.Contains(err.Error(), "native|argocd") {
		t.Errorf("error should list valid sources: %v", err)
	}
}

func TestValidateGitOpsApp_InvalidRenderType(t *testing.T) {
	app := &models.GitOpsApp{
		Name:         "app",
		Source:       "argocd",
		ClusterID:    1,
		Namespace:    "default",
		RenderType:   "jsonnet",
		SyncPolicy:   "manual",
		SyncInterval: 300,
	}
	if err := validateGitOpsApp(app); err == nil {
		t.Error("expected error for invalid render_type")
	}
}

func TestValidateGitOpsApp_InvalidSyncPolicy(t *testing.T) {
	app := &models.GitOpsApp{
		Name:         "app",
		Source:       "argocd",
		ClusterID:    1,
		Namespace:    "default",
		RenderType:   "raw",
		SyncPolicy:   "periodic",
		SyncInterval: 300,
	}
	if err := validateGitOpsApp(app); err == nil {
		t.Error("expected error for invalid sync_policy")
	}
}

func TestValidateGitOpsApp_NativeMissingRepo(t *testing.T) {
	app := &models.GitOpsApp{
		Name:         "app",
		Source:       "native",
		ClusterID:    1,
		Namespace:    "default",
		Branch:       "main",
		RenderType:   "raw",
		SyncPolicy:   "manual",
		SyncInterval: 300,
	}
	err := validateGitOpsApp(app)
	if err == nil {
		t.Error("expected error for native without repo_url")
	}
	if !strings.Contains(err.Error(), "repo_url") {
		t.Errorf("error should mention repo_url: %v", err)
	}
}

func TestValidateGitOpsApp_NativeMissingBranch(t *testing.T) {
	app := &models.GitOpsApp{
		Name:         "app",
		Source:       "native",
		ClusterID:    1,
		Namespace:    "default",
		RepoURL:      "https://github.com/org/repo",
		RenderType:   "raw",
		SyncPolicy:   "manual",
		SyncInterval: 300,
	}
	if err := validateGitOpsApp(app); err == nil {
		t.Error("expected error for native without branch")
	}
}

func TestValidateGitOpsApp_SyncIntervalTooLow(t *testing.T) {
	app := &models.GitOpsApp{
		Name:         "app",
		Source:       "argocd",
		ClusterID:    1,
		Namespace:    "default",
		RenderType:   "raw",
		SyncPolicy:   "manual",
		SyncInterval: 10,
	}
	err := validateGitOpsApp(app)
	if err == nil {
		t.Error("expected error for sync_interval < 30")
	}
}

func TestValidateGitOpsApp_SyncIntervalTooHigh(t *testing.T) {
	app := &models.GitOpsApp{
		Name:         "app",
		Source:       "argocd",
		ClusterID:    1,
		Namespace:    "default",
		RenderType:   "raw",
		SyncPolicy:   "manual",
		SyncInterval: 100000,
	}
	err := validateGitOpsApp(app)
	if err == nil {
		t.Error("expected error for sync_interval > 86400")
	}
}

// ---------------------------------------------------------------------------
// validateGitOpsSource / validateGitOpsRenderType / validateGitOpsSyncPolicy
// ---------------------------------------------------------------------------

func TestValidateGitOpsSource_Valid(t *testing.T) {
	for _, s := range []string{"native", "argocd"} {
		if err := validateGitOpsSource(s); err != nil {
			t.Errorf("expected valid for %s: %v", s, err)
		}
	}
}

func TestValidateGitOpsRenderType_Valid(t *testing.T) {
	for _, rt := range []string{"raw", "kustomize", "helm"} {
		if err := validateGitOpsRenderType(rt); err != nil {
			t.Errorf("expected valid for %s: %v", rt, err)
		}
	}
}

func TestValidateGitOpsSyncPolicy_Valid(t *testing.T) {
	for _, sp := range []string{"auto", "manual"} {
		if err := validateGitOpsSyncPolicy(sp); err != nil {
			t.Errorf("expected valid for %s: %v", sp, err)
		}
	}
}

// ---------------------------------------------------------------------------
// ValidateDeployStepExclusion（§12.1 互斥規則）
// ---------------------------------------------------------------------------

func TestValidateDeployStepExclusion_NoConflict(t *testing.T) {
	steps := []StepDeployTarget{
		{StepType: "deploy-argocd-sync", AppName: "app-a", Namespace: "ns1"},
		{StepType: "gitops-sync", AppName: "app-b", Namespace: "ns2"},
	}
	errors := ValidateDeployStepExclusion(steps)
	if len(errors) != 0 {
		t.Errorf("expected no errors, got %v", errors)
	}
}

func TestValidateDeployStepExclusion_Conflict(t *testing.T) {
	steps := []StepDeployTarget{
		{StepType: "deploy-argocd-sync", AppName: "app-a", Namespace: "ns1"},
		{StepType: "gitops-sync", AppName: "app-a", Namespace: "ns1"},
	}
	errors := ValidateDeployStepExclusion(steps)
	if len(errors) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errors), errors)
	}
	if !strings.Contains(errors[0], "§12.1") {
		t.Errorf("error should reference §12.1: %s", errors[0])
	}
}

func TestValidateDeployStepExclusion_DifferentNamespace(t *testing.T) {
	steps := []StepDeployTarget{
		{StepType: "deploy-argocd-sync", AppName: "app-a", Namespace: "ns1"},
		{StepType: "gitops-sync", AppName: "app-a", Namespace: "ns2"},
	}
	errors := ValidateDeployStepExclusion(steps)
	if len(errors) != 0 {
		t.Errorf("different namespace should not conflict, got %v", errors)
	}
}

func TestValidateDeployStepExclusion_SameTypeNoConflict(t *testing.T) {
	steps := []StepDeployTarget{
		{StepType: "deploy-argocd-sync", AppName: "app-a", Namespace: "ns1"},
		{StepType: "deploy-argocd-sync", AppName: "app-b", Namespace: "ns1"},
	}
	errors := ValidateDeployStepExclusion(steps)
	if len(errors) != 0 {
		t.Errorf("same type should not conflict, got %v", errors)
	}
}

// ---------------------------------------------------------------------------
// NewGitOpsService
// ---------------------------------------------------------------------------

func TestNewGitOpsService(t *testing.T) {
	svc := NewGitOpsService(nil)
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
}

// ---------------------------------------------------------------------------
// GitOpsApp model
// ---------------------------------------------------------------------------

func TestGitOpsAppTableName(t *testing.T) {
	app := models.GitOpsApp{}
	if app.TableName() != "gitops_apps" {
		t.Errorf("expected gitops_apps, got %s", app.TableName())
	}
}

func TestGitOpsConstants(t *testing.T) {
	if models.GitOpsSourceNative != "native" {
		t.Error("GitOpsSourceNative mismatch")
	}
	if models.GitOpsSourceArgoCD != "argocd" {
		t.Error("GitOpsSourceArgoCD mismatch")
	}
	if models.GitOpsRenderRaw != "raw" {
		t.Error("GitOpsRenderRaw mismatch")
	}
	if models.GitOpsRenderKustomize != "kustomize" {
		t.Error("GitOpsRenderKustomize mismatch")
	}
	if models.GitOpsRenderHelm != "helm" {
		t.Error("GitOpsRenderHelm mismatch")
	}
	if models.GitOpsSyncPolicyAuto != "auto" {
		t.Error("GitOpsSyncPolicyAuto mismatch")
	}
	if models.GitOpsSyncPolicyManual != "manual" {
		t.Error("GitOpsSyncPolicyManual mismatch")
	}
}
