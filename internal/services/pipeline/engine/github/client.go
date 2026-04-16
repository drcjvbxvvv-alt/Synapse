package github

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
// client — HTTP wrapper around a single GitHub instance (github.com or GHE).
// ---------------------------------------------------------------------------
//
// Authentication: Bearer token (Personal Access Token, fine-grained PAT, or
// a GitHub App installation token). Sent as `Authorization: Bearer <token>`
// alongside the modern `X-GitHub-Api-Version` header so servers stabilise
// on the `2022-11-28` surface the adapter was written against.
//
// Enterprise deployments override Endpoint to something like
// `https://github.example.com/api/v3`. The adapter appends `/api/v3` when
// the caller passes a bare `https://github.example.com` and leaves it alone
// if the suffix is already present. For github.com the default endpoint
// (set by NewAdapter) is `https://api.github.com`.

const (
	maxResponseBodyBytes = 5 * 1024 * 1024
	defaultTimeout       = 15 * time.Second
	apiVersion           = "2022-11-28"
)

// clientConfig bundles the fields the adapter reads from CIEngineConfig.
type clientConfig struct {
	Endpoint           string        // https://api.github.com (public) or https://github.example.com (GHE)
	Token              string        // PAT or installation token
	InsecureSkipVerify bool
	CABundlePEM        string
	Timeout            time.Duration
}

// client is safe for concurrent use.
type client struct {
	cfg     clientConfig
	baseURL *url.URL
	httpc   *http.Client
}

// newClient constructs a client. Returns engine.ErrInvalidInput for obvious
// misconfiguration.
func newClient(cfg clientConfig) (*client, error) {
	endpoint := strings.TrimSpace(cfg.Endpoint)
	if endpoint == "" {
		endpoint = "https://api.github.com" // public default
	}
	base, err := url.Parse(strings.TrimRight(endpoint, "/"))
	if err != nil {
		return nil, fmt.Errorf("github: invalid endpoint %q: %w", cfg.Endpoint, engine.ErrInvalidInput)
	}
	if base.Scheme == "" || base.Host == "" {
		return nil, fmt.Errorf("github: endpoint must be absolute URL (got %q): %w", cfg.Endpoint, engine.ErrInvalidInput)
	}
	// GHE: ensure /api/v3 prefix. Public api.github.com already has the
	// correct root.
	if base.Host != "api.github.com" {
		if !strings.HasSuffix(base.Path, "/api/v3") {
			base.Path = strings.TrimRight(base.Path, "/") + "/api/v3"
		}
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
	}, nil
}

func buildTLSConfig(cfg clientConfig) (*tls.Config, error) {
	//nolint:gosec // G402: the user explicitly opts into InsecureSkipVerify via CIEngineConfig.
	tlsCfg := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: cfg.InsecureSkipVerify,
	}
	if strings.TrimSpace(cfg.CABundlePEM) != "" {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM([]byte(cfg.CABundlePEM)) {
			return nil, fmt.Errorf("github: CA bundle did not contain any valid certificate: %w", engine.ErrInvalidInput)
		}
		tlsCfg.RootCAs = pool
	}
	return tlsCfg, nil
}

// newRequest builds an authenticated request. relPath MUST start with "/"
// and may include a `?query=string` suffix; the suffix is correctly
// separated from the path before being assigned to *url.URL.
func (c *client) newRequest(ctx context.Context, method, relPath string, body io.Reader) (*http.Request, error) {
	u := *c.baseURL
	p, q := splitPathAndQuery(relPath)
	u.Path = strings.TrimRight(u.Path, "/") + ensureLeadingSlash(p)
	if q != "" {
		u.RawQuery = q
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, fmt.Errorf("github: build request: %w", err)
	}
	if c.cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.Token)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", apiVersion)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

func ensureLeadingSlash(p string) string {
	if strings.HasPrefix(p, "/") {
		return p
	}
	return "/" + p
}

// splitPathAndQuery separates a relative URL of the form "/foo?a=1" into
// ("/foo", "a=1"). Inputs without a `?` return an empty query. This lets
// call sites keep the convenient fmt.Sprintf("/repos/%s/%s/runs?per_page=100")
// pattern without manually assembling url.Values.
func splitPathAndQuery(s string) (path, query string) {
	if i := strings.IndexByte(s, '?'); i >= 0 {
		return s[:i], s[i+1:]
	}
	return s, ""
}

// doJSON dispatches a request and decodes the response into out (nil to
// ignore). Non-2xx are mapped to sentinel errors via mapHTTPStatus.
func (c *client) doJSON(req *http.Request, out any) error {
	resp, err := c.httpc.Do(req)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("github: request cancelled: %w", err)
		}
		return fmt.Errorf("github: http: %w: %w", err, engine.ErrUnavailable)
	}
	defer func() { _ = resp.Body.Close() }()

	reader := io.LimitReader(resp.Body, maxResponseBodyBytes)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return mapHTTPStatus(resp.StatusCode, readPreview(reader, 512))
	}
	if out == nil || resp.StatusCode == http.StatusNoContent {
		_, _ = io.Copy(io.Discard, reader)
		return nil
	}
	if err := json.NewDecoder(reader).Decode(out); err != nil {
		return fmt.Errorf("github: decode response: %w", err)
	}
	return nil
}

// doRaw streams the body (for log downloads). Caller closes the reader.
func (c *client) doRaw(req *http.Request) (io.ReadCloser, error) {
	resp, err := c.httpc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github: http: %w: %w", err, engine.ErrUnavailable)
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
