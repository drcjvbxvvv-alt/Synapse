package services

import (
	"testing"
)

// ---------------------------------------------------------------------------
// GitHub parser
// ---------------------------------------------------------------------------

func TestGitHubParser_ParsePushEvent(t *testing.T) {
	payload := []byte(`{
		"ref": "refs/heads/main",
		"repository": {"full_name": "company/backend"},
		"commits": [
			{"added": ["src/new.go"], "modified": ["src/main.go"], "removed": []},
			{"added": [], "modified": ["README.md"], "removed": ["old.txt"]}
		]
	}`)

	p := &GitHubPayloadParser{}
	event, err := p.ParsePushEvent(payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event.Provider != "github" {
		t.Errorf("expected github, got %s", event.Provider)
	}
	if event.Repo != "company/backend" {
		t.Errorf("expected company/backend, got %s", event.Repo)
	}
	if event.Branch != "main" {
		t.Errorf("expected main, got %s", event.Branch)
	}
	if event.EventType != "push" {
		t.Errorf("expected push, got %s", event.EventType)
	}
	if len(event.ChangedFiles) != 4 {
		t.Errorf("expected 4 changed files, got %d: %v", len(event.ChangedFiles), event.ChangedFiles)
	}
}

func TestGitHubParser_ParsePushEvent_TagPush(t *testing.T) {
	payload := []byte(`{
		"ref": "refs/tags/v1.0.0",
		"repository": {"full_name": "company/backend"},
		"commits": []
	}`)

	p := &GitHubPayloadParser{}
	event, err := p.ParsePushEvent(payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event.EventType != "tag_push" {
		t.Errorf("expected tag_push, got %s", event.EventType)
	}
}

func TestGitHubParser_ParseMergeRequestEvent(t *testing.T) {
	payload := []byte(`{
		"action": "opened",
		"pull_request": {"head": {"ref": "feature/login"}},
		"repository": {"full_name": "company/backend"}
	}`)

	p := &GitHubPayloadParser{}
	event, err := p.ParseMergeRequestEvent(payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event.EventType != "pull_request" {
		t.Errorf("expected pull_request, got %s", event.EventType)
	}
	if event.Branch != "feature/login" {
		t.Errorf("expected feature/login, got %s", event.Branch)
	}
}

func TestGitHubParser_DetectEventType(t *testing.T) {
	p := &GitHubPayloadParser{}
	if p.DetectEventType("push") != "push" {
		t.Error("expected push")
	}
	if p.DetectEventType("pull_request") != "pull_request" {
		t.Error("expected pull_request")
	}
}

// ---------------------------------------------------------------------------
// GitLab parser
// ---------------------------------------------------------------------------

func TestGitLabParser_ParsePushEvent(t *testing.T) {
	payload := []byte(`{
		"ref": "refs/heads/develop",
		"project": {"path_with_namespace": "team/api-service"},
		"commits": [
			{"added": ["file1.go"], "modified": [], "removed": []}
		]
	}`)

	p := &GitLabPayloadParser{}
	event, err := p.ParsePushEvent(payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event.Provider != "gitlab" {
		t.Errorf("expected gitlab, got %s", event.Provider)
	}
	if event.Repo != "team/api-service" {
		t.Errorf("expected team/api-service, got %s", event.Repo)
	}
	if event.Branch != "develop" {
		t.Errorf("expected develop, got %s", event.Branch)
	}
	if len(event.ChangedFiles) != 1 {
		t.Errorf("expected 1 changed file, got %d", len(event.ChangedFiles))
	}
}

func TestGitLabParser_ParseMergeRequestEvent(t *testing.T) {
	payload := []byte(`{
		"object_attributes": {"source_branch": "feature/auth", "action": "open"},
		"project": {"path_with_namespace": "team/api-service"}
	}`)

	p := &GitLabPayloadParser{}
	event, err := p.ParseMergeRequestEvent(payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event.EventType != "merge_request" {
		t.Errorf("expected merge_request, got %s", event.EventType)
	}
	if event.Branch != "feature/auth" {
		t.Errorf("expected feature/auth, got %s", event.Branch)
	}
}

func TestGitLabParser_DetectEventType(t *testing.T) {
	p := &GitLabPayloadParser{}
	if p.DetectEventType("Push Hook") != "push" {
		t.Error("expected push")
	}
	if p.DetectEventType("Merge Request Hook") != "merge_request" {
		t.Error("expected merge_request")
	}
	if p.DetectEventType("Tag Push Hook") != "tag_push" {
		t.Error("expected tag_push")
	}
}

// ---------------------------------------------------------------------------
// Gitea parser
// ---------------------------------------------------------------------------

func TestGiteaParser_ParsePushEvent(t *testing.T) {
	payload := []byte(`{
		"ref": "refs/heads/main",
		"repository": {"full_name": "org/repo"},
		"commits": [
			{"added": ["a.go"], "modified": ["b.go"], "removed": ["c.go"]}
		]
	}`)

	p := &GiteaPayloadParser{}
	event, err := p.ParsePushEvent(payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event.Provider != "gitea" {
		t.Errorf("expected gitea, got %s", event.Provider)
	}
	if event.Repo != "org/repo" {
		t.Errorf("expected org/repo, got %s", event.Repo)
	}
	if len(event.ChangedFiles) != 3 {
		t.Errorf("expected 3 changed files, got %d", len(event.ChangedFiles))
	}
}

func TestGiteaParser_ParseMergeRequestEvent(t *testing.T) {
	payload := []byte(`{
		"action": "opened",
		"pull_request": {"head": {"ref": "fix/bug"}},
		"repository": {"full_name": "org/repo"}
	}`)

	p := &GiteaPayloadParser{}
	event, err := p.ParseMergeRequestEvent(payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event.Branch != "fix/bug" {
		t.Errorf("expected fix/bug, got %s", event.Branch)
	}
}

// ---------------------------------------------------------------------------
// NewWebhookPayloadParser factory
// ---------------------------------------------------------------------------

func TestNewWebhookPayloadParser_Valid(t *testing.T) {
	for _, pt := range []string{"github", "gitlab", "gitea"} {
		parser, err := NewWebhookPayloadParser(pt)
		if err != nil {
			t.Errorf("unexpected error for %s: %v", pt, err)
		}
		if parser == nil {
			t.Errorf("expected non-nil parser for %s", pt)
		}
	}
}

func TestNewWebhookPayloadParser_Invalid(t *testing.T) {
	_, err := NewWebhookPayloadParser("bitbucket")
	if err == nil {
		t.Error("expected error for unsupported provider")
	}
}

// ---------------------------------------------------------------------------
// Dedup helper
// ---------------------------------------------------------------------------

func TestDedup(t *testing.T) {
	result := dedup([]string{"a", "b", "a", "c", "b"})
	if len(result) != 3 {
		t.Errorf("expected 3 unique items, got %d: %v", len(result), result)
	}
}

func TestDedup_Empty(t *testing.T) {
	result := dedup(nil)
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

// ---------------------------------------------------------------------------
// Invalid JSON
// ---------------------------------------------------------------------------

func TestGitHubParser_ParsePushEvent_InvalidJSON(t *testing.T) {
	p := &GitHubPayloadParser{}
	_, err := p.ParsePushEvent([]byte("not-json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestGitLabParser_ParsePushEvent_InvalidJSON(t *testing.T) {
	p := &GitLabPayloadParser{}
	_, err := p.ParsePushEvent([]byte("not-json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

// ---------------------------------------------------------------------------
// GitProviderService helpers
// ---------------------------------------------------------------------------

func TestValidateProviderType(t *testing.T) {
	for _, pt := range []string{"github", "gitlab", "gitea"} {
		if err := validateProviderType(pt); err != nil {
			t.Errorf("expected valid for %s: %v", pt, err)
		}
	}
	if err := validateProviderType("bitbucket"); err == nil {
		t.Error("expected error for bitbucket")
	}
}

func TestGenerateWebhookToken(t *testing.T) {
	token, err := generateWebhookToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(token) != 64 { // 32 bytes = 64 hex chars
		t.Errorf("expected 64 char token, got %d", len(token))
	}

	// Ensure uniqueness
	token2, _ := generateWebhookToken()
	if token == token2 {
		t.Error("expected unique tokens")
	}
}
