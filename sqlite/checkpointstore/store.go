// Package checkpointstore provides a rhizome.CheckpointStore backed by SQLite.
//
// The store uses a single table keyed by thread_id; each Save upserts the
// row so Load always returns the most recent checkpoint for a thread.
//
// The driver is modernc.org/sqlite (pure Go, no CGO required).
package checkpointstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jefflinse/rhizome"

	_ "modernc.org/sqlite"
)

// DefaultTableName is the table used when WithTableName is not supplied.
const DefaultTableName = "rhizome_checkpoints"

// Store is a rhizome.CheckpointStore backed by SQLite.
type Store struct {
	db    *sql.DB
	table string
}

// Option configures a Store during construction.
type Option func(*Store)

// WithTableName overrides the default table name.
func WithTableName(name string) Option {
	return func(s *Store) {
		s.table = name
	}
}

// Open creates a new SQLite-backed Store by opening dsn with the "sqlite"
// driver. The returned Store owns the *sql.DB; close it with Store.Close.
// Common DSNs: ":memory:", "file:checkpoints.db?_pragma=journal_mode(WAL)".
//
// Open configures the connection pool to a single open connection. This is
// required for in-memory DSNs (where each new connection is an isolated
// database) and avoids SQLITE_BUSY errors under concurrent writers for
// file-backed DSNs. Callers who want different pool behavior should
// construct a *sql.DB themselves and use New.
func Open(ctx context.Context, dsn string, opts ...Option) (*Store, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	s, err := New(ctx, db, opts...)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// New wraps an existing *sql.DB. The caller retains ownership of db.
// The schema is created if it does not already exist.
func New(ctx context.Context, db *sql.DB, opts ...Option) (*Store, error) {
	if db == nil {
		return nil, errors.New("rhizome-contrib/sqlite: db is required")
	}
	s := &Store{db: db, table: DefaultTableName}
	for _, opt := range opts {
		opt(s)
	}
	if err := s.ensureSchema(ctx); err != nil {
		return nil, err
	}
	return s, nil
}

// Close closes the underlying database if Store owns it (i.e., it was
// created via Open). When constructed via New, the caller owns db and
// Close is a no-op on the db.
//
// Close is always safe to call; it releases the handle this Store holds.
func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) ensureSchema(ctx context.Context) error {
	stmt := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %q (
	thread_id TEXT PRIMARY KEY,
	node_name TEXT NOT NULL,
	data BLOB NOT NULL,
	updated_at INTEGER NOT NULL DEFAULT (strftime('%%s', 'now'))
)`, s.table)
	if _, err := s.db.ExecContext(ctx, stmt); err != nil {
		return fmt.Errorf("create schema: %w", err)
	}
	return nil
}

// Save upserts the checkpoint for threadID. It is safe for concurrent use.
func (s *Store) Save(ctx context.Context, threadID, nodeName string, data []byte) error {
	stmt := fmt.Sprintf(`INSERT INTO %q (thread_id, node_name, data, updated_at)
VALUES (?, ?, ?, strftime('%%s', 'now'))
ON CONFLICT(thread_id) DO UPDATE SET
	node_name = excluded.node_name,
	data = excluded.data,
	updated_at = excluded.updated_at`, s.table)
	if _, err := s.db.ExecContext(ctx, stmt, threadID, nodeName, data); err != nil {
		return fmt.Errorf("save checkpoint: %w", err)
	}
	return nil
}

// Load returns the latest checkpoint for threadID, or rhizome.ErrNoCheckpoint
// if no checkpoint exists.
func (s *Store) Load(ctx context.Context, threadID string) (string, []byte, error) {
	stmt := fmt.Sprintf(`SELECT node_name, data FROM %q WHERE thread_id = ?`, s.table)
	var node string
	var data []byte
	err := s.db.QueryRowContext(ctx, stmt, threadID).Scan(&node, &data)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil, rhizome.ErrNoCheckpoint
	}
	if err != nil {
		return "", nil, fmt.Errorf("load checkpoint: %w", err)
	}
	return node, data, nil
}

var _ rhizome.CheckpointStore = (*Store)(nil)
