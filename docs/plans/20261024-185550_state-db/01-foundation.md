# State database & session storage — 01-foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use supervised-plan-execution to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stand up grill-chef's persistent store foundation: Go module, the two allowed dependencies, domain upgrades (`StatusDismissed`, `UpdatedAt` staleness fields), the hexagonal port interface typed with `internal/domain` aggregates, a six-table SQLite schema applied by embedded goose migrations (WAL + `busy_timeout`), and a race-tested adapter boot both writer processes can use.

**Architecture:** Hexagonal — the domain talks to `internal/port/store` (interface only, domain types); the SQLite adapter lives in `internal/port/store/sqlite`, owns local types + conversion, and applies schema migrations from an embedded `migrations` package using `goose.NewProvider` + `provider.Up(ctx)` with a `goose.SessionLocker` so the MCP process and web daemon can migrate concurrently. Dual-writer topology: MCP owns sessions/questions/round payloads/legends, the daemon owns answers/comments and round status — enforced per-method in sub-plan 02.

**Tech Stack:** Go 1.22+, `github.com/glebarez/go-sqlite` (pure-Go driver), `github.com/pressly/goose/v3` provider API, stdlib `database/sql`, `go test`. Commit operations use `jj` only (core-commands rules; never `git`).

**Normative sources:** `docs/issues/1.md` (requirements), `docs/drafts/state.md` (decision trail), ulearn TIL goose-v3-programmatic-migrations (provider pattern).

**Design decisions settled in grilling (2026-10-25)** — the implementer must not lose these:

- The port references `internal/domain` types exclusively; adapters own local types and conversion. No second set of sandwich types is created in `internal/port/store`.
- Two MCP-owned create methods reflect the round kind typology: `CreateQuestionRound` and `CreateLegendRound`. The legend round has its own `legends` relation; it never lives in `rounds`.
- No round-cap or round-numbering CHECK at the store layer; contiguity and legend-at-end are domain policy (`IsSessionConsistent`, sub-plan 03).
- Persistence stores exactly what the domain holds: 1-based choice indexes with domain sentinels (`ChoiceIndexNone = -1`, `RecommendationNone = 0`) and no edge translation.
- Comments drop AUTOINCREMENT; PK `(session_id, round_number, question_index, comment_seq)` yields stable per-question identity for issue 8's DELETE route.
- `rounds.payload` holds intro + recap only for question rounds (questions serialize to their own rows, no post-load merge); `legends.payload` holds the full `LegendRound`.
- `domain/session` gains `StatusDismissed` (the domain is the state's truth; set by issue 14's station home); issue 1's status note and schema count are amended in Task 8.
- `domain/round` concrete round types and `domain/session.Session` gain `UpdatedAt` staleness fields (staleness commodity from issue 1 — never consulted for correctness).
- The migration session locker is a single `BEGIN IMMEDIATE` with the context deadline; SQLite itself arbitrates cross-process contention under WAL + `busy_timeout`.

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

Expected: `go build` succeeds with the existing `internal/domain` packages; test output ends `ok` (or equivalent).

- [ ] **Step 4: Commit with caveman-commit**

Use `caveman-commit` for the message content, then the commit sequence from `core-commands`:

1. `mktemp`
2. Read the temp file (required even though empty)
3. Write the commit message to the temp file
4. `jj describe --stdin < "/tmp/tmp.XXXXXX" && rm "/tmp/tmp.XXXXXX" && jj new`

Message subject: `feat(store): init module with pure-Go sqlite and goose deps`

---

### Task 2: Domain upgrades — StatusDismissed and UpdatedAt staleness fields

**Files:**
- Modify: `internal/domain/session/session.go`
- Test: `internal/domain/session/dismissed_test.go`
- Modify: `internal/domain/round/round.go`
- Test: `internal/domain/round/updatedat_test.go`

- [ ] **Step 1: Write the failing tests**

`internal/domain/session/dismissed_test.go`:

