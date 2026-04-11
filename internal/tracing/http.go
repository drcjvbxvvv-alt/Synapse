package tracing

import (
	"net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// NewHTTPClient returns an *http.Client whose transport automatically creates
// OTel spans for every outbound request and propagates trace context headers
// (W3C traceparent).
//
// Use this for all external HTTP calls: Prometheus, Grafana, ArgoCD, etc.
//
//	client := tracing.NewHTTPClient(30 * time.Second)
//	resp, err := client.Get("http://prometheus:9090/api/v1/query")
func NewHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout:   timeout,
		Transport: otelhttp.NewTransport(http.DefaultTransport),
	}
}
