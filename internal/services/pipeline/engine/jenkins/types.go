// Package jenkins implements the CI engine adapter for Jenkins (M18c).
//
// The adapter talks to a Jenkins controller over its REST API. Jenkins has
// several quirks that GitLab does not:
//   - CSRF protection via Crumb (Jenkins ≥ 2.176). Every POST must carry
//     the `Jenkins-Crumb` header (header name is returned by the crumb
//     issuer; cached per client).
//   - Basic auth using `username:api_token` — there is no equivalent of
//     GitLab's PRIVATE-TOKEN single-header scheme.
//   - Triggering a build returns a Queue URL (Location header) rather than
//     the build number itself; the caller polls the queue until a build
//     number materialises.
//   - Jobs live in a folder hierarchy: path "foo/bar/baz" maps to the URL
//     segment "/job/foo/job/bar/job/baz".
//
// The DTOs in this file mirror the subset of Jenkins JSON we consume.
package jenkins

// crumbResponse mirrors GET /crumbIssuer/api/json.
//
// Response example:
//
//	{
//	  "crumb": "abc123...",
//	  "crumbRequestField": "Jenkins-Crumb"
//	}
type crumbResponse struct {
	Crumb             string `json:"crumb"`
	CrumbRequestField string `json:"crumbRequestField"`
}

// queueItem mirrors GET /queue/item/:id/api/json.
//
// Jenkins queues the build first; the `executable` key is populated once a
// build number has been assigned. Until then, the caller must poll.
type queueItem struct {
	Cancelled  bool              `json:"cancelled"`
	Executable *queueExecutable  `json:"executable,omitempty"`
	Why        string            `json:"why,omitempty"` // human-readable waiting reason
	InQueue    bool              `json:"inQueueSince,omitempty"`
	Params     []any             `json:"params,omitempty"`
	Actions    []map[string]any  `json:"actions,omitempty"`
}

// queueExecutable is the assigned build (once Jenkins has scheduled it).
type queueExecutable struct {
	Number int64  `json:"number"`
	URL    string `json:"url"`
}

// jenkinsBuild mirrors the subset of GET /job/:path/:num/api/json that the
// adapter consumes. Full schema: https://www.jenkins.io/doc/book/using/remote-access-api/
type jenkinsBuild struct {
	Number     int64                   `json:"number"`
	Result     string                  `json:"result"`            // SUCCESS | FAILURE | UNSTABLE | ABORTED | null
	Building   bool                    `json:"building"`
	InProgress bool                    `json:"inProgress"`
	Duration   int64                   `json:"duration"`          // milliseconds; 0 while running
	Timestamp  int64                   `json:"timestamp"`         // epoch millis when queued/started
	URL        string                  `json:"url"`
	Artifacts  []jenkinsBuildArtifact  `json:"artifacts,omitempty"`
}

// jenkinsBuildArtifact is the per-file artifact descriptor nested in a build
// response.
type jenkinsBuildArtifact struct {
	DisplayPath  string `json:"displayPath"`
	FileName     string `json:"fileName"`
	RelativePath string `json:"relativePath"`
}