```go
package session

import "testing"

func TestStatusDismissed(t *testing.T) {
	if StatusDismissed != Status(3) {
		t.Fatalf("StatusDismissed must be the iota follower of StatusClosed, got %d", StatusDismissed)
	}
	if got := StatusDismissed.String(); got != "dismissed" {
		t.Fatalf("StatusDismissed.String(): want %q, got %q", "dismissed", got)
	}
}
```

`internal/domain/round/updatedat_test.go`:

```go
package round

import (
	"testing"
	"time"
)

func TestRoundsCarryUpdatedAt(t *testing.T) {
	now := time.Now()
	q := QuestionRound{}
	if !q.UpdatedAt.IsZero() {
		t.Fatal("fresh QuestionRound must carry a zero UpdatedAt")
	}
	q.UpdatedAt = now
	if !q.UpdatedAt.Equal(now) {
		t.Fatal("QuestionRound.UpdatedAt not assignable")
	}
	l := LegendRound{}
	if !l.UpdatedAt.IsZero() {
		t.Fatal("fresh LegendRound must carry a zero UpdatedAt")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/domain/session/ -run TestStatusDismissed -v && go test ./internal/domain/round/ -run TestRoundsCarryUpdatedAt -v`

Expected: FAIL — `StatusDismissed` undefined, and field `UpdatedAt` missing on both round structs.

- [ ] **Step 3: Apply the domain edits**

In `internal/domain/session/session.go` add, after `StatusClosed`:

```go
	// StatusDismissed means the human cleared the session from the station
	// home (issue 14). Dismiss is a status change, never a delete; the
	// record is preserved and rehydrates as dismissable history.
	StatusDismissed
```

and extend `String()` with:

```go
	case StatusDismissed:
		return "dismissed"
```

In `internal/domain/round/round.go` add an exported `UpdatedAt` field to both `QuestionRound` and `LegendRound`:

```go
	// UpdatedAt is the store's staleness commodity (issue 1): it rides the
	// aggregate after rehydration, but is never consulted for session
	// consistency correctness. Duck-typed by sessions with pending rounds
	// that sat idle for days.
	UpdatedAt time.Time
```

The `time` import must be added to the file's import block.

Also note the settlement: `session.Status.String()` renders the store-edge literals — `active`, `closed`, `dismissed` — matching the `sessions.status` CHECK in the migration (Task 4).

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/domain/... -race`

Expected: PASS across `session` and `round` both.

- [ ] **Step 5: Commit with caveman-commit**

Use `caveman-commit` for the message content, then the commit sequence:

1. `mktemp`
2. Read the temp file
3. Write the commit message to the temp file
4. `jj describe --stdin < "/tmp/tmp.XXXXXX" && rm "/tmp/tmp.XXXXXX" && jj new`

Message subject: `feat(domain): dismissed status and staleness timestamps`

---

### Task 3: Port interface (ownership encoded in the surface, domain-typed)

**Files:**
- Create: `internal/port/store/port.go`

- [ ] **Step 1: Write the interface**

```go
package store

import (
	"context"

	"github.com/malikbenkirane/grill-chef/internal/domain/round"
	"github.com/malikbenkirane/grill-chef/internal/domain/session"
)

