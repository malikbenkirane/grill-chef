package sqlite

import (
	"path/filepath"
	"testing"
)

// newTestStore returns a ready store backed by a fresh temp file. Sub-plans 02
// and 03 build all their TDD fixtures on this helper: never inline
// New+t.TempDir in their tests.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := New(Config{Path: filepath.Join(t.TempDir(), "state.db")})
	if err != nil {
		t.Fatalf("newTestStore: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// newMemoryStore is for tests that assert pure SQL semantics (no WAL, no
// sidecars) and want the speed of :memory:.
func newMemoryStore(t *testing.T) *Store {
	t.Helper()
	s, err := New(Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("newMemoryStore: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}
