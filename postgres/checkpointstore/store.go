// Package checkpointstore provides a rhizome.CheckpointStore backed by Postgres.
//
// The store uses a single table keyed by thread_id; each Save upserts the
// row so Load always returns the most recent checkpoint for a thread.
package checkpointstore

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jefflinse/rhizome"
)

// DefaultTableName is the table used when WithTableName is not supplied.
const DefaultTableName = "rhizome_checkpoints"

// Store is a rhizome.CheckpointStore backed by Postgres.
type Store struct {
	pool  *pgxpool.Pool
	table string
}

// Option configures a Store during construction.
type Option func(*Store)

// WithTableName overrides the default table name. The name is emitted into
// DDL and DML via identifier quoting; callers may pass schema-qualified
// names like "public.checkpoints" by passing the two parts pre-quoted.
func WithTableName(name string) Option {
	return func(s *Store) {
		s.table = name
	}
}

// Open creates a new Postgres-backed Store by opening connString with pgxpool.
// The returned Store owns the pool; close it with Store.Close.
func Open(ctx context.Context, connString string, opts ...Option) (*Store, error) {
	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		return nil, fmt.Errorf("open pgxpool: %w", err)
	}
	s, err := New(ctx, pool, opts...)
	if err != nil {
		pool.Close()
		return nil, err
	}
	return s, nil
}

// New wraps an existing *pgxpool.Pool. The caller retains ownership of pool.
// The schema is created if it does not already exist.
func New(ctx context.Context, pool *pgxpool.Pool, opts ...Option) (*Store, error) {
	if pool == nil {
		return nil, errors.New("rhizome-contrib/postgres: pool is required")
	}
	s := &Store{pool: pool, table: DefaultTableName}
	for _, opt := range opts {
		opt(s)
	}
	if err := s.ensureSchema(ctx); err != nil {
		return nil, err
	}
	return s, nil
}

// Close releases the handle this Store holds. When constructed via New the
// caller still owns the pool; callers of New should close the pool themselves.
func (s *Store) Close() {
	s.pool.Close()
}

func (s *Store) ensureSchema(ctx context.Context) error {
	stmt := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
	thread_id  TEXT PRIMARY KEY,
	node_name  TEXT NOT NULL,
	data       BYTEA NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
)`, quoteIdent(s.table))
	if _, err := s.pool.Exec(ctx, stmt); err != nil {
		return fmt.Errorf("create schema: %w", err)
	}
	return nil
}

// Save upserts the checkpoint for threadID. It is safe for concurrent use.
func (s *Store) Save(ctx context.Context, threadID, nodeName string, data []byte) error {
	stmt := fmt.Sprintf(`INSERT INTO %s (thread_id, node_name, data, updated_at)
VALUES ($1, $2, $3, now())
ON CONFLICT (thread_id) DO UPDATE SET
	node_name = EXCLUDED.node_name,
	data = EXCLUDED.data,
	updated_at = EXCLUDED.updated_at`, quoteIdent(s.table))
	if _, err := s.pool.Exec(ctx, stmt, threadID, nodeName, data); err != nil {
		return fmt.Errorf("save checkpoint: %w", err)
	}
	return nil
}

// Load returns the latest checkpoint for threadID, or rhizome.ErrNoCheckpoint
// if no checkpoint exists.
func (s *Store) Load(ctx context.Context, threadID string) (string, []byte, error) {
	stmt := fmt.Sprintf(`SELECT node_name, data FROM %s WHERE thread_id = $1`, quoteIdent(s.table))
	var node string
	var data []byte
	err := s.pool.QueryRow(ctx, stmt, threadID).Scan(&node, &data)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil, rhizome.ErrNoCheckpoint
	}
	if err != nil {
		return "", nil, fmt.Errorf("load checkpoint: %w", err)
	}
	return node, data, nil
}

// quoteIdent wraps a table name in double quotes for safe emission into
// DDL/DML. A caller-supplied name containing a schema qualifier ("a.b")
// is passed through verbatim so callers that want schema.table can do so
// by pre-quoting each part.
func quoteIdent(name string) string {
	for i := 0; i < len(name); i++ {
		if name[i] == '"' || name[i] == '.' {
			return name
		}
	}
	return `"` + name + `"`
}

var _ rhizome.CheckpointStore = (*Store)(nil)
