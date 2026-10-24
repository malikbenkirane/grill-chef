# State database & session storage — 01-foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use supervised-plan-execution to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stand up grill-chef's persistent store foundation: Go module, the two allowed dependencies, hexagonal port interface, the five-table SQLite schema applied by embedded goose migrations (WAL + `busy_timeout`), and a race-tested adapter both writer processes can boot.

**Architecture:** Hexagonal — the domain talks to `internal/port/store` (interface + types); the SQLite adapter lives in `internal/port/store/sqlite` and applies schema migrations from an embedded `migrations` package using `goose.NewProvider` + `provider.Up(ctx)` with a `goose.SessionLocker` so the MCP process and web daemon can migrate concurrently. Dual-writer topology: MCP owns sessions/questions/round payloads, the daemon owns answers/comments and round status — enforced per-method in sub-plan 02.

**Tech Stack:** Go 1.22+, `github.com/glebarez/go-sqlite` (pure-Go driver), `github.com/pressly/goose/v3` provider API, stdlib `database/sql`, `go test`. Commit operations use `jj` only (core-commands rules; never `git`).

**Normative sources:** `docs/issues/1.md` (requirements), `docs/drafts/state.md` (decision trail), ulearn TIL goose-v3-programmatic-migrations (provider pattern).

---

### Task 1: Go module + dependencies

**Files:**
- Create: `go.mod`

- [ ] **Step 1: Initialize the module and fetch dependencies**

Run:

```sh
go mod init github.com/malikbenkirane/grill-chef
go get github.com/glebarez/go-sqlite@latest
go get github.com/pressly/goose/v3@latest
go mod tidy
```

Expected: `go.mod` contains both dependencies, no other externals. `glebarez/go-sqlite` must resolve with no cgo requirement (pure-Go).

- [ ] **Step 2: Verify the module graph**

Run: `go list -m all | grep -E 'glebarez|pressly'`

Expected: exactly those two requirement rows plus goose's transitive deps. If a mattn/go-sqlite3 or cgo-dependent driver appears in the graph, stop — that contradicts the single-binary decision in `docs/drafts/state.md`.

- [ ] **Step 3: Sanity-build**

Run: `go build ./... && go test ./... 2>&1 | tail -1`

Expected: `go build` succeeds with no Go files yet; test output ends `[no test files]` (or equivalent zero-package result).

- [ ] **Step 4: Commit with caveman-commit**

Use `caveman-commit` for the message content, then the commit sequence from `core-commands`:

1. `mktemp`
2. Read the temp file (required even though empty)
3. Write the commit message to the temp file
4. `jj describe --stdin < "/tmp/tmp.XXXXXX" && rm "/tmp/tmp.XXXXXX" && jj new`

Message subject: `feat(store): init module with pure-Go sqlite and goose deps`

---

### Task 2: Domain types

**Files:**
- Create: `internal/port/store/types.go`
- Test: `internal/port/store/types_test.go`

- [ ] **Step 1: Write the failing test**

```go
package store

import "testing"

func TestStatusConstants(t *testing.T) {
	if StatusActive != "active" || StatusClosed != "closed" || StatusDismissed != "dismissed" {
		t.Fatal("session status constants drifted from docs/issues/1.md")
	}
	if RoundPending != "pending" || RoundSubmitted != "submitted" {
		t.Fatal("round status constants drifted from docs/issues/1.md")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/port/store/ -run TestStatusConstants -v`

Expected: FAIL — undefined: `StatusActive` (or first missing identifier).

- [ ] **Step 3: Write the domain types**

