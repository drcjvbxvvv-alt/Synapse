package jenkins

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

// ---------------------------------------------------------------------------
// Trigger — POST /job/:path/buildWithParameters + queue-to-build resolution
// ---------------------------------------------------------------------------
//
// Flow:
//   1. POST /job/:path/buildWithParameters?<vars>  → 201 Created, `Location`
//      header points to /queue/item/:id/.
//   2. Poll /queue/item/:id/api/json every queuePollInterval until either:
//        - executable.number becomes non-zero → build was scheduled
//        - queueItem.cancelled == true        → Jenkins discarded the item
//        - queuePollTimeout elapses           → we return a pending
//          TriggerResult with RunID = "queue:<id>" so the caller can poll
//          GetRun later (it recognises the prefix).

// queuePollInterval balances responsiveness against Jenkins load.
const queuePollInterval = 500 * time.Millisecond

// queuePollTimeout bounds the time Trigger() waits for a build number.
// After this, Trigger returns RunID="queue:<id>" and the caller polls
// GetRun later. 10 s matches the default HTTP timeout.
const queuePollTimeout = 10 * time.Second

// queueRunIDPrefix marks RunIDs that are still queue items.
const queueRunIDPrefix = "queue:"

// Trigger starts a new Jenkins build.
func (a *Adapter) Trigger(ctx context.Context, req *engine.TriggerRequest) (*engine.TriggerResult, error) {
	if req == nil {
		return nil, fmt.Errorf("jenkins.Trigger: nil request: %w", engine.ErrInvalidInput)
	}
	jobPath, err := a.extra.requireJobPath()
	if err != nil {
		return nil, err
	}

	// Jenkins buildWithParameters accepts form-url-encoded parameters.
	form := url.Values{}
	for k, v := range req.Variables {
		form.Set(k, v)
	}
	encoded := form.Encode()

	// Factory produces a replayable body for retry after crumb rotation.
	bodyFactory := func() io.Reader { return strings.NewReader(encoded) }

	triggerPath := buildJobURLPath(jobPath) + "/buildWithParameters"

	var location string
	if err := a.c.doMutation(ctx, http.MethodPost, triggerPath, bodyFactory, &location); err != nil {
		return nil, fmt.Errorf("jenkins.Trigger: %w", err)
	}
	if location == "" {
		return nil, fmt.Errorf("jenkins.Trigger: no Location header on response: %w", engine.ErrUnavailable)
	}
	queueID, err := extractQueueID(location)
	if err != nil {
		return nil, err
	}

	// Try to resolve the queue item into a build number within a bounded
	// budget. If it doesn't materialise fast enough, return a queue RunID
	// and let the caller re-poll.
	buildNum, err := a.waitForBuildNumber(ctx, queueID)
	if err != nil {
		return nil, err
	}

	queuedAt := time.Now().UTC()
	if buildNum > 0 {
		return &engine.TriggerResult{
			RunID:      strconv.FormatInt(buildNum, 10),
			ExternalID: strconv.FormatInt(buildNum, 10),
			URL:        a.buildPageURL(jobPath, buildNum),
			QueuedAt:   queuedAt,
		}, nil
	}

	qid := strconv.FormatInt(queueID, 10)
	return &engine.TriggerResult{
		RunID:      queueRunIDPrefix + qid,
		ExternalID: queueRunIDPrefix + qid,
		URL:        strings.TrimRight(a.c.baseURL.String(), "/") + "/queue/item/" + qid + "/",
		QueuedAt:   queuedAt,
	}, nil
}

// buildPageURL returns the URL to the build's web page (e.g.
// "https://jenkins.example.com/job/foo/42/"), suitable for UI hyperlinks.
func (a *Adapter) buildPageURL(jobPath string, buildNum int64) string {
	return strings.TrimRight(a.c.baseURL.String(), "/") +
		buildJobURLPath(jobPath) + "/" + strconv.FormatInt(buildNum, 10) + "/"
}

// extractQueueID parses /queue/item/:id/ from the Location header. Accepts
// both absolute URLs and root-relative paths.
func extractQueueID(location string) (int64, error) {
	loc := strings.TrimRight(location, "/")
	idx := strings.LastIndex(loc, "/")
	if idx == -1 || idx == len(loc)-1 {
		return 0, fmt.Errorf("jenkins.Trigger: unparseable Location %q: %w", location, engine.ErrUnavailable)
	}
	id, err := strconv.ParseInt(loc[idx+1:], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("jenkins.Trigger: Location tail is not a number (%q): %w", location, engine.ErrUnavailable)
	}
	return id, nil
}

// waitForBuildNumber polls the queue endpoint until the build is scheduled
// or the local timeout elapses. Returns (0, nil) on timeout — the caller
// treats this as "still queued" and surfaces a queue RunID.
func (a *Adapter) waitForBuildNumber(ctx context.Context, queueID int64) (int64, error) {
	deadline, cancel := context.WithTimeout(ctx, queuePollTimeout)
	defer cancel()
	ticker := time.NewTicker(queuePollInterval)
	defer ticker.Stop()

	path := fmt.Sprintf("/queue/item/%d/api/json", queueID)
	for {
		req, err := a.c.newRequest(deadline, http.MethodGet, path, nil)
		if err != nil {
			return 0, err
		}
		var item queueItem
		if err := a.c.doJSON(req, &item); err != nil {
			return 0, fmt.Errorf("jenkins.Trigger: poll queue %d: %w", queueID, err)
		}
		if item.Cancelled {
			return 0, fmt.Errorf("jenkins.Trigger: queue item %d cancelled: %w", queueID, engine.ErrAlreadyTerminal)
		}
		if item.Executable != nil && item.Executable.Number > 0 {
			return item.Executable.Number, nil
		}
		select {
		case <-deadline.Done():
			// Timeout: return 0 so caller knows to surface queue RunID.
			return 0, nil
		case <-ticker.C:
			// next poll
		}
	}
}
