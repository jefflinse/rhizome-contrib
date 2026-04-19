# SQLite CheckpointStore

A [`rhizome.CheckpointStore`](https://pkg.go.dev/github.com/jefflinse/rhizome#CheckpointStore) backed by SQLite via [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) (pure Go, no CGO).

## Usage

```go
import (
    "context"

    "github.com/jefflinse/rhizome"
    sqlitestore "github.com/jefflinse/rhizome-contrib/sqlite/checkpointstore"
)

store, err := sqlitestore.Open(ctx, "file:checkpoints.db?_pragma=journal_mode(WAL)")
if err != nil {
    panic(err)
}
defer store.Close()

compiled, err := g.Compile(rhizome.WithCheckpointing(store))
```

Alternatively, wrap an existing `*sql.DB` with `New`; in that case the caller owns the database handle.

## Schema

A single table (default name `rhizome_checkpoints`) stores one row per thread. Saves upsert by `thread_id`, so `Load` always returns the most recent checkpoint.

```sql
CREATE TABLE rhizome_checkpoints (
    thread_id  TEXT PRIMARY KEY,
    node_name  TEXT NOT NULL,
    data       BLOB NOT NULL,
    updated_at INTEGER NOT NULL
);
```

Use `WithTableName` to customize the table name when sharing a database with other schemas.
