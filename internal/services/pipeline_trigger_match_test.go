package services

import (
	"testing"
)

// ---------------------------------------------------------------------------
// Branch Glob Matching
// ---------------------------------------------------------------------------

func TestMatchBranchGlob_Exact(t *testing.T) {
	if !matchBranchGlob("main", "main") {
		t.Error("exact match should succeed")
	}
	if matchBranchGlob("main", "develop") {
		t.Error("exact mismatch should fail")
	}
}

func TestMatchBranchGlob_Wildcard(t *testing.T) {
	if !matchBranchGlob("*", "main") {
		t.Error("* should match any branch")
	}
	if !matchBranchGlob("**", "feature/deep/nested") {
		t.Error("** should match any branch")
	}
}

func TestMatchBranchGlob_SingleStar(t *testing.T) {
	if !matchBranchGlob("release/*", "release/v1.0") {
		t.Error("release/* should match release/v1.0")
	}
	// filepath.Match: * does not match path separator
	if matchBranchGlob("release/*", "release/v1/hotfix") {
		t.Error("release/* should NOT match multi-level release/v1/hotfix")
	}
}

func TestMatchBranchGlob_DoubleStar(t *testing.T) {
	if !matchBranchGlob("release/**", "release/v1") {
		t.Error("release/** should match release/v1")
	}
	if !matchBranchGlob("release/**", "release/v1/hotfix") {
		t.Error("release/** should match release/v1/hotfix")
	}
	if matchBranchGlob("release/**", "feature/v1") {
		t.Error("release/** should NOT match feature/v1")
	}
}

func TestMatchBranchGlob_Feature(t *testing.T) {
	if !matchBranchGlob("feature/*", "feature/login") {
		t.Error("feature/* should match feature/login")
	}
	if matchBranchGlob("feature/*", "main") {
		t.Error("feature/* should not match main")
	}
}

// ---------------------------------------------------------------------------
// Event Matching
// ---------------------------------------------------------------------------

func TestMatchEvent_EmptyAllowsAll(t *testing.T) {
	if !matchEvent(nil, "push") {
		t.Error("nil events should allow all")
	}
	if !matchEvent([]string{}, "push") {
		t.Error("empty events should allow all")
	}
}

func TestMatchEvent_Specific(t *testing.T) {
	events := []string{"push", "merge_request"}
	if !matchEvent(events, "push") {
		t.Error("should match push")
	}
	if !matchEvent(events, "merge_request") {
		t.Error("should match merge_request")
	}
	if matchEvent(events, "tag_push") {
		t.Error("should not match tag_push")
	}
}

func TestMatchEvent_CaseInsensitive(t *testing.T) {
	if !matchEvent([]string{"Push"}, "push") {
		t.Error("should match case-insensitively")
	}
}

// ---------------------------------------------------------------------------
// Path Filter Matching
// ---------------------------------------------------------------------------

func TestMatchPathFilter_NoPatterns(t *testing.T) {
	// matchPathFilter is only called when patterns exist; verify behavior
	if matchPathFilter([]string{"src/**"}, nil) {
		t.Error("should not match with no changed files")
	}
	if matchPathFilter([]string{"src/**"}, []string{}) {
		t.Error("should not match with empty changed files")
	}
}

func TestMatchPathFilter_SimpleFile(t *testing.T) {
	if !matchPathFilter([]string{"Dockerfile"}, []string{"Dockerfile"}) {
		t.Error("exact file match should succeed")
	}
	if matchPathFilter([]string{"Dockerfile"}, []string{"README.md"}) {
		t.Error("different file should not match")
	}
}

func TestMatchPathFilter_GlobPattern(t *testing.T) {
	files := []string{"src/main.go", "src/util/helper.go", "README.md"}

	if !matchPathFilter([]string{"src/**"}, files) {
		t.Error("src/** should match src/main.go")
	}
	if !matchPathFilter([]string{"**/*.go"}, files) {
		t.Error("**/*.go should match .go files")
	}
	if matchPathFilter([]string{"pkg/**"}, files) {
		t.Error("pkg/** should not match any of the files")
	}
}

func TestMatchPathFilter_MultiplePatterns(t *testing.T) {
	files := []string{"docs/README.md"}
	patterns := []string{"src/**", "docs/**"}

	if !matchPathFilter(patterns, files) {
		t.Error("should match when any pattern matches")
	}
}

