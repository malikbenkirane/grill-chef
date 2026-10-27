package sqlite

import (
	"path/filepath"
	"testing"
)

func TestNewConnectsAndMigrates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	s, err := New(Config{Path: path})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer s.Close()

	if err := s.Ready(t.Context()); err != nil {
		t.Fatalf("store not ready after boot: %v", err)
	}
}

func TestNewSetsWALAndBusyTimeout(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	s, err := New(Config{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	var wal string
	if err := s.db.QueryRow("PRAGMA journal_mode").Scan(&wal); err != nil {
		t.Fatal(err)
	}
	if wal != "wal" {
		t.Fatalf("journal_mode: want wal, got %q", wal)
	}

	var fk int
	if err := s.db.QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil {
		t.Fatal(err)
	}
	if fk != 1 {
		t.Fatalf("foreign_keys: want 1, got %d", fk)
	}

	var busyMs int64
	if err := s.db.QueryRow("PRAGMA busy_timeout").Scan(&busyMs); err != nil {
		t.Fatal(err)
	}
	if busyMs != 5000 {
		t.Fatalf("busy_timeout: want 5000, got %d", busyMs)
	}
}
