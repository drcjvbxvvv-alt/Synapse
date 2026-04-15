package services

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/shaia/Synapse/pkg/logger"
)

// ---------------------------------------------------------------------------
// Pipeline Trigger Condition Engine（CICD_ARCHITECTURE §10.5, P1-10）
//
// 設計原則：
//   - Pipeline 觸發條件定義於 PipelineVersion.TriggersJSON
//   - Webhook 事件到達時，解析 payload 取得 branch / event / changed paths
//   - 逐條比對觸發規則：repo 完全匹配 + branch glob + event 白名單 + path filter
//   - 所有條件「全部滿足」才觸發（AND 邏輯）
//   - Cron 觸發由外部排程器驅動，此引擎負責解析 cron 設定
//   - 純函數設計，方便單元測試
// ---------------------------------------------------------------------------

// TriggerRule 單條觸發規則。
type TriggerRule struct {
	Type       string   `json:"type"`                  // "webhook" | "schedule"
	Provider   string   `json:"provider,omitempty"`     // git provider name
	Repo       string   `json:"repo,omitempty"`         // full repo path (e.g. "company/backend")
	RepoURL    string   `json:"repo_url,omitempty"`     // full repo URL for precise matching（M14.1）
	Branch     string   `json:"branch,omitempty"`       // glob pattern (e.g. "main", "release/*")
	Events     []string `json:"events,omitempty"`       // allowed event types (e.g. ["push", "merge_request"])
	PathFilter []string `json:"path_filter,omitempty"`  // glob patterns for changed files
	Cron       string   `json:"cron,omitempty"`         // cron expression (schedule type only)
	ClusterID  uint     `json:"cluster_id,omitempty"`   // 目標叢集（git webhook 觸發時必填）
	Namespace  string   `json:"namespace,omitempty"`    // 目標 namespace（git webhook 觸發時必填）
}

// WebhookEvent 從 Webhook payload 解析出的事件資訊。
type WebhookEvent struct {
	Provider     string   // "github", "gitlab", "gitea"
	Repo         string   // "company/backend-service"
	RepoURL      string   // full clone URL, e.g. "https://github.com/company/backend" (M14.1)
	Branch       string   // "main", "feature/foo"
	EventType    string   // "push", "merge_request", "pull_request"
	ChangedFiles []string // list of changed file paths
}

// TriggerMatchResult 觸發匹配結果。
type TriggerMatchResult struct {
	Matched     bool   `json:"matched"`
	RuleIndex   int    `json:"rule_index"`   // which rule matched (-1 if none)
	Reason      string `json:"reason"`       // human-readable reason
}

// ParseTriggerRules 從 JSON 字串解析觸發規則列表。
func ParseTriggerRules(triggersJSON string) ([]TriggerRule, error) {
	if triggersJSON == "" || triggersJSON == "null" || triggersJSON == "[]" {
		return nil, nil
	}

	var rules []TriggerRule
	if err := json.Unmarshal([]byte(triggersJSON), &rules); err != nil {
		return nil, fmt.Errorf("parse trigger rules: %w", err)
	}
	return rules, nil
}

// EvaluateWebhookTriggers 評估 Webhook 事件是否滿足任一觸發規則。
// 遍歷所有 webhook 類型規則，任一匹配即回傳 matched=true。
func EvaluateWebhookTriggers(rules []TriggerRule, event *WebhookEvent) TriggerMatchResult {
	for i, rule := range rules {
		if rule.Type != "webhook" {
			continue
		}

		if reason, ok := matchWebhookRule(&rule, event); ok {
			return TriggerMatchResult{
				Matched:   true,
				RuleIndex: i,
				Reason:    reason,
			}
		}
	}

	return TriggerMatchResult{
		Matched:   false,
		RuleIndex: -1,
		Reason:    "no webhook trigger rule matched",
	}
}

// GetScheduleRules 從規則列表中提取所有 schedule 類型的規則及其 cron 表達式。
func GetScheduleRules(rules []TriggerRule) []TriggerRule {
	var schedules []TriggerRule
	for _, r := range rules {
		if r.Type == "schedule" && r.Cron != "" {
			schedules = append(schedules, r)
		}
	}
	return schedules
}

