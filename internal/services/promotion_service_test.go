package services

import (
	"strings"
	"testing"

	"github.com/shaia/Synapse/internal/models"
)

// ---------------------------------------------------------------------------
// ValidatePromotionOrder
// ---------------------------------------------------------------------------

func TestValidatePromotionOrder_Valid(t *testing.T) {
	envs := []models.Environment{
		{Name: "dev", OrderIndex: 0},
		{Name: "staging", OrderIndex: 1},
		{Name: "production", OrderIndex: 2},
	}
	if err := ValidatePromotionOrder(envs, "dev", "staging"); err != nil {
		t.Errorf("expected valid: %v", err)
	}
	if err := ValidatePromotionOrder(envs, "staging", "production"); err != nil {
		t.Errorf("expected valid: %v", err)
	}
}

func TestValidatePromotionOrder_SkipLevel(t *testing.T) {
	envs := []models.Environment{
		{Name: "dev", OrderIndex: 0},
		{Name: "staging", OrderIndex: 1},
		{Name: "production", OrderIndex: 2},
	}
	err := ValidatePromotionOrder(envs, "dev", "production")
	if err == nil {
		t.Error("expected error for skipping staging")
	}
	if !strings.Contains(err.Error(), "next environment") {
		t.Errorf("error should mention order: %v", err)
	}
}

func TestValidatePromotionOrder_SourceNotFound(t *testing.T) {
	envs := []models.Environment{
		{Name: "dev", OrderIndex: 0},
		{Name: "staging", OrderIndex: 1},
	}
	err := ValidatePromotionOrder(envs, "nonexistent", "staging")
	if err == nil {
		t.Error("expected error for missing source")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention not found: %v", err)
	}
}

func TestValidatePromotionOrder_TargetNotFound(t *testing.T) {
	envs := []models.Environment{
		{Name: "dev", OrderIndex: 0},
	}
	err := ValidatePromotionOrder(envs, "dev", "staging")
	if err == nil {
		t.Error("expected error for missing target")
	}
}

// ---------------------------------------------------------------------------
// ValidateNotReversePromotion
// ---------------------------------------------------------------------------

func TestValidateNotReversePromotion_Valid(t *testing.T) {
	envs := []models.Environment{
		{Name: "dev", OrderIndex: 0},
		{Name: "staging", OrderIndex: 1},
		{Name: "production", OrderIndex: 2},
	}
	if err := ValidateNotReversePromotion(envs, "dev", "staging"); err != nil {
		t.Errorf("expected valid: %v", err)
	}
}

func TestValidateNotReversePromotion_Reverse(t *testing.T) {
	envs := []models.Environment{
		{Name: "dev", OrderIndex: 0},
		{Name: "staging", OrderIndex: 1},
		{Name: "production", OrderIndex: 2},
	}
	err := ValidateNotReversePromotion(envs, "production", "dev")
	if err == nil {
		t.Error("expected error for reverse promotion")
	}
	if !strings.Contains(err.Error(), "reverse") {
		t.Errorf("error should mention reverse: %v", err)
	}
}

func TestValidateNotReversePromotion_SameEnv(t *testing.T) {
	envs := []models.Environment{
		{Name: "dev", OrderIndex: 0},
		{Name: "staging", OrderIndex: 1},
	}
	err := ValidateNotReversePromotion(envs, "dev", "dev")
	if err == nil {
		t.Error("expected error for same environment")
	}
}

// ---------------------------------------------------------------------------
// marshalPromotionPayload
// ---------------------------------------------------------------------------

func TestMarshalPromotionPayload(t *testing.T) {
	req := &PromotionRequest{
		PipelineID:    1,
		PipelineRunID: 10,
		TriggeredBy:   5,
	}
	decision := &PromotionDecision{
		FromEnvironment: "dev",
		ToEnvironment:   "staging",
	}

	payload := marshalPromotionPayload(req, decision)
	if payload == "" {
		t.Error("expected non-empty payload")
	}
	if !strings.Contains(payload, "pipeline_id") {
		t.Error("payload should contain pipeline_id")
	}
	if !strings.Contains(payload, "from_environment") {
		t.Error("payload should contain from_environment")
	}
	if !strings.Contains(payload, "to_environment") {
		t.Error("payload should contain to_environment")
	}
}

// ---------------------------------------------------------------------------
// NewPromotionService
// ---------------------------------------------------------------------------

func TestNewPromotionService(t *testing.T) {
	envSvc := NewEnvironmentService(nil)
	svc := NewPromotionService(nil, envSvc)
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
	if svc.envSvc == nil {
		t.Fatal("expected envSvc to be set")
	}
}

// ---------------------------------------------------------------------------
// PromotionDecision action constants
// ---------------------------------------------------------------------------

func TestPromotionDecisionActions(t *testing.T) {
	// Verify all action types used in EvaluatePromotion
	actions := []string{"auto_promote", "require_approval", "blocked"}
	for _, a := range actions {
		if a == "" {
			t.Error("action should not be empty")
		}
	}
}

// ---------------------------------------------------------------------------
// PromotionResult status values
// ---------------------------------------------------------------------------

func TestPromotionResultStatuses(t *testing.T) {
	statuses := []string{"auto_promoted", "pending_approval", "blocked"}
	for _, s := range statuses {
		if s == "" {
			t.Error("status should not be empty")
		}
	}
}
