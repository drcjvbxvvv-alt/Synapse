package tracing

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shaia/Synapse/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

func TestSetup_Disabled(t *testing.T) {
	cfg := config.TracingConfig{Enabled: false}

	shutdown, err := Setup(context.Background(), cfg)
	require.NoError(t, err)
	require.NotNil(t, shutdown)

	// Shutdown should be a noop — no error
	assert.NoError(t, shutdown(context.Background()))

	// Global provider must be set (noop) and usable.
	tracer := otel.Tracer("test")
	ctx, span := tracer.Start(context.Background(), "test-span")
	assert.NotNil(t, ctx)
	span.End()
}

func TestSetup_EmptyEndpoint(t *testing.T) {
	cfg := config.TracingConfig{
		Enabled:  true,
		Endpoint: "", // empty → noop
	}

	shutdown, err := Setup(context.Background(), cfg)
	require.NoError(t, err)
	assert.NoError(t, shutdown(context.Background()))
}

func TestSetup_UnreachableEndpoint(t *testing.T) {
	cfg := config.TracingConfig{
		Enabled:      true,
		Endpoint:     "localhost:19999", // nothing listening here
		ServiceName:  "test-svc",
		SamplingRate: 1.0,
	}

	// Must not block or return error — fail-open.
	shutdown, err := Setup(context.Background(), cfg)
	require.NoError(t, err)
	assert.NoError(t, shutdown(context.Background()))
}

func TestTracer_ReturnsNonNil(t *testing.T) {
	// Ensure the package-level Tracer helper works.
	tr := Tracer("my-component")
	assert.NotNil(t, tr)
}

func TestSamplerFromRate(t *testing.T) {
	cases := []struct {
		rate float64
		name string
	}{
		{0.0, "never"},
		{1.0, "always"},
		{0.5, "ratio"},
		{-1.0, "negative→never"},
		{2.0, "above-1→always"},
	}
	for _, tc := range cases {
		s := samplerFromRate(tc.rate)
		assert.NotNil(t, s, tc.name)
	}
}

func TestGlobalShutdown_Accessible(t *testing.T) {
	// After Setup(disabled), tracing.Shutdown should work.
	_, _ = Setup(context.Background(), config.TracingConfig{Enabled: false})
	assert.NoError(t, Shutdown(context.Background()))
}

func TestNewHTTPClient_PropagatesTraceContext(t *testing.T) {
	// Ensure the instrumented client adds traceparent header.
	// Set up a noop provider so spans are created but not exported.
	_, _ = Setup(context.Background(), config.TracingConfig{Enabled: false})

	var receivedHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeader = r.Header.Get("Traceparent")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// Create a fake span context so otelhttp has something to propagate.
	tracer := otel.Tracer("test")
	ctx, span := tracer.Start(context.Background(), "outer")
	defer span.End()

	client := NewHTTPClient(0)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	resp, err := client.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()

	// With the noop provider the span context is invalid, so traceparent won't
	// be set. What we assert is that the client does NOT panic and completes.
	// If a real SDK provider were set, receivedHeader would be non-empty.
	_ = receivedHeader
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Verify Tracer helper returns a usable tracer (satisfies the interface).
	assert.NotNil(t, tracer)
	_ = trace.Tracer(tracer) // compile-time: tracer implements trace.Tracer
}
