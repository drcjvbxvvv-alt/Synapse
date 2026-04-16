// Package github implements the CI engine adapter for GitHub Actions (M18e).
//
// GitHub Actions is reachable through the REST API at api.github.com (or
// self-hosted GitHub Enterprise). The adapter uses a bearer-token HTTP
// client and the /actions/* endpoint family. Notable quirks compared to
// GitLab / Jenkins:
//
//   - Triggering a workflow via POST /dispatches returns **204 No Content**
//     with no run id; the caller must poll /actions/runs filtered by the
//     workflow id + a "created after" cutoff to discover the resulting
//     run id.
//   - "Status" and "Conclusion" are two separate fields. A run is
//     completed when status==completed; conclusion then describes success /
//     failure / cancellation / skipped / timed out.
package github

import "time"

// workflowRun mirrors the subset of GET /repos/{owner}/{repo}/actions/runs
// that the adapter consumes. Full schema:
// https://docs.github.com/en/rest/actions/workflow-runs
type workflowRun struct {
	ID           int64      `json:"id"`
	Name         string     `json:"name,omitempty"`
	HeadBranch   string     `json:"head_branch,omitempty"`
	HeadSHA      string     `json:"head_sha,omitempty"`
	Status       string     `json:"status"`               // queued | in_progress | completed | requested | waiting | pending
	Conclusion   string     `json:"conclusion,omitempty"` // success | failure | cancelled | skipped | timed_out | action_required | neutral (null while running)
	Event        string     `json:"event,omitempty"`      // push | workflow_dispatch | ...
	WorkflowID   int64      `json:"workflow_id,omitempty"`
	HTMLURL      string     `json:"html_url,omitempty"`
	CreatedAt    *time.Time `json:"created_at,omitempty"`
	UpdatedAt    *time.Time `json:"updated_at,omitempty"`
	RunStartedAt *time.Time `json:"run_started_at,omitempty"`
}

// workflowRunList mirrors the list response envelope.
type workflowRunList struct {
	TotalCount   int           `json:"total_count"`
	WorkflowRuns []workflowRun `json:"workflow_runs"`
}

// workflowJob mirrors one entry of GET /runs/{run_id}/jobs.
type workflowJob struct {
	ID          int64      `json:"id"`
	Name        string     `json:"name"`
	Status      string     `json:"status"`
	Conclusion  string     `json:"conclusion,omitempty"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// workflowJobList mirrors the jobs list envelope.
type workflowJobList struct {
	TotalCount int           `json:"total_count"`
	Jobs       []workflowJob `json:"jobs"`
}

// dispatchRequest is the body for POST /workflows/{id}/dispatches.
//
// Inputs may include only strings (GitHub validates types server-side); the
// adapter passes TriggerRequest.Variables verbatim.
type dispatchRequest struct {
	Ref    string            `json:"ref"`
	Inputs map[string]string `json:"inputs,omitempty"`
}

// artifactsList mirrors GET /runs/{run_id}/artifacts.
type artifactsList struct {
	TotalCount int              `json:"total_count"`
	Artifacts  []artifactEntry `json:"artifacts"`
}

// artifactEntry is one element inside artifactsList.
type artifactEntry struct {
	ID                 int64      `json:"id"`
	Name               string     `json:"name"`
	SizeInBytes        int64      `json:"size_in_bytes"`
	URL                string     `json:"url"`
	ArchiveDownloadURL string     `json:"archive_download_url"`
	CreatedAt          *time.Time `json:"created_at,omitempty"`
}
