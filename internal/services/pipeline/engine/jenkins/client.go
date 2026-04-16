package jenkins

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
// client — low-level HTTP wrapper around a single Jenkins controller.
// ---------------------------------------------------------------------------
//
// Behaviour contract:
//   - Every method takes a ctx and propagates it to the underlying http.Request.
//   - Responses are read through a size-limited reader (5 MiB) so a malicious /
//     broken Jenkins can't OOM the adapter.
//   - GET/HEAD requests don't need a crumb; mutating verbs automatically
//     attach the cached crumb (see crumb.go).
//   - On 403 for a mutating verb, the client drops its cached crumb and retries
//     the request ONCE — this handles crumb rotation without bothering callers.
//   - Credentials (username + api token) are set per-request Basic Auth;
//     they never leak into the URL, error messages, or cached crumb struct.

// clientConfig is the set of fields the adapter needs from a CIEngineConfig.
type clientConfig struct {
	Endpoint           string        // https://jenkins.example.com
	Username           string        // Jenkins username
	APIToken           string        // Jenkins API token (treated like a password)
	InsecureSkipVerify bool
	CABundlePEM        string        // optional PEM
	Timeout            time.Duration // per-request timeout; default 15s
}

// maxResponseBodyBytes is the JSON body cap. Jenkins pages responses that
// could exceed this (artifacts lists on large builds); callers should paginate
// when implementing such endpoints.
const maxResponseBodyBytes = 5 * 1024 * 1024

// defaultTimeout matches the GitLab adapter.
const defaultTimeout = 15 * time.Second

// client is safe for concurrent use.
type client struct {
	cfg     clientConfig
	baseURL *url.URL
	httpc   *http.Client
	crumb   *crumbCache
}

// newClient constructs a configured *client. Returns engine.ErrInvalidInput
// for obvious misconfiguration so handlers can map to HTTP 400.
func newClient(cfg clientConfig) (*client, error) {
	if strings.TrimSpace(cfg.Endpoint) == "" {
		return nil, fmt.Errorf("jenkins: endpoint is required: %w", engine.ErrInvalidInput)
	}
	base, err := url.Parse(strings.TrimRight(cfg.Endpoint, "/"))
	if err != nil {
		return nil, fmt.Errorf("jenkins: invalid endpoint %q: %w", cfg.Endpoint, engine.ErrInvalidInput)
	}
	if base.Scheme == "" || base.Host == "" {
		return nil, fmt.Errorf("jenkins: endpoint must be absolute URL (got %q): %w", cfg.Endpoint, engine.ErrInvalidInput)
	}

	tlsCfg, err := buildTLSConfig(cfg)
	if err != nil {
		return nil, err
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	return &client{
		cfg:     cfg,
		baseURL: base,
		httpc: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				TLSClientConfig:       tlsCfg,
				ResponseHeaderTimeout: timeout,
				IdleConnTimeout:       60 * time.Second,
			},
		},
		crumb: newCrumbCache(),
	}, nil
}

func buildTLSConfig(cfg clientConfig) (*tls.Config, error) {
	//nolint:gosec // G402: the user explicitly opts into InsecureSkipVerify via CIEngineConfig.
	out := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: cfg.InsecureSkipVerify,
	}
	if strings.TrimSpace(cfg.CABundlePEM) != "" {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM([]byte(cfg.CABundlePEM)) {
			return nil, fmt.Errorf("jenkins: CA bundle did not contain any valid certificate: %w", engine.ErrInvalidInput)
		}
		out.RootCAs = pool
	}
	return out, nil
}

// newRequest builds a request with Basic Auth. Relative path MUST start with "/"
// and uses whatever URL layout Jenkins expects (e.g. "/job/foo/job/bar/api/json").
// The caller is responsible for crumb handling; do(request) handles it.
func (c *client) newRequest(ctx context.Context, method, relPath string, body io.Reader) (*http.Request, error) {
	u := *c.baseURL
	// Jenkins uses raw paths; stitch them directly onto the base URL path.
	u.Path = strings.TrimRight(u.Path, "/") + ensureLeadingSlash(relPath)

	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, fmt.Errorf("jenkins: build request: %w", err)
	}
	if c.cfg.Username != "" || c.cfg.APIToken != "" {
		req.SetBasicAuth(c.cfg.Username, c.cfg.APIToken)
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	return req, nil
}

