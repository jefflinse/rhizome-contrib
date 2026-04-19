# slog Middleware

A [`rhizome.Middleware`](https://pkg.go.dev/github.com/jefflinse/rhizome#Middleware) that logs node execution via `log/slog`.

For each node the middleware emits:

- `rhizome: node start` at `LevelDebug` (suppressable)
- `rhizome: node end` at `LevelDebug` — includes `rhizome.duration`
- `rhizome: node error` at `LevelError` — includes `rhizome.duration` and `error`

Every record carries a `rhizome.node=<name>` attribute.

## Usage

```go
import (
    "log/slog"

    "github.com/jefflinse/rhizome"
    slogmw "github.com/jefflinse/rhizome-contrib/slog/middleware"
)

mw := slogmw.New[*State](slogmw.WithLogger(slog.Default()))

result, err := compiled.Run(ctx, initial, rhizome.WithMiddleware(mw))
```

Levels are tunable via `WithStartLevel`, `WithEndLevel`, and `WithErrorLevel`. Pass `WithoutStartRecord()` to emit only the end record.
