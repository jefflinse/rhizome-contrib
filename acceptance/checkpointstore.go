// Package acceptance provides reusable test suites that verify a contrib
// implementation satisfies a rhizome interface contract.
package acceptance

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/jefflinse/rhizome"
)

// CheckpointStoreFactory returns a fresh, empty store for one test case.
// The returned cleanup function is invoked after the test completes.
type CheckpointStoreFactory func(t *testing.T) (store rhizome.CheckpointStore, cleanup func())

// CheckpointStore runs the standard rhizome.CheckpointStore contract tests
// against the implementation produced by newStore. Each subtest calls
// newStore to obtain an isolated, empty store.
func CheckpointStore(t *testing.T, newStore CheckpointStoreFactory) {
	t.Helper()

	t.Run("LoadMissingThreadReturnsErrNoCheckpoint", func(t *testing.T) {
		store, cleanup := newStore(t)
		defer cleanup()

		_, _, err := store.Load(context.Background(), "nope")
		if !errors.Is(err, rhizome.ErrNoCheckpoint) {
			t.Fatalf("Load: got %v, want ErrNoCheckpoint", err)
		}
	})

	t.Run("SaveThenLoadRoundTrip", func(t *testing.T) {
		store, cleanup := newStore(t)
		defer cleanup()

		ctx := context.Background()
		want := []byte(`{"values":["a"]}`)
		if err := store.Save(ctx, "t1", "a", want); err != nil {
			t.Fatalf("Save: %v", err)
		}

		node, got, err := store.Load(ctx, "t1")
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if node != "a" {
			t.Fatalf("node = %q, want %q", node, "a")
		}
		if string(got) != string(want) {
			t.Fatalf("data = %q, want %q", got, want)
		}
	})

	t.Run("LatestSaveWins", func(t *testing.T) {
		store, cleanup := newStore(t)
		defer cleanup()

		ctx := context.Background()
		if err := store.Save(ctx, "t1", "a", []byte("first")); err != nil {
			t.Fatalf("Save a: %v", err)
		}
		if err := store.Save(ctx, "t1", "b", []byte("second")); err != nil {
			t.Fatalf("Save b: %v", err)
		}

		node, data, err := store.Load(ctx, "t1")
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if node != "b" || string(data) != "second" {
			t.Fatalf("got (%q, %q), want (%q, %q)", node, data, "b", "second")
		}
	})

	t.Run("ThreadsAreIsolated", func(t *testing.T) {
		store, cleanup := newStore(t)
		defer cleanup()

		ctx := context.Background()
		if err := store.Save(ctx, "alpha", "n1", []byte("A")); err != nil {
			t.Fatalf("Save alpha: %v", err)
		}
		if err := store.Save(ctx, "beta", "n2", []byte("B")); err != nil {
			t.Fatalf("Save beta: %v", err)
		}

		node, data, err := store.Load(ctx, "alpha")
		if err != nil || node != "n1" || string(data) != "A" {
			t.Fatalf("alpha: (%q, %q, %v), want (n1, A, nil)", node, data, err)
		}
		node, data, err = store.Load(ctx, "beta")
		if err != nil || node != "n2" || string(data) != "B" {
			t.Fatalf("beta: (%q, %q, %v), want (n2, B, nil)", node, data, err)
		}
	})

	t.Run("LoadReturnsCopy", func(t *testing.T) {
		store, cleanup := newStore(t)
		defer cleanup()

		ctx := context.Background()
		original := []byte("hello")
		if err := store.Save(ctx, "t1", "n", original); err != nil {
			t.Fatalf("Save: %v", err)
		}

		_, got, err := store.Load(ctx, "t1")
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		got[0] = 'X'

		_, again, err := store.Load(ctx, "t1")
		if err != nil {
			t.Fatalf("Load again: %v", err)
		}
		if string(again) != "hello" {
			t.Fatalf("mutating returned slice corrupted the store: got %q, want %q", again, "hello")
		}
	})

	t.Run("ConcurrentSaves", func(t *testing.T) {
		store, cleanup := newStore(t)
		defer cleanup()

		ctx := context.Background()
		const threads = 16
		var wg sync.WaitGroup
		wg.Add(threads)
		for i := 0; i < threads; i++ {
			go func(i int) {
				defer wg.Done()
				tid := threadID(i)
				if err := store.Save(ctx, tid, "n", []byte(tid)); err != nil {
					t.Errorf("Save %s: %v", tid, err)
				}
			}(i)
		}
		wg.Wait()

		for i := 0; i < threads; i++ {
			tid := threadID(i)
			node, data, err := store.Load(ctx, tid)
			if err != nil {
				t.Fatalf("Load %s: %v", tid, err)
			}
			if node != "n" || string(data) != tid {
				t.Fatalf("Load %s: got (%q, %q)", tid, node, data)
			}
		}
	})
}

func threadID(i int) string {
	const hex = "0123456789abcdef"
	return "t" + string([]byte{hex[i>>4&0xf], hex[i&0xf]})
}
