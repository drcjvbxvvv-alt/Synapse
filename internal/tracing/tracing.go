// Package tracing initialises the OpenTelemetry SDK and exposes helpers for
// distributed tracing across the Synapse backend.
//
// Usage:
//
//	shutdown, err := tracing.Setup(ctx, cfg.Tracing)
//	defer shutdown(context.Background())
//
// When cfg.Tracing.Enabled is false (or the OTLP endpoint is empty) a no-op
// provider is installed so all tracer calls are safe but produce no spans.
package tracing

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/shaia/Synapse/internal/config"
	"github.com/shaia/Synapse/pkg/logger"
)


// ShutdownFunc must be called on application exit to flush and close the
// TracerProvider.
type ShutdownFunc func(context.Context) error

// globalShutdown holds the registered shutdown function so callers can invoke
// tracing.Shutdown(ctx) without keeping a reference to the original func.
var globalShutdown ShutdownFunc = func(context.Context) error { return nil }

// Shutdown flushes buffered spans and closes the OTLP connection.
// Call this during graceful shutdown (after HTTP server stops).
func Shutdown(ctx context.Context) error {
	return globalShutdown(ctx)
}

// Setup initialises the global OpenTelemetry TracerProvider and text-map
// propagator. It stores the shutdown function so callers can use tracing.Shutdown().
// It returns the ShutdownFunc for callers that prefer explicit lifecycle management.
//
// When tracing is disabled or the endpoint is empty, a no-op provider is used.
func Setup(ctx context.Context, cfg config.TracingConfig) (ShutdownFunc, error) {
	if !cfg.Enabled || cfg.Endpoint == "" {
		logger.Info("tracing: disabled (OTEL_ENABLED=false or endpoint not set)")
		otel.SetTracerProvider(noop.NewTracerProvider())
		noop := func(context.Context) error { return nil }
		globalShutdown = noop
		return noop, nil
	}

	// ── Resource ──────────────────────────────────────────────────────────
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("tracing: create resource: %w", err)
	}

	// ── OTLP gRPC exporter ────────────────────────────────────────────────
	// Use a short context just for the exporter creation; the connection
	// itself is non-blocking — the SDK handles reconnect internally.
	exportCtx, exportCancel := context.WithTimeout(ctx, 5*time.Second)
	defer exportCancel()

	exporter, err := otlptracegrpc.New(exportCtx,
		otlptracegrpc.WithEndpoint(cfg.Endpoint),
		otlptracegrpc.WithDialOption(
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		),
	)
	if err != nil {
		// Fail-open: warn and use noop rather than preventing startup.
		logger.Warn("tracing: could not create OTLP exporter, using noop",
			"endpoint", cfg.Endpoint, "error", err)
		otel.SetTracerProvider(noop.NewTracerProvider())
		noopFn := func(context.Context) error { return nil }
		globalShutdown = noopFn
		return noopFn, nil
	}

	// ── Sampler ───────────────────────────────────────────────────────────
	sampler := samplerFromRate(cfg.SamplingRate)

	// ── TracerProvider ────────────────────────────────────────────────────
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)

	// ── Global registration ───────────────────────────────────────────────
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, // W3C traceparent / tracestate
		propagation.Baggage{},
	))

	logger.Info("tracing: initialised",
		"endpoint", cfg.Endpoint,
		"service", cfg.ServiceName,
		"sampling_rate", cfg.SamplingRate,
	)

	shutdown := func(ctx context.Context) error {
		return tp.Shutdown(ctx)
	}
	globalShutdown = shutdown
	return shutdown, nil
}

// Tracer returns a named tracer from the global provider.
// Components that want to create custom spans use this instead of
// otel.Tracer() directly, so the import path stays internal.
func Tracer(name string) trace.Tracer {
	return otel.Tracer(name)
}

// samplerFromRate returns a sampler for a 0.0–1.0 rate.
// 1.0 = always sample, 0.0 = never sample, anything else = probabilistic.
func samplerFromRate(rate float64) sdktrace.Sampler {
	switch {
	case rate <= 0:
		return sdktrace.NeverSample()
	case rate >= 1:
		return sdktrace.AlwaysSample()
	default:
		return sdktrace.TraceIDRatioBased(rate)
	}
}