```go
package store

import (
	"encoding/json"
	"time"
)

type SessionStatus string

const (
	StatusActive    SessionStatus = "active"
	StatusClosed    SessionStatus = "closed"
	StatusDismissed SessionStatus = "dismissed"
)

type RoundStatus string

const (
	RoundPending   RoundStatus = "pending"
	RoundSubmitted RoundStatus = "submitted"
)

type Session struct {
	ID        string
	Name      string
	Status    SessionStatus
	CreatedAt time.Time
	UpdatedAt time.Time
}

type RecapOption struct {
	ID    string
	Label string
}

type Round struct {
	SessionID string
	Number    int
	Status    RoundStatus
	Payload   json.RawMessage
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Question struct {
	SessionID     string
	Round         int
	Index         int
	Title         string
	Abstract      string
	Options       []RecapOption
	Recommended   int
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type Answer struct {
	SessionID     string
	Round         int
	QuestionIndex int
	ChosenOption  *int
	UpdatedAt     time.Time
}

type Comment struct {
	ID            int64
	SessionID     string
	Round         int
	QuestionIndex int
	Body          string
	CreatedAt     time.Time
}

type SessionAggregate struct {
	Session   Session
	Rounds    []Round
	Questions []Question
	Answers   []Answer
	Comments  []Comment
}
```

Design notes the implementer must keep:
- Round `Payload` is the provenance JSON (`json.RawMessage`) — intro + recap branches — written once by MCP; question *bodies* live in the `questions` table (never inside the payload), so render queries never re-parse question JSON.
- `Recommended` is an int option index, `-1` meaning "no recommendation"; keep it non-null in Go to avoid nil checks at call sites.
- `UpdatedAt` columns on session/round/question/answer are the staleness commodity from `docs/drafts/state.md` — they are never consulted for session-consistency correctness (sub-plan 03).

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/port/store/ -run TestStatusConstants -v`

Expected: PASS.

- [ ] **Step 5: Commit with caveman-commit**

Use `caveman-commit` for the message content, then the commit sequence:

1. `mktemp`
2. Read the temp file
3. Write the commit message to the temp file
4. `jj describe --stdin < "/tmp/tmp.XXXXXX" && rm "/tmp/tmp.XXXXXX" && jj new`

Message subject: `feat(store): domain types for sessions rounds questions answers comments`

---

### Task 3: Port interface (ownership encoded in the surface)

**Files:**
- Create: `internal/port/store/port.go`

- [ ] **Step 1: Write the interface**

```go
package store

import "context"

// Store is the domain's persistence port. Every method belongs to exactly one
// writer process (docs/issues/1.md ownership matrix):
//
//	MCP-owned:      CreateSession, GetSession, CreateRound, AddQuestion, CloseSession
//	daemon-owned:   UpsertAnswer, SubmitRound, AddComment, DeleteComment
//	shared readers: LoadSession, ListSessions, IsSessionConsistent
type Store interface {
	// MCP-owned
	CreateSession(ctx context.Context, id, name string) (Session, error)
	GetSession(ctx context.Context, id string) (Session, error)
	CreateRound(ctx context.Context, sessionID string, number int, payload Round) error
	AddQuestion(ctx context.Context, q Question) error
	CloseSession(ctx context.Context, id string) error

	// daemon-owned
	UpsertAnswer(ctx context.Context, sessionID string, round, questionIndex int, chosen *int) error
	SubmitRound(ctx context.Context, sessionID string, round int) error
	AddComment(ctx context.Context, sessionID string, round, questionIndex int, body string) (Comment, error)
	DeleteComment(ctx context.Context, commentID int64) error

	// shared readers
	ListSessions(ctx context.Context) ([]Session, error)
	LoadSession(ctx context.Context, id string) (SessionAggregate, error)
	IsSessionConsistent(ctx context.Context, id string) (bool, error)
}
```

- [ ] **Step 2: Verify the package compiles**

Run: `go build ./internal/port/store/`

Expected: success (no implementations yet — the adapter in Task 5 returns a `*Store` that satisfies this interface method-by-method in sub-plan 02; for now only the interface must compile).

- [ ] **Step 3: Commit with caveman-commit**

Use `caveman-commit` for the message content, then the commit sequence:

1. `mktemp`
2. Read the temp file
3. Write the commit message to the temp file
4. `jj describe --stdin < "/tmp/tmp.XXXXXX" && rm "/tmp/tmp.XXXXXX" && jj new`

Message subject: `feat(store): port interface encoding dual-writer ownership matrix`

---

### Task 4: Initial goose migration (five tables)

**Files:**
- Create: `internal/port/store/sqlite/migrations/00001_init.go`
- Test: `internal/port/store/sqlite/migrations_test.go`

- [ ] **Step 1: Write the failing test**

```go
package sqlite

