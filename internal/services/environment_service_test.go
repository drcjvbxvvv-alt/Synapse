package services

import (
	"strings"
	"testing"

	"github.com/shaia/Synapse/internal/models"
)

// ---------------------------------------------------------------------------
// validateEnvironment
// ---------------------------------------------------------------------------

func TestValidateEnvironment_Valid(t *testing.T) {
	env := &models.Environment{
		Name:       "dev",
		PipelineID: 1,
		ClusterID:  1,
		Namespace:  "app-dev",
		OrderIndex: 0,
	}
	if err := validateEnvironment(env); err != nil {
		t.Errorf("expected valid: %v", err)
	}
}

func TestValidateEnvironment_WithApproverIDs(t *testing.T) {
	env := &models.Environment{
		Name:        "production",
		PipelineID:  1,
		ClusterID:   2,
		Namespace:   "app-prod",
		OrderIndex:  2,
		ApproverIDs: "[1, 2, 3]",
	}
	if err := validateEnvironment(env); err != nil {
		t.Errorf("expected valid: %v", err)
	}
}

func TestValidateEnvironment_MissingName(t *testing.T) {
	env := &models.Environment{
		PipelineID: 1,
		ClusterID:  1,
		Namespace:  "ns",
		OrderIndex: 0,
	}
	err := validateEnvironment(env)
	if err == nil {
		t.Error("expected error for missing name")
	}
	if !strings.Contains(err.Error(), "name") {
		t.Errorf("error should mention name: %v", err)
	}
}

func TestValidateEnvironment_MissingPipelineID(t *testing.T) {
	env := &models.Environment{
		Name:       "dev",
		ClusterID:  1,
		Namespace:  "ns",
		OrderIndex: 0,
	}
	err := validateEnvironment(env)
	if err == nil {
		t.Error("expected error for missing pipeline_id")
	}
}

func TestValidateEnvironment_MissingClusterID(t *testing.T) {
	env := &models.Environment{
		Name:       "dev",
		PipelineID: 1,
		Namespace:  "ns",
		OrderIndex: 0,
	}
	err := validateEnvironment(env)
	if err == nil {
		t.Error("expected error for missing cluster_id")
	}
}

func TestValidateEnvironment_MissingNamespace(t *testing.T) {
	env := &models.Environment{
		Name:       "dev",
		PipelineID: 1,
		ClusterID:  1,
		OrderIndex: 0,
	}
	err := validateEnvironment(env)
	if err == nil {
		t.Error("expected error for missing namespace")
	}
}

func TestValidateEnvironment_NegativeOrderIndex(t *testing.T) {
	env := &models.Environment{
		Name:       "dev",
		PipelineID: 1,
		ClusterID:  1,
		Namespace:  "ns",
		OrderIndex: -1,
	}
	err := validateEnvironment(env)
	if err == nil {
		t.Error("expected error for negative order_index")
	}
}

func TestValidateEnvironment_InvalidApproverIDs(t *testing.T) {
	env := &models.Environment{
		Name:        "staging",
		PipelineID:  1,
		ClusterID:   1,
		Namespace:   "ns",
		OrderIndex:  1,
		ApproverIDs: "not-json",
	}
	err := validateEnvironment(env)
	if err == nil {
		t.Error("expected error for invalid approver_ids JSON")
	}
	if !strings.Contains(err.Error(), "approver_ids") {
		t.Errorf("error should mention approver_ids: %v", err)
	}
}

// ---------------------------------------------------------------------------
// NewEnvironmentService
// ---------------------------------------------------------------------------

func TestNewEnvironmentService(t *testing.T) {
	svc := NewEnvironmentService(nil)
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
}

// ---------------------------------------------------------------------------
// PromotionHistory status constants
// ---------------------------------------------------------------------------

func TestPromotionStatusConstants(t *testing.T) {
	expected := map[string]string{
		"pending":       models.PromotionStatusPending,
		"approved":      models.PromotionStatusApproved,
		"rejected":      models.PromotionStatusRejected,
		"auto_promoted": models.PromotionStatusAutoPromoted,
	}
	for want, got := range expected {
		if got != want {
			t.Errorf("expected %q, got %q", want, got)
		}
	}
}

// ---------------------------------------------------------------------------
// Environment model TableName
// ---------------------------------------------------------------------------

func TestEnvironmentTableName(t *testing.T) {
	env := models.Environment{}
	if env.TableName() != "environments" {
		t.Errorf("expected environments, got %s", env.TableName())
	}
}

func TestPromotionHistoryTableName(t *testing.T) {
	ph := models.PromotionHistory{}
	if ph.TableName() != "promotion_history" {
		t.Errorf("expected promotion_history, got %s", ph.TableName())
	}
}

// ---------------------------------------------------------------------------
// Registry model TableName
// ---------------------------------------------------------------------------

func TestRegistryTableName(t *testing.T) {
	r := models.Registry{}
	if r.TableName() != "registries" {
		t.Errorf("expected registries, got %s", r.TableName())
	}
}

// ---------------------------------------------------------------------------
// Registry type constants
// ---------------------------------------------------------------------------

func TestRegistryTypeConstants(t *testing.T) {
	if models.RegistryTypeHarbor != "harbor" {
		t.Error("RegistryTypeHarbor mismatch")
	}
	if models.RegistryTypeDockerHub != "dockerhub" {
		t.Error("RegistryTypeDockerHub mismatch")
	}
	if models.RegistryTypeACR != "acr" {
		t.Error("RegistryTypeACR mismatch")
	}
	if models.RegistryTypeECR != "ecr" {
		t.Error("RegistryTypeECR mismatch")
	}
	if models.RegistryTypeGCR != "gcr" {
		t.Error("RegistryTypeGCR mismatch")
	}
}
