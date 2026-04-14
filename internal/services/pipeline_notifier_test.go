package services

import (
	"strings"
	"testing"
	"time"

	"github.com/shaia/Synapse/internal/models"
)

func TestFormatEventTitle(t *testing.T) {
	tests := []struct {
		eventType string
		pipeline  string
		want      string
	}{
		{"run_success", "deploy-api", "[Synapse] Pipeline `deploy-api` succeeded"},
		{"run_failed", "build-web", "[Synapse] Pipeline `build-web` failed"},
		{"scan_critical", "scan-images", "[Synapse] Pipeline `scan-images` scan found critical vulnerabilities"},
		{"run_cancelled", "my-pipe", "[Synapse] Pipeline `my-pipe` — run_cancelled"},
	}

	for _, tt := range tests {
		t.Run(tt.eventType, func(t *testing.T) {
			event := &PipelineEvent{Type: tt.eventType, PipelineName: tt.pipeline}
			got := formatEventTitle(event)
			if got != tt.want {
				t.Errorf("formatEventTitle() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatEventBody(t *testing.T) {
	event := &PipelineEvent{
		RunID:       42,
		ClusterName: "prod-cluster",
		Namespace:   "default",
		TriggerType: "webhook",
		Duration:    90 * time.Second,
	}
	got := formatEventBody(event)
	if got == "" {
		t.Fatal("expected non-empty body")
	}

	// Check key fields are present
	for _, want := range []string{"Run #42", "prod-cluster", "default", "webhook", "1m30s"} {
		if !strings.Contains(got, want) {
			t.Errorf("body %q missing expected substring %q", got, want)
		}
	}

	// With error
	event.Error = "image pull failed"
	got = formatEventBody(event)
	if !strings.Contains(got, "image pull failed") {
		t.Error("expected error message in body")
	}
}

func TestFormatEventBody_NoError(t *testing.T) {
	event := &PipelineEvent{
		RunID:       1,
		ClusterName: "test",
		Namespace:   "ns",
		TriggerType: "manual",
		Duration:    5 * time.Second,
	}
	got := formatEventBody(event)
	if strings.Contains(got, "Error:") {
		t.Error("expected no Error line when event.Error is empty")
	}
}

func TestResolveChannelIDs(t *testing.T) {
	n := &PipelineNotifier{}

	tests := []struct {
		name      string
		pipeline  *models.Pipeline
		eventType string
		wantLen   int
	}{
		{
			name: "run_success with IDs",
			pipeline: &models.Pipeline{
				NotifyOnSuccess: "[1,2,3]",
			},
			eventType: "run_success",
			wantLen:   3,
		},
		{
			name: "run_failed with IDs",
			pipeline: &models.Pipeline{
				NotifyOnFailure: "[10]",
			},
			eventType: "run_failed",
			wantLen:   1,
		},
		{
			name: "scan_critical with IDs",
			pipeline: &models.Pipeline{
				NotifyOnScan: "[5,6]",
			},
			eventType: "scan_critical",
			wantLen:   2,
		},
		{
			name:      "empty string",
			pipeline:  &models.Pipeline{NotifyOnSuccess: ""},
			eventType: "run_success",
			wantLen:   0,
		},
		{
			name:      "null string",
			pipeline:  &models.Pipeline{NotifyOnSuccess: "null"},
			eventType: "run_success",
			wantLen:   0,
		},
		{
			name:      "empty array",
			pipeline:  &models.Pipeline{NotifyOnSuccess: "[]"},
			eventType: "run_success",
			wantLen:   0,
		},
		{
			name:      "unknown event type",
			pipeline:  &models.Pipeline{NotifyOnSuccess: "[1]"},
			eventType: "run_cancelled",
			wantLen:   0,
		},
		{
			name:      "invalid JSON",
			pipeline:  &models.Pipeline{NotifyOnSuccess: "not-json"},
			eventType: "run_success",
			wantLen:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ids := n.resolveChannelIDs(tt.pipeline, tt.eventType)
			if len(ids) != tt.wantLen {
				t.Errorf("resolveChannelIDs() returned %d IDs, want %d", len(ids), tt.wantLen)
			}
		})
	}
}

func TestFormatPayload_Slack(t *testing.T) {
	n := &PipelineNotifier{}
	event := &PipelineEvent{
		Type:         "run_success",
		PipelineName: "deploy",
		RunID:        1,
		ClusterName:  "prod",
		Namespace:    "default",
		TriggerType:  "manual",
		Duration:     30 * time.Second,
	}

	payload := n.formatPayload("slack", event)
	text, ok := payload["text"].(string)
	if !ok {
		t.Fatal("expected text field in slack payload")
	}
	if !strings.Contains(text, "deploy") {
		t.Error("slack payload should contain pipeline name")
	}
	if !strings.Contains(text, "succeeded") {
		t.Error("slack payload should contain event description")
	}
}

func TestFormatPayload_Telegram(t *testing.T) {
	n := &PipelineNotifier{}
	event := &PipelineEvent{
		Type:         "run_failed",
		PipelineName: "build",
		RunID:        5,
		ClusterName:  "staging",
		Namespace:    "ci",
		TriggerType:  "webhook",
		Duration:     60 * time.Second,
		Error:        "build timeout",
	}

	payload := n.formatPayload("telegram", event)
	if payload["parse_mode"] != "Markdown" {
		t.Error("telegram payload should have Markdown parse_mode")
	}
	text, ok := payload["text"].(string)
	if !ok {
		t.Fatal("expected text field")
	}
	if !strings.Contains(text, "build timeout") {
		t.Error("telegram payload should contain error")
	}
}

func TestFormatPayload_Teams(t *testing.T) {
	n := &PipelineNotifier{}
	event := &PipelineEvent{
		Type:         "scan_critical",
		PipelineName: "security-scan",
		RunID:        10,
		ClusterName:  "prod",
		Namespace:    "security",
		TriggerType:  "schedule",
		Duration:     120 * time.Second,
	}

	payload := n.formatPayload("teams", event)
	if payload["type"] != "message" {
		t.Error("teams payload should have type=message")
	}
	attachments, ok := payload["attachments"].([]map[string]interface{})
	if !ok || len(attachments) == 0 {
		t.Fatal("teams payload should have attachments")
	}
	if attachments[0]["contentType"] != "application/vnd.microsoft.card.adaptive" {
		t.Error("teams attachment should be adaptive card")
	}
}

func TestFormatPayload_GenericWebhook(t *testing.T) {
	n := &PipelineNotifier{}
	event := &PipelineEvent{
		Type:         "run_failed",
		PipelineName: "deploy",
		RunID:        7,
		ClusterName:  "dev",
		Namespace:    "app",
		TriggerType:  "manual",
		Duration:     45 * time.Second,
		Error:        "OOM killed",
	}

	payload := n.formatPayload("custom", event)
	if payload["event"] != "run_failed" {
		t.Error("generic webhook should include event type")
	}
	if payload["pipeline"] != "deploy" {
		t.Error("generic webhook should include pipeline name")
	}
	if payload["run_id"] != uint(7) {
		t.Errorf("generic webhook should include run_id, got %v", payload["run_id"])
	}
	if payload["error"] != "OOM killed" {
		t.Error("generic webhook should include error")
	}
}

func TestNewPipelineNotifier(t *testing.T) {
	dedup := NewNotifyDedup(5 * time.Minute)
	defer dedup.Stop()

	notifier := NewPipelineNotifier(nil, dedup)
	if notifier == nil {
		t.Fatal("expected non-nil PipelineNotifier")
	}
	if notifier.dedup != dedup {
		t.Error("expected dedup to be set")
	}
	if notifier.client == nil {
		t.Error("expected http client to be set")
	}
}