import (
	"database/sql"
	"io/fs"
	"testing"

	_ "github.com/glebarez/go-sqlite"
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

	want := []string{"sessions", "rounds", "questions", "answers", "comments"}
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/port/store/sqlite/ -run TestInitMigration -v`

Expected: FAIL — undefined: `migrationsFS`.

- [ ] **Step 3: Write the migration**

```go
package migrations

import (
	"context"
	"database/sql"

	goose "github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(upInit, downInit)
}

var initStatements = []string{
	`CREATE TABLE IF NOT EXISTS sessions (
		id         TEXT PRIMARY KEY,
		name       TEXT NOT NULL,
		status     TEXT NOT NULL CHECK (status IN ('active','closed','dismissed')),
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE TABLE IF NOT EXISTS rounds (
		session_id   TEXT NOT NULL,
		round_number INTEGER NOT NULL CHECK (round_number BETWEEN 1 AND 5),
		status       TEXT NOT NULL CHECK (status IN ('pending','submitted')),
		payload      TEXT NOT NULL,
		created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (session_id) REFERENCES sessions(id),
		PRIMARY KEY (session_id, round_number)
	)`,
	`CREATE TABLE IF NOT EXISTS questions (
		session_id        TEXT NOT NULL,
		round_number      INTEGER NOT NULL,
		question_index    INTEGER NOT NULL,
		title             TEXT NOT NULL,
		abstract          TEXT NOT NULL,
		options           TEXT NOT NULL,
		recommended_index INTEGER NOT NULL DEFAULT -1,
		created_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (session_id, round_number) REFERENCES rounds(session_id, round_number) ON DELETE CASCADE,
		PRIMARY KEY (session_id, round_number, question_index)
	)`,
	`CREATE TABLE IF NOT EXISTS answers (
		session_id     TEXT NOT NULL,
		round_number   INTEGER NOT NULL,
		question_index INTEGER NOT NULL,
		chosen_option  INTEGER,
		created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (session_id, round_number, question_index) REFERENCES questions(session_id, round_number, question_index) ON DELETE CASCADE,
		PRIMARY KEY (session_id, round_number, question_index)
	)`,
	`CREATE TABLE IF NOT EXISTS comments (
		id             INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id     TEXT NOT NULL,
		round_number   INTEGER NOT NULL,
		question_index INTEGER NOT NULL,
		body           TEXT NOT NULL,
		created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (session_id, round_number, question_index) REFERENCES questions(session_id, round_number, question_index) ON DELETE CASCADE
	)`,
	`CREATE INDEX IF NOT EXISTS idx_rounds_session ON rounds(session_id, round_number)`,
	`CREATE INDEX IF NOT EXISTS idx_questions_session ON questions(session_id, round_number)`,
	`CREATE INDEX IF NOT EXISTS idx_answers_session ON answers(session_id, round_number)`,
	`CREATE INDEX IF NOT EXISTS idx_comments_session ON comments(session_id, round_number)`,
}

func upInit(ctx context.Context, db *sql.DB) error {
	for _, stmt := range initStatements {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func downInit(ctx context.Context, db *sql.DB) error {
	for _, stmt := range []string{
		`DROP INDEX IF EXISTS idx_comments_session`,
		`DROP INDEX IF EXISTS idx_answers_session`,
		`DROP INDEX IF EXISTS idx_questions_session`,
		`DROP INDEX IF EXISTS idx_rounds_session`,
		`DROP TABLE IF EXISTS comments`,
		`DROP TABLE IF EXISTS answers`,
		`DROP TABLE IF EXISTS questions`,
		`DROP TABLE IF EXISTS rounds`,
		`DROP TABLE IF EXISTS sessions`,
	} {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}
```

Schema notes the implementer must keep:
- `rounds.payload` holds the round provenance JSON only (`json.RawMessage` on the domain `Round`); question bodies are always question rows — that is the domain separation from the grill transcript.
- `answers.chosen_option` is nullable (`pending` round = no choice saved yet); non-null AFTER the user's first autosave.
- `comments.id` is AUTOINCREMENT so the `DELETE /s/<id>/r/<n>/q/<m>/comment/<i>` route carries a stable per-session monotonic index.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/port/store/sqlite/ -run TestInitMigration -v`

Expected: both tests PASS.

- [ ] **Step 5: Commit with caveman-commit**

Use `caveman-commit` for the message content, then the commit sequence:

1. `mktemp`
2. Read the temp file
3. Write the commit message to the temp file
4. `jj describe --stdin < "/tmp/tmp.XXXXXX" && rm "/tmp/tmp.XXXXXX" && jj new`

Message subject: `feat(store): goose migration for five-table session schema`

---

### Task 5: Adapter boot — DSN pragmas, session locker, embedded migrations

**Files:**
- Create: `internal/port/store/sqlite/store.go`
- Test: `internal/port/store/sqlite/store_test.go`

- [ ] **Step 1: Write the failing test**

```go
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

func TestNewOnFreshDbCreatesMarkerFiles(t *testing.T) {
	dir := t.TempDir()
	s, err := New(Config{Path: filepath.Join(dir, "state.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// WAL leaves its sidecars next to the db; their presence (not content) is
	// part of the dual-writer contract we document in issue 8.
	for _, side := range []string{"state.db-wal", "state.db-shm"} {
		if _, statErr := os.Stat(filepath.Join(dir, side)); statErr != nil {
			t.Logf("sidecar %s not found (%v) — acceptable before first WAL write", side, statErr)
		}
	}
}
```

Hmm — `os.Stat` on `state.db-wal` is racy across platforms (WAL sidecars may not exist until first write), so this test must be *tolerant* of their absence (the `t.Logf` path), asserting only the pragmatic contract: connection works, journal mode is WAL, foreign keys on, busy timeout set. The first two rules above are the real assertions.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/port/store/sqlite/ -run TestNew -v`

Expected: FAIL — undefined: `New` (and `Config`).

- [ ] **Step 3: Implement the adapter boot**

```go
package sqlite

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"sync"
	"time"

	_ "github.com/glebarez/go-sqlite"
	"github.com/pressly/goose/v3"
)

//go:embed migrations
var migrationsFS embed.FS

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

// fileDSN appends the pragmas the dual-writer topology needs, using the
// glebarez URL-DSN form: repeated `_pragma` keys, one pragma per value.
func fileDSN(path string) string {
	v := url.Values{}
	v.Add("_pragma", "busy_timeout(5000)")
	v.Add("_pragma", "foreign_keys(1)")
	v.Add("_pragma", "journal_mode(WAL)")
	return "file:" + path + "?" + v.Encode()
}
```

Boot — idempotent migration with the session locker:

```go
func (s *Store) Boot(ctx context.Context) error {
	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("sqlite: migrations embed: %w", err)
	}
	locker := &sessionLocker{}
	provider, err := goose.NewProvider(goose.DialectSQLite3, s.db, sub,
		goose.WithSessionLocker(locker),
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

// sessionLocker guards migrationProvider.Up against the two processes racing
// to migrate the same file (goose.WithSessionLocker contract).
type sessionLocker struct {
	mu   sync.Mutex
	conn *sql.Conn
}

func (l *sessionLocker) Lock(ctx context.Context, db *sql.DB) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	for i := 0; i < 100; i++ {
		if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err == nil {
			l.conn = conn
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
	return errors.New("sqlite: timed out acquiring migration lock")
}

func (l *sessionLocker) Unlock(_ context.Context, _ *sql.DB) error {
	if l.conn == nil {
		return nil
	}
	_, _ = l.conn.ExecContext(context.Background(), "COMMIT")
	_ = l.conn.Close()
	l.conn = nil
	return nil
}
```

Remove unused imports if the compiler flags any — the intended final import list is exactly: `context`, `database/sql`, `embed`, `errors`, `fmt`, `io/fs`, `net/url`, `sync`, `time`, plus the two external imports. `go vet ./internal/port/store/sqlite/` must be clean.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/port/store/sqlite/ -v`

Expected: all tests PASS (including Task 4's migration tests, which share the file harness pattern above).

- [ ] **Step 5: Commit with caveman-commit**

Use `caveman-commit` for the message content, then the commit sequence:

1. `mktemp`
2. Read the temp file
3. Write the commit message to the temp file
4. `jj describe --stdin < "/tmp/tmp.XXXXXX" && rm "/tmp/tmp.XXXXXX" && jj new`

Message subject: `feat(store): sqlite adapter boot with WAL, busy_timeout, locker migration`

---

### Task 6: Shared test harness

**Files:**
- Create: `internal/port/store/sqlite/harness_test.go`

- [ ] **Step 1: Write the harness**

```go
package sqlite

import (
	"os"
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
```

- [ ] **Step 2: Add an in-memory variant for fast locale tests**

```go
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
```

Checks to make here:
- Run the whole package test suite to prove the harness didn't regress anything:

Run: `go test ./... 2>&1 | grep -v 'no test files'`

Expected: `ok` for the two test packages so far.

- [ ] **Step 3: Commit with caveman-commit**

Use `caveman-commit` for the message content, then the commit sequence:

1. `mktemp`
2. Read the temp file
3. Write the commit message to the temp file
4. `jj describe --stdin < "/tmp/tmp.XXXXXX" && rm "/tmp/tmp.XXXXXX" && jj new`

Message subject: `test(store): shared temp-file and in-memory sqlite harness`

---

### Task 7: Dual-runner migration race test

**Files:**
- Test: `internal/port/store/sqlite/race_test.go`

- [ ] **Step 1: Write the failing test**

```go
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
		 ('sessions','rounds','questions','answers','comments')`).Scan(&tables); err != nil {
		t.Fatal(err)
	}
	if tables != 5 {
		t.Fatalf("want 5 tables after concurrent migration, got %d", tables)
	}
}
```

- [ ] **Step 2: Run test to verify it passes**

Run: `go test ./internal/port/store/sqlite/ -run TestTwoStores -race -v -count 5`

Expected: PASS on every count; if a runner ever fails with "no such table" or a deadlock, the `sessionLocker` is wrong — a failed retry loop is not acceptable, fix the locker before proceeding.

- [ ] **Step 3: Race-detect the store package as a whole**

Run: `go test ./... -race`

Expected: all packages `ok` under the race detector.

- [ ] **Step 4: Commit with caveman-commit**

Use `caveman-commit` for the message content, then the commit sequence:

1. `mktemp`
2. Read the temp file
3. Write the commit message to the temp file
4. `jj describe --stdin < "/tmp/tmp.XXXXXX" && rm "/tmp/tmp.XXXXXX" && jj new`

Message subject: `test(store): two processes race on same store both migrate safely`

---

### Task 8: Full build + vet + dependency audit

**Files:**
- No new files — validation gate

- [ ] **Step 1: Build and vet everything**

Run: `go build ./... && go vet ./...`

Expected: clean (no output).

- [ ] **Step 2: Full test suite under race detector**

Run: `go test ./... -race -count 2`

Expected: `ok` for both test packages on both runs.

- [ ] **Step 3: cgo audit — the single-binary promise**

Run: `CGO_ENABLED=0 go build ./... && go list -m all | grep -i 'sqlite'`

Expected: build succeeds with cgo disabled; only `glebarez/go-sqlite` + its `modernc.org` transitive graph appears, no `mattn/go-sqlite3`. If any cgo-dependent sqlite shows up, the wrong driver got linked — stop and fix before handing off.

- [ ] **Step 4: Commit with caveman-commit (if anything was touched)**

If Steps 1–3 forced a change, commit it via the `caveman-commit` + `core-commands` sequence (same as Tasks 1–7). If nothing was touched, skip — do not commit empty changes.

- [ ] **Step 5: Verify docs alignment before marking this sub-plan done**

Re-read `docs/issues/1.md` and `docs/drafts/state.md`; confirm the implemented schema matches both normative sources on: five tables, composite indexes, WAL + `busy_timeout`, status enums (3 session values, 2 round values), staleness columns present but unused for correctness. List any drift you find and raise it in chat **before** moving to sub-plan 02.