// ValidateTriggerRules 驗證觸發規則的完整性。
func ValidateTriggerRules(rules []TriggerRule) []string {
	var errs []string
	for i, rule := range rules {
		prefix := fmt.Sprintf("triggers[%d]", i)
		switch rule.Type {
		case "webhook":
			if rule.Repo == "" {
				errs = append(errs, fmt.Sprintf("%s: webhook trigger requires 'repo'", prefix))
			}
			if rule.Branch == "" {
				errs = append(errs, fmt.Sprintf("%s: webhook trigger requires 'branch'", prefix))
			}
			if len(rule.Events) == 0 {
				errs = append(errs, fmt.Sprintf("%s: webhook trigger requires at least one event", prefix))
			}
			for _, ev := range rule.Events {
				if !isValidWebhookEvent(ev) {
					errs = append(errs, fmt.Sprintf("%s: unknown event type %q", prefix, ev))
				}
			}
			for _, pf := range rule.PathFilter {
				if _, err := filepath.Match(pf, "test"); err != nil {
					errs = append(errs, fmt.Sprintf("%s: invalid path_filter pattern %q: %v", prefix, pf, err))
				}
			}
		case "schedule":
			if rule.Cron == "" {
				errs = append(errs, fmt.Sprintf("%s: schedule trigger requires 'cron'", prefix))
			} else if err := ValidateCronExpression(rule.Cron); err != nil {
				errs = append(errs, fmt.Sprintf("%s: invalid cron expression: %v", prefix, err))
			}
		default:
			errs = append(errs, fmt.Sprintf("%s: unknown trigger type %q", prefix, rule.Type))
		}
	}
	return errs
}

// --- Matching logic ---

func matchWebhookRule(rule *TriggerRule, event *WebhookEvent) (string, bool) {
	// 1. Provider match (optional — if specified, must match)
	if rule.Provider != "" && !strings.EqualFold(rule.Provider, event.Provider) {
		return "", false
	}

	// 2. Repo match:
	//    - If rule.RepoURL is set (M14.1 precise matching), compare full URL.
	//    - Otherwise fall back to Repo path matching (legacy).
	if rule.RepoURL != "" {
		if event.RepoURL == "" || !strings.EqualFold(rule.RepoURL, event.RepoURL) {
			return "", false
		}
	} else if !strings.EqualFold(rule.Repo, event.Repo) {
		return "", false
	}

	// 3. Branch glob match
	if !matchBranchGlob(rule.Branch, event.Branch) {
		return "", false
	}

	// 4. Event type match
	if !matchEvent(rule.Events, event.EventType) {
		return "", false
	}

	// 5. Path filter (optional — if specified, at least one changed file must match)
	if len(rule.PathFilter) > 0 {
		if !matchPathFilter(rule.PathFilter, event.ChangedFiles) {
			logger.Debug("webhook trigger: path filter not matched",
				"repo", event.Repo,
				"branch", event.Branch,
				"filter", rule.PathFilter,
				"changed_files_count", len(event.ChangedFiles),
			)
			return "", false
		}
	}

	reason := fmt.Sprintf("matched: repo=%s branch=%s event=%s", event.Repo, event.Branch, event.EventType)
	return reason, true
}

// matchBranchGlob 比對 branch 是否匹配 glob pattern。
// 支援：
//   - 完全匹配: "main"
//   - 萬用字元: "release/*", "feature/**", "*"
//   - 多段萬用: "release/**" 匹配 "release/v1/hotfix"
func matchBranchGlob(pattern, branch string) bool {
	if pattern == "*" || pattern == "**" {
		return true
	}

	// filepath.Match handles single-level wildcards well
	// For "**" (multi-level), we need special handling
	if strings.Contains(pattern, "**") {
		return matchDoubleStarBranch(pattern, branch)
	}

	matched, err := filepath.Match(pattern, branch)
	if err != nil {
		logger.Warn("invalid branch glob pattern", "pattern", pattern, "error", err)
		return false
	}
	return matched
}

// matchDoubleStarBranch handles ** patterns for branches like "release/**"
// matching "release/v1", "release/v1/hotfix", etc.
func matchDoubleStarBranch(pattern, branch string) bool {
	// Split pattern at "**"
	parts := strings.SplitN(pattern, "**", 2)
	if len(parts) != 2 {
		return false
	}

	prefix := parts[0]
	suffix := parts[1]

	// Check prefix
	if prefix != "" && !strings.HasPrefix(branch, prefix) {
		return false
	}

	// Check suffix
	if suffix != "" {
		remaining := branch[len(prefix):]
		return strings.HasSuffix(remaining, suffix)
	}

	// Pattern is "prefix/**" — branch must start with prefix and have more content
	if prefix != "" {
		return strings.HasPrefix(branch, prefix)
	}

	return true
}

// matchEvent 檢查事件類型是否在允許列表中。
func matchEvent(allowedEvents []string, eventType string) bool {
	if len(allowedEvents) == 0 {
		return true // 未設定 = 全部允許
	}
	for _, ev := range allowedEvents {
		if strings.EqualFold(ev, eventType) {
			return true
		}
	}
	return false
}

// matchPathFilter 檢查是否有任一 changed file 匹配任一 path filter pattern。
func matchPathFilter(patterns []string, changedFiles []string) bool {
	if len(changedFiles) == 0 {
		return false // 無檔案變動資訊 → 不匹配
	}

	for _, file := range changedFiles {
		for _, pattern := range patterns {
			if matchFilePath(pattern, file) {
				return true
			}
		}
	}
	return false
}

