package services

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ---------------------------------------------------------------------------
// Git Provider Webhook Payload Parser（CICD_ARCHITECTURE §10, P2-2）
//
// 設計原則：
//   - 每個 Git Provider（GitHub / GitLab / Gitea）有不同的 webhook payload 格式
//   - 統一轉換為 WebhookEvent，餵入 P1-10 觸發條件引擎
//   - 純函數設計，不依賴外部狀態，方便測試
// ---------------------------------------------------------------------------

// WebhookPayloadParser 解析各 Git Provider 的 webhook payload。
type WebhookPayloadParser interface {
	// ParsePushEvent 解析 push 事件 payload。
	ParsePushEvent(payload []byte) (*WebhookEvent, error)
	// ParseMergeRequestEvent 解析 merge/pull request 事件 payload。
	ParseMergeRequestEvent(payload []byte) (*WebhookEvent, error)
	// DetectEventType 從 HTTP header 或 payload 偵測事件類型。
	DetectEventType(eventHeader string) string
}

// NewWebhookPayloadParser 根據 provider 類型建立對應的 parser。
func NewWebhookPayloadParser(providerType string) (WebhookPayloadParser, error) {
	switch providerType {
	case "github":
		return &GitHubPayloadParser{}, nil
	case "gitlab":
		return &GitLabPayloadParser{}, nil
	case "gitea":
		return &GiteaPayloadParser{}, nil
	default:
		return nil, fmt.Errorf("unsupported git provider type: %s", providerType)
	}
}

// ---------------------------------------------------------------------------
// GitHub
// ---------------------------------------------------------------------------

// GitHubPayloadParser 解析 GitHub webhook payload。
type GitHubPayloadParser struct{}

// githubPushPayload GitHub push event 的 payload 結構（僅取需要的欄位）。
type githubPushPayload struct {
	Ref        string `json:"ref"`
	Repository struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
	Commits []struct {
		Added    []string `json:"added"`
		Modified []string `json:"modified"`
		Removed  []string `json:"removed"`
	} `json:"commits"`
}

// githubPRPayload GitHub pull_request event 的 payload 結構。
type githubPRPayload struct {
	Action      string `json:"action"`
	PullRequest struct {
		Head struct {
			Ref string `json:"ref"`
		} `json:"head"`
	} `json:"pull_request"`
	Repository struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
}

func (p *GitHubPayloadParser) ParsePushEvent(payload []byte) (*WebhookEvent, error) {
	var data githubPushPayload
	if err := json.Unmarshal(payload, &data); err != nil {
		return nil, fmt.Errorf("parse GitHub push payload: %w", err)
	}

	branch := strings.TrimPrefix(data.Ref, "refs/heads/")
	var changedFiles []string
	for _, c := range data.Commits {
		changedFiles = append(changedFiles, c.Added...)
		changedFiles = append(changedFiles, c.Modified...)
		changedFiles = append(changedFiles, c.Removed...)
	}
	changedFiles = dedup(changedFiles)

	eventType := "push"
	if strings.HasPrefix(data.Ref, "refs/tags/") {
		eventType = "tag_push"
	}

	return &WebhookEvent{
		Provider:     "github",
		Repo:         data.Repository.FullName,
		Branch:       branch,
		EventType:    eventType,
		ChangedFiles: changedFiles,
	}, nil
}

func (p *GitHubPayloadParser) ParseMergeRequestEvent(payload []byte) (*WebhookEvent, error) {
	var data githubPRPayload
	if err := json.Unmarshal(payload, &data); err != nil {
		return nil, fmt.Errorf("parse GitHub PR payload: %w", err)
	}

	return &WebhookEvent{
		Provider:  "github",
		Repo:      data.Repository.FullName,
		Branch:    data.PullRequest.Head.Ref,
		EventType: "pull_request",
	}, nil
}

func (p *GitHubPayloadParser) DetectEventType(eventHeader string) string {
	switch eventHeader {
	case "push":
		return "push"
	case "pull_request":
		return "pull_request"
	case "release":
		return "release"
	default:
		return eventHeader
	}
}

// ---------------------------------------------------------------------------
// GitLab
// ---------------------------------------------------------------------------

// GitLabPayloadParser 解析 GitLab webhook payload。
type GitLabPayloadParser struct{}

// gitlabPushPayload GitLab push event 的 payload 結構。
type gitlabPushPayload struct {
	Ref     string `json:"ref"`
	Project struct {
		PathWithNamespace string `json:"path_with_namespace"`
	} `json:"project"`
	Commits []struct {
		Added    []string `json:"added"`
		Modified []string `json:"modified"`
		Removed  []string `json:"removed"`
	} `json:"commits"`
}

