package checkpointstore_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/jefflinse/rhizome"
	"github.com/jefflinse/rhizome-contrib/acceptance"
	sqlitestore "github.com/jefflinse/rhizome-contrib/sqlite/checkpointstore"
)

func TestAcceptance_InMemory(t *testing.T) {
	acceptance.CheckpointStore(t, func(t *testing.T) (rhizome.CheckpointStore, func()) {
		store, err := sqlitestore.Open(context.Background(), ":memory:")
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		return store, func() { _ = store.Close() }
	})
}

func TestAcceptance_FileBacked(t *testing.T) {
	acceptance.CheckpointStore(t, func(t *testing.T) (rhizome.CheckpointStore, func()) {
		dsn := "file:" + filepath.Join(t.TempDir(), "ck.db") + "?_pragma=journal_mode(WAL)"
		store, err := sqlitestore.Open(context.Background(), dsn)
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		return store, func() { _ = store.Close() }
	})
}

func TestCustomTable(t *testing.T) {
	store, err := sqlitestore.Open(context.Background(), ":memory:",
		sqlitestore.WithTableName("custom_checkpoints"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	if err := store.Save(ctx, "t1", "n1", []byte("x")); err != nil {
		t.Fatalf("save: %v", err)
	}
	node, data, err := store.Load(ctx, "t1")
	if err != nil || node != "n1" || string(data) != "x" {
		t.Fatalf("load: (%q, %q, %v)", node, data, err)
	}
}