// matchFilePath 比對單個檔案路徑是否匹配 glob pattern。
// 支援：
//   - "*.go" 匹配 "main.go"
//   - "src/**" 匹配 "src/foo/bar.ts"
//   - "src/*.ts" 匹配 "src/index.ts"
//   - "Dockerfile" 完全匹配
func matchFilePath(pattern, filePath string) bool {
	// Handle "**" patterns (recursive match)
	if strings.Contains(pattern, "**") {
		return matchDoubleStarPath(pattern, filePath)
	}

	// Standard glob match
	matched, err := filepath.Match(pattern, filePath)
	if err != nil {
		return false
	}
	if matched {
		return true
	}

	// Also try matching just the filename for patterns without path separator
	if !strings.Contains(pattern, "/") {
		base := filepath.Base(filePath)
		matched, err = filepath.Match(pattern, base)
		if err != nil {
			return false
		}
		return matched
	}

	return false
}

// matchDoubleStarPath handles ** in file path patterns.
// "src/**" matches "src/foo.ts", "src/a/b/c.ts"
// "**/*.go" matches "main.go", "pkg/util/helper.go"
func matchDoubleStarPath(pattern, filePath string) bool {
	parts := strings.SplitN(pattern, "**", 2)
	if len(parts) != 2 {
		return false
	}

	prefix := parts[0]
	suffix := parts[1]

	// Check prefix
	if prefix != "" {
		if !strings.HasPrefix(filePath, prefix) {
			return false
		}
		filePath = filePath[len(prefix):]
	}

	// If no suffix after **, match anything
	if suffix == "" {
		return true
	}

	// Remove leading / from suffix
	suffix = strings.TrimPrefix(suffix, "/")

	// suffix is like "*.go" — match against every possible sub-path
	// Try matching suffix against the remaining path and all sub-paths
	segments := strings.Split(filePath, "/")
	for i := range segments {
		remaining := strings.Join(segments[i:], "/")
		matched, err := filepath.Match(suffix, remaining)
		if err == nil && matched {
			return true
		}
		// Also try just the filename for single-segment suffix
		if !strings.Contains(suffix, "/") {
			matched, err = filepath.Match(suffix, segments[i])
			if err == nil && matched {
				return true
			}
		}
	}

	return false
}

// --- Cron validation ---

// ValidateCronExpression 驗證 cron 表達式（5 欄位標準格式）。
// 格式：minute hour day-of-month month day-of-week
func ValidateCronExpression(expr string) error {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return fmt.Errorf("expected 5 fields, got %d", len(fields))
	}

	limits := []struct {
		name string
		min  int
		max  int
	}{
		{"minute", 0, 59},
		{"hour", 0, 23},
		{"day-of-month", 1, 31},
		{"month", 1, 12},
		{"day-of-week", 0, 7}, // 0 and 7 = Sunday
	}

	for i, field := range fields {
		if err := validateCronField(field, limits[i].min, limits[i].max); err != nil {
			return fmt.Errorf("field %q (%s): %w", field, limits[i].name, err)
		}
	}

	return nil
}

func validateCronField(field string, min, max int) error {
	// Handle wildcards
	if field == "*" {
		return nil
	}

	// Handle */N (step)
	if strings.HasPrefix(field, "*/") {
		step := field[2:]
		n, err := parseInt(step)
		if err != nil {
			return fmt.Errorf("invalid step value %q", step)
		}
		if n <= 0 || n > max {
			return fmt.Errorf("step %d out of range [1-%d]", n, max)
		}
		return nil
	}

	// Handle comma-separated values: "1,5,10"
	if strings.Contains(field, ",") {
		for _, part := range strings.Split(field, ",") {
			if err := validateCronField(part, min, max); err != nil {
				return err
			}
		}
		return nil
	}

	// Handle ranges: "1-5"
	if strings.Contains(field, "-") {
		parts := strings.SplitN(field, "-", 2)
		lo, err := parseInt(parts[0])
		if err != nil {
			return fmt.Errorf("invalid range start %q", parts[0])
		}
		hi, err := parseInt(parts[1])
		if err != nil {
			return fmt.Errorf("invalid range end %q", parts[1])
		}
		if lo < min || lo > max {
			return fmt.Errorf("range start %d out of range [%d-%d]", lo, min, max)
		}
		if hi < min || hi > max {
			return fmt.Errorf("range end %d out of range [%d-%d]", hi, min, max)
		}
		if lo > hi {
			return fmt.Errorf("range start %d > end %d", lo, hi)
		}
		return nil
	}

	// Single value
	n, err := parseInt(field)
	if err != nil {
		return fmt.Errorf("invalid value %q", field)
	}
	if n < min || n > max {
		return fmt.Errorf("value %d out of range [%d-%d]", n, min, max)
	}
	return nil
}

func parseInt(s string) (int, error) {
	n := 0
	if len(s) == 0 {
		return 0, fmt.Errorf("empty string")
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("non-numeric %q", s)
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}

// --- Event type validation ---

var validWebhookEvents = map[string]bool{
	"push":             true,
	"merge_request":    true,
	"pull_request":     true,
	"tag_push":         true,
	"release":          true,
}

func isValidWebhookEvent(event string) bool {
	return validWebhookEvents[strings.ToLower(event)]
}