// gitlabMRPayload GitLab merge_request event 的 payload 結構。
type gitlabMRPayload struct {
	ObjectAttributes struct {
		SourceBranch string `json:"source_branch"`
		Action       string `json:"action"`
	} `json:"object_attributes"`
	Project struct {
		PathWithNamespace string `json:"path_with_namespace"`
	} `json:"project"`
}

func (p *GitLabPayloadParser) ParsePushEvent(payload []byte) (*WebhookEvent, error) {
	var data gitlabPushPayload
	if err := json.Unmarshal(payload, &data); err != nil {
		return nil, fmt.Errorf("parse GitLab push payload: %w", err)
	}

	branch := strings.TrimPrefix(data.Ref, "refs/heads/")
	var changedFiles []string
	for _, c := range data.Commits {
		changedFiles = append(changedFiles, c.Added...)
		changedFiles = append(changedFiles, c.Modified...)
		changedFiles = append(changedFiles, c.Removed...)
	}
	changedFiles = dedup(changedFiles)

	eventType := "push"
	if strings.HasPrefix(data.Ref, "refs/tags/") {
		eventType = "tag_push"
	}

	return &WebhookEvent{
		Provider:     "gitlab",
		Repo:         data.Project.PathWithNamespace,
		Branch:       branch,
		EventType:    eventType,
		ChangedFiles: changedFiles,
	}, nil
}

func (p *GitLabPayloadParser) ParseMergeRequestEvent(payload []byte) (*WebhookEvent, error) {
	var data gitlabMRPayload
	if err := json.Unmarshal(payload, &data); err != nil {
		return nil, fmt.Errorf("parse GitLab MR payload: %w", err)
	}

	return &WebhookEvent{
		Provider:  "gitlab",
		Repo:      data.Project.PathWithNamespace,
		Branch:    data.ObjectAttributes.SourceBranch,
		EventType: "merge_request",
	}, nil
}

func (p *GitLabPayloadParser) DetectEventType(eventHeader string) string {
	switch eventHeader {
	case "Push Hook":
		return "push"
	case "Tag Push Hook":
		return "tag_push"
	case "Merge Request Hook":
		return "merge_request"
	case "Release Hook":
		return "release"
	default:
		return eventHeader
	}
}

// ---------------------------------------------------------------------------
// Gitea
// ---------------------------------------------------------------------------

// GiteaPayloadParser 解析 Gitea webhook payload。
type GiteaPayloadParser struct{}

// giteaPushPayload Gitea push event 的 payload 結構（與 GitHub 相似）。
type giteaPushPayload struct {
	Ref        string `json:"ref"`
	Repository struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
	Commits []struct {
		Added    []string `json:"added"`
		Modified []string `json:"modified"`
		Removed  []string `json:"removed"`
	} `json:"commits"`
}

// giteaPRPayload Gitea pull_request event 的 payload 結構。
type giteaPRPayload struct {
	Action      string `json:"action"`
	PullRequest struct {
		Head struct {
			Ref string `json:"ref"`
		} `json:"head"`
	} `json:"pull_request"`
	Repository struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
}

func (p *GiteaPayloadParser) ParsePushEvent(payload []byte) (*WebhookEvent, error) {
	var data giteaPushPayload
	if err := json.Unmarshal(payload, &data); err != nil {
		return nil, fmt.Errorf("parse Gitea push payload: %w", err)
	}

	branch := strings.TrimPrefix(data.Ref, "refs/heads/")
	var changedFiles []string
	for _, c := range data.Commits {
		changedFiles = append(changedFiles, c.Added...)
		changedFiles = append(changedFiles, c.Modified...)
		changedFiles = append(changedFiles, c.Removed...)
	}
	changedFiles = dedup(changedFiles)

	eventType := "push"
	if strings.HasPrefix(data.Ref, "refs/tags/") {
		eventType = "tag_push"
	}

	return &WebhookEvent{
		Provider:     "gitea",
		Repo:         data.Repository.FullName,
		Branch:       branch,
		EventType:    eventType,
		ChangedFiles: changedFiles,
	}, nil
}

func (p *GiteaPayloadParser) ParseMergeRequestEvent(payload []byte) (*WebhookEvent, error) {
	var data giteaPRPayload
	if err := json.Unmarshal(payload, &data); err != nil {
		return nil, fmt.Errorf("parse Gitea PR payload: %w", err)
	}

	return &WebhookEvent{
		Provider:  "gitea",
		Repo:      data.Repository.FullName,
		Branch:    data.PullRequest.Head.Ref,
		EventType: "pull_request",
	}, nil
}

func (p *GiteaPayloadParser) DetectEventType(eventHeader string) string {
	// Gitea uses X-Gitea-Event header
	switch eventHeader {
	case "push":
		return "push"
	case "pull_request":
		return "pull_request"
	case "release":
		return "release"
	case "create":
		if strings.Contains(eventHeader, "tag") {
			return "tag_push"
		}
		return eventHeader
	default:
		return eventHeader
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func dedup(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	return result
}
