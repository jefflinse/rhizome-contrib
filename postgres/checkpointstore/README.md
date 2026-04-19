# Postgres CheckpointStore

A [`rhizome.CheckpointStore`](https://pkg.go.dev/github.com/jefflinse/rhizome#CheckpointStore) backed by Postgres via [jackc/pgx/v5](https://pkg.go.dev/github.com/jackc/pgx/v5).

## Usage

```go
import (
    "context"

    "github.com/jefflinse/rhizome"
    pgstore "github.com/jefflinse/rhizome-contrib/postgres/checkpointstore"
)

store, err := pgstore.Open(ctx, "postgres://user:pass@localhost:5432/app?sslmode=disable")
if err != nil {
    panic(err)
}
defer store.Close()

compiled, err := g.Compile(rhizome.WithCheckpointing(store))
```

Alternatively, wrap an existing `*pgxpool.Pool` with `New`; in that case the caller owns the pool.

## Schema

A single table (default name `rhizome_checkpoints`) stores one row per thread. Saves upsert by `thread_id`, so `Load` always returns the most recent checkpoint.

```sql
CREATE TABLE rhizome_checkpoints (
    thread_id  TEXT PRIMARY KEY,
    node_name  TEXT NOT NULL,
    data       BYTEA NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);
```

Use `WithTableName` to customize the table name when sharing a database with other schemas.

## Tests

Integration tests use [testcontainers-go](https://github.com/testcontainers/testcontainers-go) to spin up a disposable Postgres. Set `POSTGRES_TEST_DSN` to run against an existing database instead. Tests are skipped when Docker is unavailable and no DSN is provided.
