# OpenTelemetry Middleware

OpenTelemetry-instrumented [`rhizome.Middleware`](https://pkg.go.dev/github.com/jefflinse/rhizome#Middleware). Each node execution emits:

- A span named `rhizome.node` with `rhizome.node=<name>` attribute
- A counter `rhizome.node.executions` with `rhizome.node` and `error` attributes
- A histogram `rhizome.node.duration` (seconds) with the same attributes

Errors returned from a node are recorded on the span and set its status to `Error`.

## Usage

```go
import (
    "github.com/jefflinse/rhizome"
    otelmw "github.com/jefflinse/rhizome-contrib/opentelemetry/middleware"
)

mw, err := otelmw.New[*State]()
if err != nil {
    panic(err)
}

result, err := compiled.Run(ctx, initial, rhizome.WithMiddleware(mw))
```

By default the middleware uses the globally registered tracer and meter providers. Override with `WithTracerProvider` / `WithMeterProvider`, or disable either signal with `WithTracingDisabled` / `WithMetricsDisabled`.

Span and metric name prefixes default to `rhizome` and can be changed with `WithTraceNamespace` and `WithMetricNamespace`.
