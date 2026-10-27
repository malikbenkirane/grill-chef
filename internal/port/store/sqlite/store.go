package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"net/url"

	_ "github.com/glebarez/go-sqlite"
	"github.com/pressly/goose/v3"
)

type Config struct {
	// Path is the sqlite file path, e.g. os.UserCacheDir()+"/grill-chef/state.db".
	// ":memory:" is allowed for tests.
	Path string
}

type Store struct {
	db *sql.DB
}

func New(cfg Config) (*Store, error) {
	db, err := sql.Open("sqlite", fileDSN(cfg.Path))
	if err != nil {
		return nil, fmt.Errorf("sqlite: open %q: %w", cfg.Path, err)
	}
	s := &Store{db: db}
	if err := s.Boot(context.Background()); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// Boot runs the embedded goose migrations idempotently with the single-shot
// session locker.
func (s *Store) Boot(ctx context.Context) error {
	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("sqlite: migrations embed: %w", err)
	}
	locker := &sessionLocker{}
	provider, err := goose.NewProvider(goose.DialectSQLite3, s.db, sub,
		goose.WithSessionLocker(locker),
		goose.WithIsolateDDL(true),
		goose.WithVerbose(false))
	if err != nil {
		return fmt.Errorf("sqlite: goose provider: %w", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("sqlite: migrate: %w", err)
	}
	return nil
}

// Ready is the post-boot health assertion used by daemon and MCP startup.
func (s *Store) Ready(ctx context.Context) error {
	var one int
	return s.db.QueryRowContext(ctx, "SELECT 1").Scan(&one)
}

func (s *Store) Close() error { return s.db.Close() }

// fileDSN appends the pragmas the dual-writer topology needs, using the
// glebarez URL-DSN form: repeated `_pragma` keys, one pragma per value.
func fileDSN(path string) string {
	if path == ":memory:" {
		return ":memory:"
	}
	v := url.Values{}
	v.Add("_pragma", "busy_timeout(5000)")
	v.Add("_pragma", "foreign_keys(1)")
	v.Add("_pragma", "journal_mode(WAL)")
	return "file:" + path + "?" + v.Encode()
}

// sessionLocker serializes concurrent migrations against the same file by
// holding one dedicated connection open inside an IMMEDIATE transaction for
// the duration of the Up call. Cross-process contention is SQLite's job
// (WAL + busy_timeout): the context deadline governs, no retry loop here.
// goose acquires and owns the connection; lock and unlock run on the same one.
type sessionLocker struct{}

func (s *sessionLocker) SessionLock(ctx context.Context, conn *sql.Conn) error {
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return fmt.Errorf("sqlite: locker begin: %w", err)
	}
	return nil
}

func (s *sessionLocker) SessionUnlock(_ context.Context, conn *sql.Conn) error {
	if _, err := conn.ExecContext(context.Background(), "COMMIT"); err != nil {
		return fmt.Errorf("sqlite: locker commit: %w", err)
	}
	return nil
}
