package gitlab

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

// ---------------------------------------------------------------------------
// client — low-level HTTP wrapper around a single GitLab instance.
// ---------------------------------------------------------------------------
//
// The client is intentionally thin: one Go method per GitLab endpoint the
// adapter needs. Caller code (adapter.go) owns the translation to the
// unified engine contract.
//
// Safety contract:
//   - Every method takes a ctx and propagates it to the underlying http.Request.
//   - Every response is read through a size-limited reader so a malicious /
//     broken server can't OOM us.
//   - Error responses (non-2xx) are mapped via mapHTTPStatus (errors.go) so
//     callers can errors.Is() against engine.Err* sentinels.
//   - Credentials (PRIVATE-TOKEN) are set per-request; they never leak into
//     error messages or the request URL.

// clientConfig is the collection of settings used to build a *client.
// Populated from models.CIEngineConfig by the adapter.
type clientConfig struct {
	Endpoint           string // https://gitlab.example.com (no trailing /api/v4)
	Token              string // PRIVATE-TOKEN value (PAT / project access token)
	InsecureSkipVerify bool
	CABundlePEM        string // optional PEM, empty = use system roots
	Timeout            time.Duration
}

// maxResponseBodyBytes caps the size of any single JSON response we will
// read. GitLab pages responses; if a caller hits this ceiling it indicates
// adapter logic should paginate instead of slurping.
const maxResponseBodyBytes = 5 * 1024 * 1024 // 5 MiB

// defaultTimeout is used when clientConfig.Timeout is zero.
const defaultTimeout = 15 * time.Second

// client is the low-level GitLab HTTP client. Instances are safe for
// concurrent use because http.Client is.
type client struct {
	cfg     clientConfig
	baseURL *url.URL // parsed endpoint + /api/v4 suffix
	httpc   *http.Client
}

// newClient builds a client from cfg. Returns an error if Endpoint is
// malformed or the CA bundle is invalid.
func newClient(cfg clientConfig) (*client, error) {
	if strings.TrimSpace(cfg.Endpoint) == "" {
		return nil, fmt.Errorf("gitlab: endpoint is required: %w", engine.ErrInvalidInput)
	}
	base, err := url.Parse(strings.TrimRight(cfg.Endpoint, "/"))
	if err != nil {
		return nil, fmt.Errorf("gitlab: invalid endpoint %q: %w", cfg.Endpoint, engine.ErrInvalidInput)
	}
	if base.Scheme == "" || base.Host == "" {
		return nil, fmt.Errorf("gitlab: endpoint must be absolute URL (got %q): %w", cfg.Endpoint, engine.ErrInvalidInput)
	}
	// Append /api/v4 exactly once; callers build relative paths against the
	// result so joining is a no-op if the user's endpoint already includes
	// the suffix.
	if !strings.HasSuffix(base.Path, "/api/v4") {
		base.Path = strings.TrimRight(base.Path, "/") + "/api/v4"
	}

	tlsCfg, err := buildTLSConfig(cfg)
	if err != nil {
		return nil, err
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}

	httpc := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig:       tlsCfg,
			ResponseHeaderTimeout: timeout,
			IdleConnTimeout:       60 * time.Second,
		},
	}
	return &client{cfg: cfg, baseURL: base, httpc: httpc}, nil
}

// buildTLSConfig honors InsecureSkipVerify and CABundlePEM per CLAUDE §10.
// InsecureSkipVerify is documented as user-opt-in; we include a comment here
// so `golangci-lint`'s G402 rule doesn't need a blanket //nolint.
func buildTLSConfig(cfg clientConfig) (*tls.Config, error) {
	//nolint:gosec // G402: the user explicitly opts into InsecureSkipVerify via CIEngineConfig.
	tlsCfg := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: cfg.InsecureSkipVerify,
	}
	if strings.TrimSpace(cfg.CABundlePEM) != "" {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM([]byte(cfg.CABundlePEM)) {
			return nil, fmt.Errorf("gitlab: CA bundle did not contain any valid certificate: %w", engine.ErrInvalidInput)
		}
		tlsCfg.RootCAs = pool
	}
	return tlsCfg, nil
}

// newRequest builds an authenticated request against the configured base URL.
// Relative path MUST NOT include /api/v4 (the client adds it).
func (c *client) newRequest(ctx context.Context, method, relPath string, body io.Reader) (*http.Request, error) {
	u := *c.baseURL
	u.Path = strings.TrimRight(u.Path, "/") + "/" + strings.TrimLeft(relPath, "/")

	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, fmt.Errorf("gitlab: build request: %w", err)
	}
	if c.cfg.Token != "" {
		req.Header.Set("PRIVATE-TOKEN", c.cfg.Token)
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

// doJSON issues a request and decodes a JSON response into out (may be nil
// to ignore the body). Non-2xx responses are converted to sentinel errors.
func (c *client) doJSON(req *http.Request, out any) error {
	resp, err := c.httpc.Do(req)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("gitlab: request cancelled: %w", err)
		}
		return fmt.Errorf("gitlab: http: %w: %w", err, engine.ErrUnavailable)
	}
	defer func() { _ = resp.Body.Close() }()

	reader := io.LimitReader(resp.Body, maxResponseBodyBytes)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Read up to 512 bytes for diagnostic context. Never surface the
		// full response body in logs, only in the wrapped error that
		// reaches the caller.
		preview := readPreview(reader, 512)
		return mapHTTPStatus(resp.StatusCode, preview)
	}

	if out == nil {
		// Drain so the connection is reusable.
		_, _ = io.Copy(io.Discard, reader)
		return nil
	}
	if err := json.NewDecoder(reader).Decode(out); err != nil {
		return fmt.Errorf("gitlab: decode response: %w", err)
	}
	return nil
}

// doRaw issues a request and returns the raw body reader for the caller to
// consume. Intended for streaming endpoints like /jobs/:id/trace.
//
// The returned io.ReadCloser wraps a size-limited reader over the underlying
// body; callers MUST close it.
func (c *client) doRaw(req *http.Request) (io.ReadCloser, error) {
	resp, err := c.httpc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gitlab: http: %w: %w", err, engine.ErrUnavailable)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		preview := readPreview(resp.Body, 512)
		_ = resp.Body.Close()
		return nil, mapHTTPStatus(resp.StatusCode, preview)
	}
	return &limitedReadCloser{
		ReadCloser: resp.Body,
		Reader:     io.LimitReader(resp.Body, maxResponseBodyBytes),
	}, nil
}

// readPreview reads up to n bytes as a short string for error diagnostics.
// Never used on success paths.
func readPreview(r io.Reader, n int64) string {
	buf, _ := io.ReadAll(io.LimitReader(r, n))
	return strings.TrimSpace(string(buf))
}

// limitedReadCloser combines io.LimitReader with the original Closer so
// callers can still close the underlying connection.
type limitedReadCloser struct {
	io.ReadCloser
	io.Reader
}

// Read prefers the size-limited reader; Close still goes to the original.
func (l *limitedReadCloser) Read(p []byte) (int, error) { return l.Reader.Read(p) }

// Close proxies to the wrapped ReadCloser.
func (l *limitedReadCloser) Close() error { return l.ReadCloser.Close() }
