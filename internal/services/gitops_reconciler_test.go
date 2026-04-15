package services

import (
	"testing"
	"time"

	"github.com/shaia/Synapse/internal/models"
)

// ---------------------------------------------------------------------------
// parseNotifyChannelIDs
// ---------------------------------------------------------------------------

func TestParseNotifyChannelIDs_Valid(t *testing.T) {
	ids := parseNotifyChannelIDs("[1,2,3]")
	if len(ids) != 3 {
		t.Fatalf("expected 3 IDs, got %d", len(ids))
	}
	if ids[0] != 1 || ids[1] != 2 || ids[2] != 3 {
		t.Errorf("expected [1,2,3], got %v", ids)
	}
}

func TestParseNotifyChannelIDs_Empty(t *testing.T) {
	tests := []string{"", "null", "[]"}
	for _, raw := range tests {
		ids := parseNotifyChannelIDs(raw)
		if ids != nil {
			t.Errorf("parseNotifyChannelIDs(%q) = %v, want nil", raw, ids)
		}
	}
}

func TestParseNotifyChannelIDs_Invalid(t *testing.T) {
	ids := parseNotifyChannelIDs("not-json")
	if ids != nil {
		t.Errorf("expected nil for invalid JSON, got %v", ids)
	}
}

func TestParseNotifyChannelIDs_Single(t *testing.T) {
	ids := parseNotifyChannelIDs("[42]")
	if len(ids) != 1 || ids[0] != 42 {
		t.Errorf("expected [42], got %v", ids)
	}
}

// ---------------------------------------------------------------------------
// formatDriftPayload
// ---------------------------------------------------------------------------

func TestFormatDriftPayload_Slack(t *testing.T) {
	event := &DriftEvent{
		AppID:       1,
		AppName:     "my-app",
		Namespace:   "production",
		DiffSummary: "2 drifted, 1 in sync",
		RepoURL:     "https://github.com/org/repo",
		Branch:      "main",
	}

	payload := formatDriftPayload("slack", event)
	text, ok := payload["text"].(string)
	if !ok {
		t.Fatal("expected text field")
	}
	if !containsStr(text, "Drift Detected") {
		t.Error("expected 'Drift Detected' in text")
	}
	if !containsStr(text, "my-app") {
		t.Error("expected app name in text")
	}
}

func TestFormatDriftPayload_Telegram(t *testing.T) {
	event := &DriftEvent{AppID: 1, AppName: "test"}
	payload := formatDriftPayload("telegram", event)
	if payload["parse_mode"] != "Markdown" {
		t.Error("expected Markdown parse_mode for Telegram")
	}
}

func TestFormatDriftPayload_Teams(t *testing.T) {
	event := &DriftEvent{AppID: 1, AppName: "test"}
	payload := formatDriftPayload("teams", event)
	if payload["type"] != "message" {
		t.Error("expected message type for Teams")
	}
	attachments, ok := payload["attachments"].([]map[string]interface{})
	if !ok || len(attachments) == 0 {
		t.Error("expected attachments for Teams")
	}
}

func TestFormatDriftPayload_Webhook(t *testing.T) {
	event := &DriftEvent{
		AppID:       5,
		AppName:     "backend",
		ClusterID:   1,
		Namespace:   "prod",
		DiffSummary: "3 drifted",
		RepoURL:     "https://github.com/org/backend",
		Branch:      "main",
	}

	payload := formatDriftPayload("webhook", event)
	if payload["event"] != "gitops_drift" {
		t.Errorf("expected event=gitops_drift, got %v", payload["event"])
	}
	if payload["app_name"] != "backend" {
		t.Errorf("expected app_name=backend, got %v", payload["app_name"])
	}
	if payload["diff_summary"] != "3 drifted" {
		t.Errorf("expected diff_summary, got %v", payload["diff_summary"])
	}
}

// ---------------------------------------------------------------------------
// formatDriftBody
// ---------------------------------------------------------------------------

func TestFormatDriftBody(t *testing.T) {
	event := &DriftEvent{
		AppID:       1,
		AppName:     "my-app",
		Namespace:   "production",
		DiffSummary: "2 drifted, 1 in sync",
		RepoURL:     "https://github.com/org/repo",
		Branch:      "main",
	}

	body := formatDriftBody(event)
	if !containsStr(body, "my-app") {
		t.Error("expected app name in body")
	}
	if !containsStr(body, "production") {
		t.Error("expected namespace in body")
	}
	if !containsStr(body, "github.com/org/repo") {
		t.Error("expected repo URL in body")
	}
	if !containsStr(body, "2 drifted") {
		t.Error("expected diff summary in body")
	}
}

func TestFormatDriftBody_NoRepo(t *testing.T) {
	event := &DriftEvent{AppID: 1, AppName: "app", Namespace: "ns"}
	body := formatDriftBody(event)
	if containsStr(body, "Repo:") {
		t.Error("should not contain Repo when empty")
	}
}

// ---------------------------------------------------------------------------
// findAppsNeedingReconcile (logic test via time check)
// ---------------------------------------------------------------------------

func TestReconcileNeedCheck_IntervalLogic(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name         string
		lastDiffAt   *time.Time
		syncInterval int
		wantReconcile bool
	}{
		{"never diffed", nil, 300, true},
		{"diffed 10 min ago, interval 5 min", timePtr(now.Add(-10 * time.Minute)), 300, true},
		{"diffed 1 min ago, interval 5 min", timePtr(now.Add(-1 * time.Minute)), 300, false},
		{"diffed 31 sec ago, interval 30 sec", timePtr(now.Add(-31 * time.Second)), 30, true},
		{"interval below minimum uses 30s", timePtr(now.Add(-31 * time.Second)), 10, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := models.GitOpsApp{
				LastDiffAt:   tt.lastDiffAt,
				SyncInterval: tt.syncInterval,
			}

			interval := time.Duration(app.SyncInterval) * time.Second
			if interval < 30*time.Second {
				interval = 30 * time.Second
			}

			needReconcile := app.LastDiffAt == nil || now.Sub(*app.LastDiffAt) >= interval
			if needReconcile != tt.wantReconcile {
				t.Errorf("got needReconcile=%v, want %v", needReconcile, tt.wantReconcile)
			}
		})
	}
}

func timePtr(t time.Time) *time.Time {
	return &t
}

// ---------------------------------------------------------------------------
// DriftEvent
// ---------------------------------------------------------------------------

func TestDriftEvent_Fields(t *testing.T) {
	event := DriftEvent{
		AppID:       10,
		AppName:     "frontend",
		ClusterID:   2,
		Namespace:   "staging",
		DiffSummary: "1 to add",
		RepoURL:     "https://github.com/org/frontend",
		Branch:      "develop",
	}

	if event.AppID != 10 {
		t.Errorf("expected AppID=10")
	}
	if event.AppName != "frontend" {
		t.Errorf("expected AppName=frontend")
	}
	if event.Branch != "develop" {
		t.Errorf("expected Branch=develop")
	}
}