// Store is the domain's persistence port. Every method belongs to exactly one
// writer process (docs/issues/1.md ownership matrix):
//
//	MCP-owned:      CreateSession, CloseSession, CreateQuestionRound, CreateLegendRound
//	daemon-owned:   UpsertAnswer, SubmitRound, AddComment, DeleteComment, DismissSession
//	shared readers: ListSessions, LoadSession, IsSessionConsistent
//
// The port speaks only in domain aggregates; the SQLite adapter owns local
// types and converts at the edge. Persistence stores domain conventions
// verbatim (1-based choice indexes, sentinels) with no translation.
type Store interface {
	// MCP-owned: sessions and rounds payload agents (stdio tool calls)
	CreateSession(ctx context.Context, s *session.Session) error
	CloseSession(ctx context.Context, id string) error
	CreateQuestionRound(ctx context.Context, r round.QuestionRound) error
	CreateLegendRound(ctx context.Context, r round.LegendRound) error

	// daemon-owned: browser autosave / submit / comment / station-home dismiss
	UpsertAnswer(ctx context.Context, sessionID string, roundNumber, questionIndex int, choice round.ChoiceIndex) error
	SubmitRound(ctx context.Context, sessionID string, roundNumber int) error
	AddComment(ctx context.Context, sessionID string, roundNumber, questionIndex int, body string) (seq int, err error)
	DeleteComment(ctx context.Context, sessionID string, roundNumber, questionIndex, seq int) error
	DismissSession(ctx context.Context, id string) error

	// shared readers
	ListSessions(ctx context.Context) ([]session.Session, error)
	LoadSession(ctx context.Context, id string) (*session.Session, error)
	IsSessionConsistent(ctx context.Context, id string) (bool, error)
}
```

Design notes the implementer must keep:
- `CreateSession` takes the `*session.Session` domain aggregate whole (ID minted by the caller's tool layer); the adapter stamps nothing but persists fields as-is.
- `UpsertAnswer` persists exactly the domain choice: `ChoiceIndexNone` (-1) and `ChoiceIndexOther` (0) travel unmodified as `answers.chosen_option`; the "Needs at least one comment" rule is domain `AnswerSheet.Validate`, called by the tool layer, never by the store.
- `AddComment` returns the per-question `comment_seq` (PK `(session_id, round_number, question_index, comment_seq)`), the identity issue 8's `DELETE /s/<id>/r/<n>/q/<m>/comment/<i>` route carries round-tripped to the browser.
- `DismissSession` is a status change (`StatusDismissed`), never a delete — retention keeps everything (issue 14 / decision trail).
- `StatusUndefined` and `KindUndefined` never reach the store; the adapter returns an error if an undefined status arrives — see 02 for the exact test.

- [ ] **Step 2: Verify the package compiles**

Run: `go build ./internal/port/store/`

Expected: success (no implementations yet — the adapter in Task 5 returns a `*Store` that satisfies this interface method-by-method in sub-plan 02; for now only the interface must compile).

- [ ] **Step 3: Commit with caveman-commit**

Use `caveman-commit` for the message content, then the commit sequence:

1. `mktemp`
2. Read the temp file
3. Write the commit message to the temp file
4. `jj describe --stdin < "/tmp/tmp.XXXXXX" && rm "/tmp/tmp.XXXXXX" && jj new`

Message subject: `feat(store): domain-typed port interface encoding ownership matrix`

---

### Task 4: Initial goose migration (six tables)

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
		round_number INTEGER NOT NULL,
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
		recommended_index INTEGER NOT NULL DEFAULT 0,
		reminders         TEXT NOT NULL DEFAULT '[]',
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
		session_id     TEXT NOT NULL,
		round_number   INTEGER NOT NULL,
		question_index INTEGER NOT NULL,
		comment_seq    INTEGER NOT NULL,
		body           TEXT NOT NULL,
		created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (session_id, round_number, question_index) REFERENCES questions(session_id, round_number, question_index) ON DELETE CASCADE,
		PRIMARY KEY (session_id, round_number, question_index, comment_seq)
	)`,
	`CREATE TABLE IF NOT EXISTS legends (
		session_id   TEXT PRIMARY KEY,
		payload      TEXT NOT NULL,
		created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE
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
		`DROP TABLE IF EXISTS legends`,
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
- The sessions `status` CHECK still carries the three literals `('active','closed','dismissed')` — these are the store-edge rendering of the domain's `session.Status.String()` (including `dismissed` after Task 2).
- Round numbering has NO CHECK: contingency and legend-at-end are domain policy (`IsSessionConsistent`, sub-plan 03), not schema policy. The MCP observation rounds do run 1..5 plus a legend round, but the store never hard-codes a count.
- `rounds.payload` holds the question-round JSON minus the questions (`json.RawMessage` of intro + recap) — the questions table is the single source of truth for question bodies. `legends.payload` holds the whole `LegendRound` JSON (branches, bullets) — it is never copied anywhere.
- `answers.chosen_option` is nullable and stores the domain's raw `round.ChoiceIndex` values: `-1` (none/skip), `0` (other — free-text comments carry the content), 1..N (listed choice). Non-null only after the user's first autosave.
- `questions.recommended_index` stores the domain's `Recommendation`: `0` = none, 1..N otherwise. `reminders` carries the serialized `[]Reminder` array (`'[]'` when empty).
- `comments.comment_seq` is monotonic per question (allocated by `MAX(comment_seq)+1` inside the write transaction, see 02); no AUTOINCREMENT — comment identity is `(session, round, question, seq)` for the DELETE route in issue 8.
- `updated_at` columns on session/round/question/answer/legend are the staleness commodity from `docs/drafts/state.md` — never consulted for session-consistency correctness (sub-plan 03).

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/port/store/sqlite/ -run TestInitMigration -v`

