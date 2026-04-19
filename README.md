# rhizome-contrib

Third-party component implementations for [rhizome](https://github.com/jefflinse/rhizome).

## Checkpoint Stores

| Name | Description |
|------|-------------|
| [Postgres](./postgres/checkpointstore) | Distributed store backed by [jackc/pgx/v5](https://pkg.go.dev/github.com/jackc/pgx/v5). |
| [SQLite](./sqlite/checkpointstore) | Durable single-process store backed by [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) (pure Go, no CGO). |

## Middleware

| Name | Description |
|------|-------------|
| [OpenTelemetry](./opentelemetry/middleware) | Wraps node execution with OpenTelemetry traces and metrics. |
| [slog](./slog/middleware) | Logs node entry, exit, duration, and errors via `log/slog`. |

## Acceptance Tests

`CheckpointStore` implementations can verify they satisfy the rhizome contract by calling [`acceptance.CheckpointStore`](./acceptance) from their own test files.

## License

See [LICENSE](LICENSE.md).