func ensureLeadingSlash(p string) string {
	if strings.HasPrefix(p, "/") {
		return p
	}
	return "/" + p
}

// doJSON dispatches a GET request and decodes the response. Non-mutating
// verbs don't require a crumb, so no retry logic is needed.
func (c *client) doJSON(req *http.Request, out any) error {
	resp, err := c.httpc.Do(req)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("jenkins: request cancelled: %w", err)
		}
		return fmt.Errorf("jenkins: http: %w: %w", err, engine.ErrUnavailable)
	}
	defer func() { _ = resp.Body.Close() }()

	reader := io.LimitReader(resp.Body, maxResponseBodyBytes)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return mapHTTPStatus(resp.StatusCode, readPreview(reader, 512))
	}
	if out == nil {
		_, _ = io.Copy(io.Discard, reader)
		return nil
	}
	if err := json.NewDecoder(reader).Decode(out); err != nil {
		return fmt.Errorf("jenkins: decode response: %w", err)
	}
	return nil
}

// doMutation issues a POST / DELETE / PUT that requires a CSRF crumb. It
// transparently attaches the cached crumb; on 403 it drops the crumb and
// retries exactly once. On any non-403 error, no retry.
//
// `bodyFactory` is required because the body has to be replayable on retry.
// Passing nil means "no body".
func (c *client) doMutation(ctx context.Context, method, relPath string, bodyFactory func() io.Reader, out any) error {
	return c.doMutationInternal(ctx, method, relPath, bodyFactory, out, false)
}

func (c *client) doMutationInternal(
	ctx context.Context,
	method, relPath string,
	bodyFactory func() io.Reader,
	out any,
	isRetry bool,
) error {
	// Acquire (possibly cached) crumb.
	crumb, field, err := c.crumb.get(ctx, c)
	if err != nil {
		// A Jenkins instance with CSRF disabled returns 404 on crumbIssuer;
		// treat that as "no crumb needed" rather than failing the operation.
		if !errors.Is(err, engine.ErrNotFound) {
			return fmt.Errorf("jenkins: obtain crumb: %w", err)
		}
		crumb, field = "", ""
	}

	var body io.Reader
	if bodyFactory != nil {
		body = bodyFactory()
	}
	req, err := c.newRequest(ctx, method, relPath, body)
	if err != nil {
		return err
	}
	if crumb != "" && field != "" {
		req.Header.Set(field, crumb)
	}

	resp, err := c.httpc.Do(req)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("jenkins: request cancelled: %w", err)
		}
		return fmt.Errorf("jenkins: http: %w: %w", err, engine.ErrUnavailable)
	}
	defer func() { _ = resp.Body.Close() }()

	// Capture Location before we consume body (for Trigger → Queue URL).
	if out != nil {
		if loc, ok := out.(*string); ok {
			*loc = resp.Header.Get("Location")
		}
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		reader := io.LimitReader(resp.Body, maxResponseBodyBytes)
		// Drain so the connection is reusable.
		_, _ = io.Copy(io.Discard, reader)
		return nil
	}

	preview := readPreview(io.LimitReader(resp.Body, maxResponseBodyBytes), 512)

	// Retry exactly once on 403 — Jenkins rotates crumbs on some events
	// (jenkins.security.ApiTokenProperty reset, controller restart…).
	if !isRetry && isCSRFError(resp.StatusCode) {
		c.crumb.invalidate()
		return c.doMutationInternal(ctx, method, relPath, bodyFactory, out, true)
	}
	return mapHTTPStatus(resp.StatusCode, preview)
}

// doRaw issues a GET and returns the raw body reader (for streaming logs).
// The caller MUST close the returned ReadCloser.
func (c *client) doRaw(req *http.Request) (io.ReadCloser, error) {
	resp, err := c.httpc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jenkins: http: %w: %w", err, engine.ErrUnavailable)
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

func readPreview(r io.Reader, n int64) string {
	buf, _ := io.ReadAll(io.LimitReader(r, n))
	return strings.TrimSpace(string(buf))
}

type limitedReadCloser struct {
	io.ReadCloser
	io.Reader
}

func (l *limitedReadCloser) Read(p []byte) (int, error) { return l.Reader.Read(p) }
func (l *limitedReadCloser) Close() error               { return l.ReadCloser.Close() }
