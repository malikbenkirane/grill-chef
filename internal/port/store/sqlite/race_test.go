package sqlite

import (
	"path/filepath"
	"sync"
	"testing"
)

// Two grill-chef processes (MCP + daemon) may boot against the same store
// concurrently. Both must converge on the migrated schema without error —
// goose's WithSessionLocker serializes the migrations.
func TestTwoStoresRacingOnSameFileBothMigrate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")

	const runners = 2
	errs := make([]error, runners)
	stores := make([]*Store, runners)

	var wg sync.WaitGroup
	for i := 0; i < runners; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			s, err := New(Config{Path: path})
			stores[i] = s
			errs[i] = err
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("runner %d failed to boot: %v", i, err)
		}
	}
	for _, s := range stores {
		defer s.Close()
		if err := s.Ready(t.Context()); err != nil {
			t.Fatalf("runner store not ready: %v", err)
		}
	}

	// Schema exists exactly once: sessions table present, no partial state.
	var tables int
	if err := stores[0].db.QueryRow(
		`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name IN
		 ('sessions','rounds','questions','answers','comments','legends')`).Scan(&tables); err != nil {
		t.Fatal(err)
	}
	if tables != 6 {
		t.Fatalf("want 6 tables after concurrent migration, got %d", tables)
	}
}
