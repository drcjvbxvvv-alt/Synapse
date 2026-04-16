// Package gitlab implements the CI engine adapter for GitLab CI (M18b).
//
// The adapter delegates pipeline execution to a remote GitLab instance via the
// REST API v4. Synapse acts as a "war room" that triggers, polls and streams
// results; the GitLab instance remains the source of truth for its own runs.
//
// This file defines the Data Transfer Objects that mirror GitLab's JSON
// responses. We deliberately keep them private (lowercase field references
// only through the adapter) and minimal — add fields as new adapter features
// need them.
package gitlab

import "time"

// gitlabPipeline mirrors the subset of fields returned by
// GET /projects/:id/pipelines/:id that the adapter needs.
//
// Full schema: https://docs.gitlab.com/ee/api/pipelines.html
type gitlabPipeline struct {
	ID         int64      `json:"id"`
	IID        int64      `json:"iid"`
	ProjectID  int64      `json:"project_id"`
	Status     string     `json:"status"`
	Ref        string     `json:"ref"`
	SHA        string     `json:"sha"`
	WebURL     string     `json:"web_url"`
	CreatedAt  *time.Time `json:"created_at,omitempty"`
	UpdatedAt  *time.Time `json:"updated_at,omitempty"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
}

// gitlabJob mirrors the subset of fields returned by
// GET /projects/:id/pipelines/:id/jobs.
//
// Full schema: https://docs.gitlab.com/ee/api/jobs.html
type gitlabJob struct {
	ID         int64      `json:"id"`
	Name       string     `json:"name"`
	Stage      string     `json:"stage"`
	Status     string     `json:"status"`
	Ref        string     `json:"ref"`
	CreatedAt  *time.Time `json:"created_at,omitempty"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`

	// Artifacts file metadata; present only when the job produces artifacts.
	ArtifactsFile *gitlabArtifactMeta `json:"artifacts_file,omitempty"`
}

// gitlabArtifactMeta is the single-file artifact descriptor nested inside
// gitlabJob responses.
type gitlabArtifactMeta struct {
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
}

// gitlabVersion mirrors GET /api/v4/version.
type gitlabVersion struct {
	Version  string `json:"version"`
	Revision string `json:"revision,omitempty"`
}

// triggerVariable is one element of the `variables[]` array sent to the
// trigger pipeline endpoint. GitLab accepts key/value pairs of arbitrary
// string variables that show up as CI_JOB environment variables.
type triggerVariable struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// triggerRequest is the payload for POST /api/v4/projects/:id/pipeline.
// See https://docs.gitlab.com/ee/api/pipelines.html#create-a-new-pipeline.
type triggerRequest struct {
	Ref       string            `json:"ref"`
	Variables []triggerVariable `json:"variables,omitempty"`
}
