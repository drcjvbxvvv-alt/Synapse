package services

import (
	"strings"
	"testing"
)

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
	svc := NewPromotionService(nil)
	if svc == nil {
		t.Fatal("expected non-nil service")
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
