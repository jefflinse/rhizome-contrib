package middleware_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jefflinse/rhizome"
	otelmw "github.com/jefflinse/rhizome-contrib/opentelemetry/middleware"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// runGraph builds a two-node graph, wires the middleware, and runs it.
// If failB is non-nil, node "b" returns that error.
func runGraph(t *testing.T, mw rhizome.Middleware[int], failB error) (int, error) {
	t.Helper()
	g := rhizome.New[int]()
	if err := g.AddNode("a", func(_ context.Context, n int) (int, error) {
		return n + 1, nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := g.AddNode("b", func(_ context.Context, n int) (int, error) {
		if failB != nil {
			return n, failB
		}
		return n * 10, nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := g.AddEdge(rhizome.Start, "a"); err != nil {
		t.Fatal(err)
	}
	if err := g.AddEdge("a", "b"); err != nil {
		t.Fatal(err)
	}
	if err := g.AddEdge("b", rhizome.End); err != nil {
		t.Fatal(err)
	}
	compiled, err := g.Compile()
	if err != nil {
		t.Fatal(err)
	}
	return compiled.Run(context.Background(), 0, rhizome.WithMiddleware(mw))
}

func TestEmitsSpansAndMetrics(t *testing.T) {
	spanRec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spanRec))
	reader := metric.NewManualReader()
	mp := metric.NewMeterProvider(metric.WithReader(reader))

	mw, err := otelmw.New[int](
		otelmw.WithTracerProvider(tp),
		otelmw.WithMeterProvider(mp),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result, err := runGraph(t, mw, nil)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result != 10 {
		t.Fatalf("result = %d, want 10", result)
	}

	spans := spanRec.Ended()
	if len(spans) != 2 {
		t.Fatalf("got %d spans, want 2", len(spans))
	}
	for _, s := range spans {
		if s.Name() != "rhizome.node" {
			t.Errorf("span name = %q, want rhizome.node", s.Name())
		}
	}

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("collect: %v", err)
	}

	seen := map[string]bool{}
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			seen[m.Name] = true
		}
	}
	for _, want := range []string{"rhizome.node.executions", "rhizome.node.duration"} {
		if !seen[want] {
			t.Errorf("missing metric %q (saw %v)", want, seen)
		}
	}
}

func TestErrorMarksSpan(t *testing.T) {
	spanRec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spanRec))

	mw, err := otelmw.New[int](
		otelmw.WithTracerProvider(tp),
		otelmw.WithMetricsDisabled(),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	boom := errors.New("boom")
	_, runErr := runGraph(t, mw, boom)
	if !errors.Is(runErr, boom) {
		t.Fatalf("run err = %v, want wrapped boom", runErr)
	}

	spans := spanRec.Ended()
	if len(spans) != 2 {
		t.Fatalf("got %d spans, want 2", len(spans))
	}
	// The second span corresponds to node "b" and should be in error state.
	b := spans[1]
	if b.Status().Code.String() != "Error" {
		t.Errorf("span status = %v, want Error", b.Status().Code)
	}
	if len(b.Events()) == 0 {
		t.Errorf("expected span to record an exception event")
	}
}
