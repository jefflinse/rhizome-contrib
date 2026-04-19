package checkpointstore_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jefflinse/rhizome"
	"github.com/jefflinse/rhizome-contrib/acceptance"
	pgstore "github.com/jefflinse/rhizome-contrib/postgres/checkpointstore"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// connString returns a Postgres DSN suitable for the test run.
//
// Preference order:
//  1. POSTGRES_TEST_DSN environment variable (e.g., a locally running DB)
//  2. A testcontainers-managed container
//
// When neither is available (e.g., Docker is unavailable), the test is skipped.
func connString(t *testing.T) string {
	t.Helper()

	if dsn := os.Getenv("POSTGRES_TEST_DSN"); dsn != "" {
		return dsn
	}

	ctx := context.Background()
	container, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("rhizome_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second)),
	)
	if err != nil {
		t.Skipf("testcontainers unavailable: %v", err)
	}
	t.Cleanup(func() {
		_ = container.Terminate(context.Background())
	})

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}
	return dsn
}

func TestAcceptance(t *testing.T) {
	dsn := connString(t)

	// One pool shared across subtests; each subtest uses a distinct table
	// so rows do not leak between cases.
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	t.Cleanup(pool.Close)

	acceptance.CheckpointStore(t, func(t *testing.T) (rhizome.CheckpointStore, func()) {
		table := "ck_" + sanitize(t.Name())
		store, err := pgstore.New(context.Background(), pool, pgstore.WithTableName(table))
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		return store, func() {
			_, _ = pool.Exec(context.Background(), `DROP TABLE IF EXISTS "`+table+`"`)
		}
	})
}

// sanitize maps test names like "SaveThenLoadRoundTrip" or
// "TestAcceptance/SaveThenLoad" to valid table-name suffixes.
func sanitize(name string) string {
	out := make([]byte, 0, len(name))
	for i := 0; i < len(name); i++ {
		c := name[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
			out = append(out, c)
		default:
			out = append(out, '_')
		}
	}
	return string(out)
}