func TestMatchFilePath_SingleStar(t *testing.T) {
	if !matchFilePath("*.go", "main.go") {
		t.Error("*.go should match main.go")
	}
	// *.go pattern applied to basename
	if !matchFilePath("*.go", "src/main.go") {
		t.Error("*.go should match src/main.go via basename")
	}
}

func TestMatchFilePath_DoubleStarPrefix(t *testing.T) {
	if !matchFilePath("src/**", "src/foo.ts") {
		t.Error("src/** should match src/foo.ts")
	}
	if !matchFilePath("src/**", "src/a/b/c.ts") {
		t.Error("src/** should match deep nested file")
	}
	if matchFilePath("src/**", "pkg/foo.ts") {
		t.Error("src/** should NOT match pkg/foo.ts")
	}
}

func TestMatchFilePath_DoubleStarMiddle(t *testing.T) {
	if !matchFilePath("**/*.go", "main.go") {
		t.Error("**/*.go should match main.go")
	}
	if !matchFilePath("**/*.go", "pkg/util/helper.go") {
		t.Error("**/*.go should match nested .go file")
	}
	if matchFilePath("**/*.go", "main.ts") {
		t.Error("**/*.go should NOT match .ts file")
	}
}

// ---------------------------------------------------------------------------
// Full Webhook Trigger Evaluation
// ---------------------------------------------------------------------------

func TestEvaluateWebhookTriggers_Match(t *testing.T) {
	rules := []TriggerRule{
		{
			Type:   "webhook",
			Repo:   "company/backend",
			Branch: "main",
			Events: []string{"push"},
		},
	}
	event := &WebhookEvent{
		Repo:      "company/backend",
		Branch:    "main",
		EventType: "push",
	}

	result := EvaluateWebhookTriggers(rules, event)
	if !result.Matched {
		t.Error("should match")
	}
	if result.RuleIndex != 0 {
		t.Errorf("expected rule index 0, got %d", result.RuleIndex)
	}
}

func TestEvaluateWebhookTriggers_NoMatch(t *testing.T) {
	rules := []TriggerRule{
		{
			Type:   "webhook",
			Repo:   "company/backend",
			Branch: "main",
			Events: []string{"push"},
		},
	}
	event := &WebhookEvent{
		Repo:      "company/frontend",
		Branch:    "main",
		EventType: "push",
	}

	result := EvaluateWebhookTriggers(rules, event)
	if result.Matched {
		t.Error("should not match different repo")
	}
}

func TestEvaluateWebhookTriggers_BranchMismatch(t *testing.T) {
	rules := []TriggerRule{
		{
			Type:   "webhook",
			Repo:   "company/backend",
			Branch: "main",
			Events: []string{"push"},
		},
	}
	event := &WebhookEvent{
		Repo:      "company/backend",
		Branch:    "develop",
		EventType: "push",
	}

	result := EvaluateWebhookTriggers(rules, event)
	if result.Matched {
		t.Error("should not match different branch")
	}
}

func TestEvaluateWebhookTriggers_EventMismatch(t *testing.T) {
	rules := []TriggerRule{
		{
			Type:   "webhook",
			Repo:   "company/backend",
			Branch: "main",
			Events: []string{"push"},
		},
	}
	event := &WebhookEvent{
		Repo:      "company/backend",
		Branch:    "main",
		EventType: "merge_request",
	}

	result := EvaluateWebhookTriggers(rules, event)
	if result.Matched {
		t.Error("should not match wrong event type")
	}
}

func TestEvaluateWebhookTriggers_WithPathFilter(t *testing.T) {
	rules := []TriggerRule{
		{
			Type:       "webhook",
			Repo:       "company/backend",
			Branch:     "main",
			Events:     []string{"push"},
			PathFilter: []string{"src/**", "Dockerfile"},
		},
	}

	// Match — changed file in src/
	result := EvaluateWebhookTriggers(rules, &WebhookEvent{
		Repo:         "company/backend",
		Branch:       "main",
		EventType:    "push",
		ChangedFiles: []string{"src/main.go"},
	})
	if !result.Matched {
		t.Error("should match — changed file matches path filter")
	}

	// No match — only docs changed
	result = EvaluateWebhookTriggers(rules, &WebhookEvent{
		Repo:         "company/backend",
		Branch:       "main",
		EventType:    "push",
		ChangedFiles: []string{"docs/README.md"},
	})
	if result.Matched {
		t.Error("should NOT match — no changed file matches path filter")
	}
}

