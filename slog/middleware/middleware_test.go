package middleware_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/jefflinse/rhizome"
	slogmw "github.com/jefflinse/rhizome-contrib/slog/middleware"
)

// capture returns a JSON-backed logger whose records can be inspected.
func capture(t *testing.T, level slog.Level) (*slog.Logger, func() []map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	h := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: level})
	return slog.New(h), func() []map[string]any {
		var out []map[string]any
		for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
			if line == "" {
				continue
			}
			var m map[string]any
			if err := json.Unmarshal([]byte(line), &m); err != nil {
				t.Fatalf("unmarshal %q: %v", line, err)
			}
			out = append(out, m)
		}
		return out
	}
}

func runGraph(t *testing.T, mw rhizome.Middleware[int], failB error) (int, error) {
	t.Helper()
	g := rhizome.New[int]()
	_ = g.AddNode("a", func(_ context.Context, n int) (int, error) { return n + 1, nil })
	_ = g.AddNode("b", func(_ context.Context, n int) (int, error) {
		if failB != nil {
			return n, failB
		}
		return n * 10, nil
	})
	_ = g.AddEdge(rhizome.Start, "a")
	_ = g.AddEdge("a", "b")
	_ = g.AddEdge("b", rhizome.End)
	compiled, err := g.Compile()
	if err != nil {
		t.Fatal(err)
	}
	return compiled.Run(context.Background(), 0, rhizome.WithMiddleware(mw))
}

func TestLogsStartAndEnd(t *testing.T) {
	logger, drain := capture(t, slog.LevelDebug)

	mw := slogmw.New[int](slogmw.WithLogger(logger))

	if _, err := runGraph(t, mw, nil); err != nil {
		t.Fatalf("run: %v", err)
	}

	records := drain()
	// 2 nodes × (start + end) = 4 records
	if len(records) != 4 {
		t.Fatalf("got %d records, want 4: %#v", len(records), records)
	}

	wantMsgs := []string{"rhizome: node start", "rhizome: node end", "rhizome: node start", "rhizome: node end"}
	for i, want := range wantMsgs {
		if got, _ := records[i]["msg"].(string); got != want {
			t.Errorf("records[%d].msg = %q, want %q", i, got, want)
		}
	}
}

func TestLogsErrorAtErrorLevel(t *testing.T) {
	logger, drain := capture(t, slog.LevelDebug)
	mw := slogmw.New[int](slogmw.WithLogger(logger))

	boom := errors.New("boom")
	if _, err := runGraph(t, mw, boom); !errors.Is(err, boom) {
		t.Fatalf("run err = %v, want wrapped boom", err)
	}

	records := drain()
	// a(start+end) + b(start+error) = 4
	last := records[len(records)-1]
	if got, _ := last["msg"].(string); got != "rhizome: node error" {
		t.Fatalf("last msg = %q, want rhizome: node error", got)
	}
	if got, _ := last["level"].(string); got != "ERROR" {
		t.Fatalf("last level = %q, want ERROR", got)
	}
	if got, _ := last["rhizome.node"].(string); got != "b" {
		t.Fatalf("last node = %q, want b", got)
	}
}

func TestSuppressStart(t *testing.T) {
	logger, drain := capture(t, slog.LevelDebug)
	mw := slogmw.New[int](slogmw.WithLogger(logger), slogmw.WithoutStartRecord())

	if _, err := runGraph(t, mw, nil); err != nil {
		t.Fatalf("run: %v", err)
	}

	records := drain()
	if len(records) != 2 {
		t.Fatalf("got %d records, want 2", len(records))
	}
	for i, r := range records {
		if got, _ := r["msg"].(string); got != "rhizome: node end" {
			t.Errorf("records[%d].msg = %q, want rhizome: node end", i, got)
		}
	}
}
