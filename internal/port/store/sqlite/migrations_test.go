package sqlite

import (
	"database/sql"
	"io/fs"
	"testing"

	_ "github.com/glebarez/go-sqlite"
	_ "github.com/malikbenkirane/grill-chef/internal/port/store/sqlite/migrations"
	"github.com/pressly/goose/v3"
)

func TestInitMigrationCreatesTables(t *testing.T) {
	dir := t.TempDir()
	db, err := sql.Open("sqlite", dir+"/test.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	fsys, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, db, fsys)
	if err != nil {
		t.Fatal(err)
	}
	results, err := provider.Up(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("want exactly 1 migration applied on fresh db, got %d", len(results))
	}

	want := []string{"sessions", "rounds", "questions", "answers", "comments", "legends"}
	for _, name := range want {
		var got string
		row := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, name)
		if err := row.Scan(&got); err != nil {
			t.Fatalf("table %q missing after migration: %v", name, err)
		}
		if got != name {
			t.Fatalf("table name mismatch: want %q, got %q", name, got)
		}
	}
}

func TestInitMigrationIdempotent(t *testing.T) {
	dir := t.TempDir()
	db, err := sql.Open("sqlite", dir+"/test.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	fsys, _ := fs.Sub(migrationsFS, "migrations")
	provider, _ := goose.NewProvider(goose.DialectSQLite3, db, fsys)
	if _, err := provider.Up(t.Context()); err != nil {
		t.Fatal(err)
	}
	results, err := provider.Up(t.Context())
	if err != nil {
		t.Fatalf("re-running Up on migrated db must be a no-op, got %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 applied migrations on second Up, got %d", len(results))
	}
}
