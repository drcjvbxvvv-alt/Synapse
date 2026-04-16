package jenkins

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

// ExtraConfig holds Jenkins-specific settings that live inside
// CIEngineConfig.ExtraJSON.
//
// Example payload stored in DB:
//
//	{"job_path": "myFolder/myJob"}
//
// JobPath is the Jenkins job path. Nested folders are joined with "/"; the
// adapter converts path "foo/bar/baz" into URL segment "/job/foo/job/bar/job/baz".
type ExtraConfig struct {
	JobPath string `json:"job_path"`
}

// parseExtra parses ExtraJSON. An empty string yields a zero ExtraConfig —
// callers fail later if JobPath is required for the operation.
func parseExtra(raw string) (*ExtraConfig, error) {
	cfg := &ExtraConfig{}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return cfg, nil
	}
	if err := json.Unmarshal([]byte(raw), cfg); err != nil {
		return nil, fmt.Errorf("jenkins: invalid extra_json: %w: %w", err, engine.ErrInvalidInput)
	}
	return cfg, nil
}

// requireJobPath returns ErrInvalidInput when JobPath was not provided.
// Also normalises the path: strips leading/trailing slashes.
func (e *ExtraConfig) requireJobPath() (string, error) {
	if e == nil {
		return "", fmt.Errorf("jenkins: extra config missing: %w", engine.ErrInvalidInput)
	}
	p := strings.Trim(e.JobPath, "/ ")
	if p == "" {
		return "", fmt.Errorf("jenkins: job_path is required in extra_json: %w", engine.ErrInvalidInput)
	}
	return p, nil
}

// buildJobURLPath converts a slash-separated job path into Jenkins' URL form.
//
//	buildJobURLPath("foo/bar/baz") == "/job/foo/job/bar/job/baz"
//
// Empty / whitespace-only segments are dropped. The caller is expected to
// have normalised path separators via requireJobPath().
func buildJobURLPath(jobPath string) string {
	segments := strings.Split(jobPath, "/")
	var b strings.Builder
	for _, seg := range segments {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		b.WriteString("/job/")
		b.WriteString(seg)
	}
	return b.String()
}