Expected: both tests PASS.

- [ ] **Step 5: Commit with caveman-commit**

Use `caveman-commit` for the message content, then the commit sequence:

1. `mktemp`
2. Read the temp file
3. Write the commit message to the temp file
4. `jj describe --stdin < "/tmp/tmp.XXXXXX" && rm "/tmp/tmp.XXXXXX" && jj new`

Message subject: `feat(store): goose migration for six-table session schema`

---

### Task 5: Adapter boot — DSN pragmas, session locker, embedded migrations

**Files:**
- Create: `internal/port/store/sqlite/store.go`
- Test: `internal/port/store/sqlite/store_test.go`

- [ ] **Step 1: Write the failing tests**

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
```

Note on sidecars: `os.Stat` on `state.db-wal`/`state.db-shm` is racy (they may not exist until the first WAL write), so the tests DO NOT assert sidecar presence — only the connection, journal mode, foreign keys, and busy timeout. The WAL sidecar contract is documented in issue 8, not asserted in tests here.

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
	"fmt"
	"io/fs"
	"net/url"

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

Boot — idempotent migration with the single-shot session locker:

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

// sessionLocker serializes concurrent migrations against the same file by
// holding one dedicated connection open inside an IMMEDIATE transaction for
// the duration of the Up call. Cross-process contention is SQLite's job
// (WAL + busy_timeout): the context deadline governs, no retry loop here.
type sessionLocker struct {
	conn *sql.Conn
}

func (l *sessionLocker) Lock(ctx context.Context, db *sql.DB) error {
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("sqlite: locker conn: %w", err)
	}
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		_ = conn.Close()
		return fmt.Errorf("sqlite: locker begin: %w", err)
	}
	l.conn = conn
	return nil
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

Removed from the previous draft version: the `sync.Mutex`-guarded 100×50 ms retry loop. The `busy_timeout(5000)` DSN pragma already makes `BEGIN IMMEDIATE` wait up to 5 s for cross-process contention; a bounded context deadline (whatever the caller of `New` provides) bounds the total wait. If a runner failed with "database is locked" under Task 7's race test, the timeout is too tight — tune the context deadline in the caller, never resurrect the loop in the locker.

The `errors`/`sync`/`time` imports from the old draft are gone too. The final import list is exactly: `context`, `database/sql`, `embed`, `fmt`, `io/fs`, `net/url`, plus the two external imports. `go vet ./internal/port/store/sqlite/` must be clean.

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

- [ ] **Step 2: Add an in-memory variant for fast SQL-semantics tests**

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
		 ('sessions','rounds','questions','answers','comments','legends')`).Scan(&tables); err != nil {
		t.Fatal(err)
	}
	if tables != 6 {
		t.Fatalf("want 6 tables after concurrent migration, got %d", tables)
	}
}
```

- [ ] **Step 2: Run test to verify it passes**

Run: `go test ./internal/port/store/sqlite/ -run TestTwoStores -race -v -count 5`

Expected: PASS on every count; if a runner ever fails with "no such table" or a lock-timeout, the locker's single `BEGIN IMMEDIATE` is inside a lost context (Task 5's settled design) — inspect the caller's deadline before assuming SQLite contention is the problem.

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

### Task 8: Full build + vet + dependency audit + docs reconciliation

**Files:**
- Modify: `docs/issues/1.md` (status note + schema bullet)
- Modify: `docs/drafts/state.md` (legends decision-trail bullet)
- No Go files — validation gate

- [ ] **Step 1: Build and vet everything**

Run: `go build ./... && go vet ./...`

Expected: clean (no output).

- [ ] **Step 2: Full test suite under race detector**

Run: `go test ./... -race -count 2`

Expected: `ok` for both test packages on both runs.

- [ ] **Step 3: cgo audit — the single-binary promise**

Run: `CGO_ENABLED=0 go build ./... && go list -m all | grep -i 'sqlite'`

Expected: build succeeds with cgo disabled; only `glebarez/go-sqlite` + its `modernc.org` transitive graph appears, no `mattn/go-sqlite3`. If any cgo-dependent sqlite shows up, the wrong driver got linked — stop and fix before handing off.

- [ ] **Step 4: Amend `docs/issues/1.md`**

Three edits, keeping every other line untouched:

1. The parenthetical note at the top of Requirements — replace it so statuses name the domain's truth:

   *Old:*
   > session statuses are the domain's `Active | Closed` (rendered as `active | closed` at the store edge).

   *New:*
   > session statuses are the domain's `StatusActive | StatusClosed | StatusDismissed` — the domain is the state's truth, and the store renders its `String()` values (`active | closed | dismissed`) at the edge. `StatusDismissed` is set by the station home (issue 14), written by the web daemon through `DismissSession`.

2. The schema bullet — replace "five tables" and add `legends`:

   *New bullet:*
   > `sessions`, `rounds`, `questions`, `answers`, `comments`, `legends` (comments are first-class, matching the per-comment DELETE route in issue 8; `legends` stores the closing legend round in its own relation — `rounds` holds question rounds only). FK spine on `session_id`; no round-cap CHECK at the store layer; composite indexes on `(session_id, round)` and `(session_id, round, question)`. Full session rehydration is a single indexed read — no post-load merge in Go.

3. The dual-writer topology table row for sessions — append the daemon's one exception (station-home dismiss):
   > sessions | MCP, plus daemon's `DismissSession` (status change only, issue 14)

- [ ] **Step 5: Amend `docs/drafts/state.md`**

In the "Schema" section, replace the "Five tables: …" bullet with:

> Six tables: `sessions`, `rounds`, `questions`, `answers`, `comments`, `legends`. The legend round was originally sketched as a row of `rounds` and **split off into its own `legends` relation** (one per session) once the kind-gated domain (`QuestionRound` vs `LegendRound`) landed: question-round rows carry questions/options/answers/comments, legend rows carry the closing legend payload whole; no round-cap CHECK lives in the store — contiguity and legend-at-end are domain policy enforced by `IsSessionConsistent`.

And append to the same section:

> Comments dropped AUTOINCREMENT for PK `(session_id, round_number, question_index, comment_seq)` — the identity the DELETE route in issue 8 carries is per-question sequence, stable across sessions; a global auto counter bought nothing.

- [ ] **Step 6: Commit with caveman-commit (docs amendment)**

Use `caveman-commit` for the message content, then the commit sequence (same temp-file pattern as Tasks 1–7).

Message subject: `docs(issue-1): six-table schema, dismissed status, daemon dismiss write`

- [ ] **Step 7: Verify docs alignment before marking this sub-plan done**

Re-read `docs/issues/1.md` and `docs/drafts/state.md`; confirm the implemented schema matches both normative sources on: six tables (incl. `legends`), composite indexes, WAL + `busy_timeout`, status enums (3 session values via `session.Status.String()`, 2 round values), staleness columns present but unused for correctness. List any drift you find and raise it in chat **before** moving to sub-plan 02.
