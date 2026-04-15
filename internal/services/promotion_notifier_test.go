package services

import (
	"testing"
)

// ---------------------------------------------------------------------------
// parseGateChannelIDs
// ---------------------------------------------------------------------------

func TestParseGateChannelIDs_Valid(t *testing.T) {
	ids := parseGateChannelIDs("[1,5,10]")
	if len(ids) != 3 {
		t.Fatalf("expected 3 IDs, got %d", len(ids))
	}
	if ids[0] != 1 || ids[1] != 5 || ids[2] != 10 {
		t.Errorf("expected [1,5,10], got %v", ids)
	}
}

func TestParseGateChannelIDs_Empty(t *testing.T) {
	for _, raw := range []string{"", "null", "[]"} {
		ids := parseGateChannelIDs(raw)
		if ids != nil {
			t.Errorf("parseGateChannelIDs(%q) = %v, want nil", raw, ids)
		}
	}
}

func TestParseGateChannelIDs_Invalid(t *testing.T) {
	ids := parseGateChannelIDs("invalid-json")
	if ids != nil {
		t.Errorf("expected nil for invalid JSON, got %v", ids)
	}
}

// ---------------------------------------------------------------------------
// formatGatePayload
// ---------------------------------------------------------------------------

func TestFormatGatePayload_Slack(t *testing.T) {
	event := &GateEvent{
		PipelineID:      1,
		PipelineName:    "deploy-api",
		PipelineRunID:   42,
		FromEnvironment: "staging",
		ToEnvironment:   "production",
		RequesterName:   "alice",
		ApprovalID:      100,
		Reason:          "promotion to production requires approval",
	}

	payload := formatGatePayload("slack", event)
	text, ok := payload["text"].(string)
	if !ok {
		t.Fatal("expected text field")
	}
	if !containsSubstr(text, "Production Gate") {
		t.Error("expected 'Production Gate' in text")
	}
	if !containsSubstr(text, "staging → production") {
		t.Error("expected 'staging → production' in text")
	}
	if !containsSubstr(text, "deploy-api") {
		t.Error("expected pipeline name in text")
	}
	if !containsSubstr(text, "alice") {
		t.Error("expected requester name in text")
	}
}

func TestFormatGatePayload_Telegram(t *testing.T) {
	event := &GateEvent{PipelineID: 1, PipelineName: "test", FromEnvironment: "dev", ToEnvironment: "prod"}
	payload := formatGatePayload("telegram", event)
	if payload["parse_mode"] != "Markdown" {
		t.Error("expected Markdown parse_mode for Telegram")
	}
}

func TestFormatGatePayload_Teams(t *testing.T) {
	event := &GateEvent{PipelineID: 1, PipelineName: "test", FromEnvironment: "dev", ToEnvironment: "prod"}
	payload := formatGatePayload("teams", event)
	if payload["type"] != "message" {
		t.Error("expected message type for Teams")
	}
	attachments, ok := payload["attachments"].([]map[string]interface{})
	if !ok || len(attachments) == 0 {
		t.Error("expected attachments for Teams")
	}
}

func TestFormatGatePayload_Webhook(t *testing.T) {
	event := &GateEvent{
		PipelineID:      5,
		PipelineName:    "backend-deploy",
		PipelineRunID:   99,
		FromEnvironment: "staging",
		ToEnvironment:   "production",
		RequesterName:   "bob",
		ApprovalID:      200,
		Reason:          "requires approval",
	}

	payload := formatGatePayload("webhook", event)
	if payload["event"] != "production_gate" {
		t.Errorf("expected event=production_gate, got %v", payload["event"])
	}
	if payload["pipeline_name"] != "backend-deploy" {
		t.Errorf("expected pipeline_name, got %v", payload["pipeline_name"])
	}
	if payload["from_environment"] != "staging" {
		t.Errorf("expected from_environment=staging, got %v", payload["from_environment"])
	}
	if payload["to_environment"] != "production" {
		t.Errorf("expected to_environment=production, got %v", payload["to_environment"])
	}
	if payload["approval_id"] != uint(200) {
		t.Errorf("expected approval_id=200, got %v", payload["approval_id"])
	}
}

// ---------------------------------------------------------------------------
// formatGateBody
// ---------------------------------------------------------------------------

func TestFormatGateBody_Full(t *testing.T) {
	event := &GateEvent{
		PipelineName:    "api-deploy",
		PipelineRunID:   42,
		FromEnvironment: "staging",
		ToEnvironment:   "production",
		RequesterName:   "charlie",
		ApprovalID:      150,
		Reason:          "production requires approval",
	}

	body := formatGateBody(event)
	checks := []string{
		"api-deploy",
		"Run #42",
		"staging → production",
		"charlie",
		"150",
		"production requires approval",
		"Action required",
	}
	for _, check := range checks {
		if !containsSubstr(body, check) {
			t.Errorf("expected %q in body, got:\n%s", check, body)
		}
	}
}

func TestFormatGateBody_NoRequester(t *testing.T) {
	event := &GateEvent{
		PipelineName:    "deploy",
		PipelineRunID:   1,
		FromEnvironment: "dev",
		ToEnvironment:   "staging",
		ApprovalID:      1,
	}

	body := formatGateBody(event)
	if containsSubstr(body, "Requested by:") {
		t.Error("should not contain 'Requested by:' when no requester")
	}
}

func TestFormatGateBody_NoReason(t *testing.T) {
	event := &GateEvent{
		PipelineName:    "deploy",
		PipelineRunID:   1,
		FromEnvironment: "dev",
		ToEnvironment:   "staging",
		ApprovalID:      1,
	}

	body := formatGateBody(event)
	if containsSubstr(body, "Reason:") {
		t.Error("should not contain 'Reason:' when no reason")
	}
}

// ---------------------------------------------------------------------------
// GateEvent struct
// ---------------------------------------------------------------------------

func TestGateEvent_Fields(t *testing.T) {
	event := GateEvent{
		PipelineID:      10,
		PipelineName:    "my-pipeline",
		PipelineRunID:   55,
		FromEnvironment: "staging",
		ToEnvironment:   "production",
		RequesterName:   "admin",
		ApprovalID:      99,
		Reason:          "policy",
	}

	if event.PipelineID != 10 || event.PipelineName != "my-pipeline" {
		t.Error("field mismatch")
	}
	if event.ApprovalID != 99 || event.Reason != "policy" {
		t.Error("field mismatch")
	}
}
