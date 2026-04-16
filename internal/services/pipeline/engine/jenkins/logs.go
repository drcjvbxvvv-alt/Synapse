package jenkins

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

// StreamLogs returns the console output for a Jenkins build.
//
// Jenkins unit of log is the **build**, not stages within it (retrieving
// per-stage logs requires the Pipeline-Stage-View plugin API which varies
// by installation). For M18c the adapter returns the full build log; the
// `stepID` parameter is accepted for interface symmetry but ignored.
//
// Endpoint: /job/:path/:num/logText/progressiveText?start=0 — Jenkins
// returns `text/plain` with custom headers:
//   - `X-Text-Size`: current log length in bytes
//   - `X-More-Data`: "true" while the log is still growing
//
// Incremental fetching via `start=<X-Text-Size>` is a future enhancement
// (M18c follow-up). Current implementation returns a snapshot.
//
// runID: must be the build number. Queue-prefixed IDs return
// ErrInvalidInput because queued items have no log yet.
func (a *Adapter) StreamLogs(ctx context.Context, runID, stepID string) (io.ReadCloser, error) {
	_ = stepID // intentionally unused — Jenkins logs are per-build, not per-stage
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return nil, fmt.Errorf("jenkins.StreamLogs: empty run id: %w", engine.ErrInvalidInput)
	}
	if strings.HasPrefix(runID, queueRunIDPrefix) {
		return nil, fmt.Errorf("jenkins.StreamLogs: build not started (queue id): %w", engine.ErrInvalidInput)
	}
	jobPath, err := a.extra.requireJobPath()
	if err != nil {
		return nil, err
	}
	buildNum, err := strconv.ParseInt(runID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("jenkins.StreamLogs: runID %q is not a build number: %w", runID, engine.ErrInvalidInput)
	}

	path := buildJobURLPath(jobPath) + "/" + strconv.FormatInt(buildNum, 10) + "/logText/progressiveText?start=0"
	req, err := a.c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("jenkins.StreamLogs: %w", err)
	}
	req.Header.Set("Accept", "text/plain")

	rc, err := a.c.doRaw(req)
	if err != nil {
		return nil, fmt.Errorf("jenkins.StreamLogs %d: %w", buildNum, err)
	}
	return rc, nil
}
