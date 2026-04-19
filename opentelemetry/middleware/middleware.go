// Package middleware provides an OpenTelemetry-instrumented rhizome.Middleware
// that emits a span, a counter, and a duration histogram per node execution.
package middleware

import (
	"context"
	"fmt"
	"time"

	"github.com/jefflinse/rhizome"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	noopmetric "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/trace"
	nooptrace "go.opentelemetry.io/otel/trace/noop"
)

// Scope is the instrumentation scope used when acquiring tracers and meters.
const Scope = "github.com/jefflinse/rhizome-contrib/opentelemetry/middleware"

// Config configures the middleware. The zero value is valid and emits both
// traces and metrics using the globally registered providers.
type Config struct {
	// TracerProvider overrides the global tracer provider.
	TracerProvider trace.TracerProvider
	// MeterProvider overrides the global meter provider.
	MeterProvider metric.MeterProvider
	// TraceNamespace is the prefix for span names. Default: "rhizome".
	// Spans are named "<TraceNamespace>.node".
	TraceNamespace string
	// MetricNamespace is the prefix for metric names. Default: "rhizome".
	// Emitted metrics:
	//   - <MetricNamespace>.node.executions  (counter)
	//   - <MetricNamespace>.node.duration    (histogram, seconds)
	MetricNamespace string
	// DisableTracing skips span creation when true.
	DisableTracing bool
	// DisableMetrics skips metric emission when true.
	DisableMetrics bool
}

// Option configures Config.
type Option func(*Config)

// WithTracerProvider overrides the global tracer provider.
func WithTracerProvider(tp trace.TracerProvider) Option {
	return func(c *Config) { c.TracerProvider = tp }
}

// WithMeterProvider overrides the global meter provider.
func WithMeterProvider(mp metric.MeterProvider) Option {
	return func(c *Config) { c.MeterProvider = mp }
}

// WithTraceNamespace sets the prefix used when naming spans.
func WithTraceNamespace(ns string) Option {
	return func(c *Config) { c.TraceNamespace = ns }
}

// WithMetricNamespace sets the prefix used when naming metrics.
func WithMetricNamespace(ns string) Option {
	return func(c *Config) { c.MetricNamespace = ns }
}

// WithTracingDisabled turns span creation off.
func WithTracingDisabled() Option {
	return func(c *Config) { c.DisableTracing = true }
}

// WithMetricsDisabled turns metric emission off.
func WithMetricsDisabled() Option {
	return func(c *Config) { c.DisableMetrics = true }
}

// New builds a rhizome.Middleware[S] that instruments each node execution
// with an OpenTelemetry span plus an execution counter and duration histogram.
//
// S is the caller's graph state type. Middleware[S] is generic because
// rhizome's Middleware type is generic; the instrumentation itself does
// not inspect state.
func New[S any](opts ...Option) (rhizome.Middleware[S], error) {
	cfg := Config{}
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.TraceNamespace == "" {
		cfg.TraceNamespace = "rhizome"
	}
	if cfg.MetricNamespace == "" {
		cfg.MetricNamespace = "rhizome"
	}

	tracer := resolveTracer(cfg)
	meter := resolveMeter(cfg)

	counter, err := meter.Int64Counter(
		cfg.MetricNamespace+".node.executions",
		metric.WithDescription("Number of rhizome node executions."),
	)
	if err != nil {
		return nil, fmt.Errorf("create executions counter: %w", err)
	}
	duration, err := meter.Float64Histogram(
		cfg.MetricNamespace+".node.duration",
		metric.WithDescription("Duration of rhizome node executions."),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, fmt.Errorf("create duration histogram: %w", err)
	}

	spanName := cfg.TraceNamespace + ".node"

	return func(ctx context.Context, node string, state S, next rhizome.NodeFunc[S]) (S, error) {
		nodeAttr := attribute.String("rhizome.node", node)

		ctx, span := tracer.Start(ctx, spanName, trace.WithAttributes(nodeAttr))
		start := time.Now()

		result, err := next(ctx, state)

		elapsed := time.Since(start).Seconds()
		attrs := metric.WithAttributes(nodeAttr, attribute.Bool("error", err != nil))
		counter.Add(ctx, 1, attrs)
		duration.Record(ctx, elapsed, attrs)

		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "node returned error")
		}
		span.End()

		return result, err
	}, nil
}

func resolveTracer(cfg Config) trace.Tracer {
	if cfg.DisableTracing {
		return nooptrace.NewTracerProvider().Tracer(Scope)
	}
	tp := cfg.TracerProvider
	if tp == nil {
		tp = otel.GetTracerProvider()
	}
	return tp.Tracer(Scope)
}

func resolveMeter(cfg Config) metric.Meter {
	if cfg.DisableMetrics {
		return noopmetric.NewMeterProvider().Meter(Scope)
	}
	mp := cfg.MeterProvider
	if mp == nil {
		mp = otel.GetMeterProvider()
	}
	return mp.Meter(Scope)
}