func TestEvaluateWebhookTriggers_MultipleRules(t *testing.T) {
	rules := []TriggerRule{
		{
			Type:   "webhook",
			Repo:   "company/backend",
			Branch: "main",
			Events: []string{"push"},
		},
		{
			Type:   "webhook",
			Repo:   "company/backend",
			Branch: "release/*",
			Events: []string{"push", "tag_push"},
		},
		{
			Type: "schedule",
			Cron: "0 2 * * *",
		},
	}

	// Match second rule
	result := EvaluateWebhookTriggers(rules, &WebhookEvent{
		Repo:      "company/backend",
		Branch:    "release/v1.0",
		EventType: "push",
	})
	if !result.Matched {
		t.Error("should match second rule")
	}
	if result.RuleIndex != 1 {
		t.Errorf("expected rule index 1, got %d", result.RuleIndex)
	}
}

func TestEvaluateWebhookTriggers_SkipsScheduleRules(t *testing.T) {
	rules := []TriggerRule{
		{Type: "schedule", Cron: "0 2 * * *"},
	}
	result := EvaluateWebhookTriggers(rules, &WebhookEvent{
		Repo:      "company/backend",
		Branch:    "main",
		EventType: "push",
	})
	if result.Matched {
		t.Error("schedule rules should not match webhook events")
	}
}

func TestEvaluateWebhookTriggers_ProviderMatch(t *testing.T) {
	rules := []TriggerRule{
		{
			Type:     "webhook",
			Provider: "gitlab",
			Repo:     "company/backend",
			Branch:   "main",
			Events:   []string{"push"},
		},
	}

	// Match with correct provider
	result := EvaluateWebhookTriggers(rules, &WebhookEvent{
		Provider:  "gitlab",
		Repo:      "company/backend",
		Branch:    "main",
		EventType: "push",
	})
	if !result.Matched {
		t.Error("should match correct provider")
	}

	// No match with wrong provider
	result = EvaluateWebhookTriggers(rules, &WebhookEvent{
		Provider:  "github",
		Repo:      "company/backend",
		Branch:    "main",
		EventType: "push",
	})
	if result.Matched {
		t.Error("should not match wrong provider")
	}
}

func TestEvaluateWebhookTriggers_CaseInsensitiveRepo(t *testing.T) {
	rules := []TriggerRule{
		{
			Type:   "webhook",
			Repo:   "Company/Backend",
			Branch: "main",
			Events: []string{"push"},
		},
	}
	result := EvaluateWebhookTriggers(rules, &WebhookEvent{
		Repo:      "company/backend",
		Branch:    "main",
		EventType: "push",
	})
	if !result.Matched {
		t.Error("repo matching should be case-insensitive")
	}
}

// ---------------------------------------------------------------------------
// Parse & Validate
// ---------------------------------------------------------------------------

func TestParseTriggerRules_Valid(t *testing.T) {
	json := `[{"type":"webhook","repo":"co/be","branch":"main","events":["push"]},{"type":"schedule","cron":"0 2 * * *"}]`
	rules, err := ParseTriggerRules(json)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rules) != 2 {
		t.Errorf("expected 2 rules, got %d", len(rules))
	}
}

func TestParseTriggerRules_Empty(t *testing.T) {
	for _, input := range []string{"", "null", "[]"} {
		rules, err := ParseTriggerRules(input)
		if err != nil {
			t.Errorf("unexpected error for %q: %v", input, err)
		}
		if len(rules) != 0 {
			t.Errorf("expected 0 rules for %q, got %d", input, len(rules))
		}
	}
}

func TestParseTriggerRules_Invalid(t *testing.T) {
	_, err := ParseTriggerRules("not-json")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestValidateTriggerRules_Valid(t *testing.T) {
	rules := []TriggerRule{
		{Type: "webhook", Repo: "co/be", Branch: "main", Events: []string{"push"}},
		{Type: "schedule", Cron: "0 2 * * *"},
	}
	errs := ValidateTriggerRules(rules)
	if len(errs) > 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestValidateTriggerRules_MissingFields(t *testing.T) {
	rules := []TriggerRule{
		{Type: "webhook"}, // missing repo, branch, events
	}
	errs := ValidateTriggerRules(rules)
	if len(errs) != 3 {
		t.Errorf("expected 3 errors, got %d: %v", len(errs), errs)
	}
}

func TestValidateTriggerRules_InvalidEvent(t *testing.T) {
	rules := []TriggerRule{
		{Type: "webhook", Repo: "co/be", Branch: "main", Events: []string{"invalid_event"}},
	}
	errs := ValidateTriggerRules(rules)
	if len(errs) != 1 {
		t.Errorf("expected 1 error for invalid event, got %d: %v", len(errs), errs)
	}
}

func TestValidateTriggerRules_UnknownType(t *testing.T) {
	rules := []TriggerRule{
		{Type: "unknown"},
	}
	errs := ValidateTriggerRules(rules)
	if len(errs) != 1 {
		t.Errorf("expected 1 error for unknown type, got %d: %v", len(errs), errs)
	}
}

// ---------------------------------------------------------------------------
// Cron Validation
// ---------------------------------------------------------------------------

func TestValidateCronExpression_Valid(t *testing.T) {
	valid := []string{
		"0 2 * * *",
		"*/15 * * * *",
		"0 0 1 * *",
		"30 4 * * 1-5",
		"0 0,12 * * *",
		"0 0 * * 0,6",
	}
	for _, expr := range valid {
		if err := ValidateCronExpression(expr); err != nil {
			t.Errorf("expected valid cron %q, got error: %v", expr, err)
		}
	}
}

func TestValidateCronExpression_Invalid(t *testing.T) {
	invalid := []string{
		"",
		"0 2 * *",        // only 4 fields
		"0 2 * * * *",    // 6 fields
		"60 * * * *",     // minute 60 out of range
		"* 25 * * *",     // hour 25 out of range
		"* * 32 * *",     // day 32 out of range
		"* * * 13 *",     // month 13 out of range
		"* * * * 8",      // day-of-week 8 out of range
		"abc * * * *",    // non-numeric
		"*/0 * * * *",    // step 0
	}
	for _, expr := range invalid {
		if err := ValidateCronExpression(expr); err == nil {
			t.Errorf("expected invalid cron %q to fail", expr)
		}
	}
}

func TestValidateCronExpression_Ranges(t *testing.T) {
	if err := ValidateCronExpression("0 9-17 * * *"); err != nil {
		t.Errorf("valid range should pass: %v", err)
	}
	if err := ValidateCronExpression("0 17-9 * * *"); err == nil {
		t.Error("inverted range should fail")
	}
}

// ---------------------------------------------------------------------------
// GetScheduleRules
// ---------------------------------------------------------------------------

func TestGetScheduleRules(t *testing.T) {
	rules := []TriggerRule{
		{Type: "webhook", Repo: "co/be", Branch: "main"},
		{Type: "schedule", Cron: "0 2 * * *"},
		{Type: "schedule", Cron: "0 14 * * 1-5"},
		{Type: "schedule"}, // no cron — should be excluded
	}
	schedules := GetScheduleRules(rules)
	if len(schedules) != 2 {
		t.Errorf("expected 2 schedule rules, got %d", len(schedules))
	}
}

// ---------------------------------------------------------------------------
// isValidWebhookEvent
// ---------------------------------------------------------------------------

func TestIsValidWebhookEvent(t *testing.T) {
	valid := []string{"push", "merge_request", "pull_request", "tag_push", "release"}
	for _, ev := range valid {
		if !isValidWebhookEvent(ev) {
			t.Errorf("expected %q to be valid", ev)
		}
	}
	if isValidWebhookEvent("unknown") {
		t.Error("expected unknown to be invalid")
	}
}

// ---------------------------------------------------------------------------
// RepoURL precise matching (M14.1)
// ---------------------------------------------------------------------------

func TestEvaluateWebhookTriggers_RepoURLPreciseMatch(t *testing.T) {
	rules := []TriggerRule{
		{
			Type:    "webhook",
			RepoURL: "https://github.com/company/backend",
			Branch:  "main",
			Events:  []string{"push"},
		},
	}

	t.Run("matches exact repo URL", func(t *testing.T) {
		event := &WebhookEvent{
			Provider:  "github",
			Repo:      "company/backend",
			RepoURL:   "https://github.com/company/backend",
			Branch:    "main",
			EventType: "push",
		}
		result := EvaluateWebhookTriggers(rules, event)
		if !result.Matched {
			t.Errorf("expected match, got: %s", result.Reason)
		}
	})

	t.Run("rejects different repo URL", func(t *testing.T) {
		event := &WebhookEvent{
			Provider:  "github",
			Repo:      "company/backend",
			RepoURL:   "https://github.com/company/other-service",
			Branch:    "main",
			EventType: "push",
		}
		result := EvaluateWebhookTriggers(rules, event)
		if result.Matched {
			t.Error("expected no match for different repo URL")
		}
	})

	t.Run("rejects when event has no RepoURL but rule requires it", func(t *testing.T) {
		event := &WebhookEvent{
			Provider:  "github",
			Repo:      "company/backend",
			RepoURL:   "", // not populated
			Branch:    "main",
			EventType: "push",
		}
		result := EvaluateWebhookTriggers(rules, event)
		if result.Matched {
			t.Error("expected no match when event has no RepoURL and rule requires it")
		}
	})

	t.Run("case-insensitive URL match", func(t *testing.T) {
		event := &WebhookEvent{
			Provider:  "github",
			Repo:      "company/backend",
			RepoURL:   "HTTPS://GITHUB.COM/COMPANY/BACKEND",
			Branch:    "main",
			EventType: "push",
		}
		result := EvaluateWebhookTriggers(rules, event)
		if !result.Matched {
			t.Errorf("expected case-insensitive URL match, got: %s", result.Reason)
		}
	})
}

// ---------------------------------------------------------------------------
// Monorepo: multiple Pipelines with distinct path_filter should isolate
// ---------------------------------------------------------------------------

func TestMonorepo_PathFilterIsolation(t *testing.T) {
	userSvcRules := []TriggerRule{{
		Type: "webhook", Repo: "root/saas-uat", Branch: "main",
		Events: []string{"push"}, PathFilter: []string{"services/user-service/**", "shared/**"},
	}}
	orderSvcRules := []TriggerRule{{
		Type: "webhook", Repo: "root/saas-uat", Branch: "main",
		Events: []string{"push"}, PathFilter: []string{"services/order-service/**", "shared/**"},
	}}

	tests := []struct {
		name         string
		changedFiles []string
		userMatch    bool
		orderMatch   bool
	}{
		{
			name:         "user-service only",
			changedFiles: []string{"services/user-service/src/main.go"},
			userMatch:    true,
			orderMatch:   false,
		},
		{
			name:         "order-service only",
			changedFiles: []string{"services/order-service/pom.xml"},
			userMatch:    false,
			orderMatch:   true,
		},
		{
			name:         "shared changes trigger both",
			changedFiles: []string{"shared/lib/auth.go"},
			userMatch:    true,
			orderMatch:   true,
		},
		{
			name:         "root README triggers neither",
			changedFiles: []string{"README.md"},
			userMatch:    false,
			orderMatch:   false,
		},
		{
			name:         "mixed changes",
			changedFiles: []string{"services/user-service/handler.go", "docs/api.md"},
			userMatch:    true,
			orderMatch:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := &WebhookEvent{
				Repo: "root/saas-uat", Branch: "main", EventType: "push",
				ChangedFiles: tt.changedFiles,
			}
			userResult := EvaluateWebhookTriggers(userSvcRules, event)
			orderResult := EvaluateWebhookTriggers(orderSvcRules, event)

			if userResult.Matched != tt.userMatch {
				t.Errorf("user-service: expected matched=%v, got %v (%s)", tt.userMatch, userResult.Matched, userResult.Reason)
			}
			if orderResult.Matched != tt.orderMatch {
				t.Errorf("order-service: expected matched=%v, got %v (%s)", tt.orderMatch, orderResult.Matched, orderResult.Reason)
			}
		})
	}
}

func TestEvaluateWebhookTriggers_LegacyRepoPatchMatchStillWorks(t *testing.T) {
	// Rules without RepoURL should still use Repo path matching
	rules := []TriggerRule{
		{
			Type:   "webhook",
			Repo:   "company/backend",
			Branch: "main",
			Events: []string{"push"},
		},
	}
	event := &WebhookEvent{
		Provider:  "github",
		Repo:      "company/backend",
		RepoURL:   "https://github.com/company/backend",
		Branch:    "main",
		EventType: "push",
	}
	result := EvaluateWebhookTriggers(rules, event)
	if !result.Matched {
		t.Errorf("legacy repo path matching should still work, got: %s", result.Reason)
	}
}
