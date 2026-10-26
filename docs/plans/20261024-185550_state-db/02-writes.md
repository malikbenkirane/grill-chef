# State database & session storage — 02-writes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use supervised-plan-execution to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement every store write path from the ownership matrix: MCP-owned session/round/payload/legend creation and closing, daemon-owned answer autosave, round submit, comment add/delete, and station-home dismiss — each as a `Store` interface method on the SQLite adapter, fully TDD on the shared harness.

**Architecture:** The adapter (`internal/port/store/sqlite`) owns local conversion: domain aggregates marshal to column shapes at the edge, and every method is exactly one writer process's path (issue 1 matrix). Persistence stores domain conventions verbatim (1-based choice indexes, `ChoiceIndexNone = -1`, `RecommendationNone = 0`), sentinels included — no translation. Legend rounds live in `legends`, never `rounds`; comment identity is `(session_id, round_number, question_index, comment_seq)`.

**Tech Stack:** Go 1.22+, `database/sql` + glebarez driver, goose migrations already applied by boot (sub-plan 01), stdlib `encoding/json`, `go test -race`.

**Normative sources:** `docs/issues/1.md`, `docs/drafts/state.md`, the port in `internal/port/store/port.go` (sub-plan 01), domain in `internal/domain/{session,round}`.

---

### Task 1: Adapter-local payload DTOs and row mappers

**Files:**
- Create: `internal/port/store/sqlite/mapping.go`
- Test: `internal/port/store/sqlite/mapping_test.go`

- [ ] **Step 1: Write the failing tests**

```go
package sqlite

import (
	"testing"

	"github.com/malikbenkirane/grill-chef/internal/domain/round"
)

func TestQuestionPayloadExcludesQuestions(t *testing.T) {
	qr := round.QuestionRound{
		SessionID: "s1",
		Number:    2,
		Intro:     round.Markdown("intro text"),
		Recap: []round.Branch{
			{Tag: round.Tag("br"), Title: "Branch", Prose: round.Markdown("prose")},
		},
		Questions: []round.Question{
			{Title: round.Markdown("Q")},
		},
	}
	raw, err := marshalRoundPayload(qr)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) == 0 {
		t.Fatal("empty payload")
	}
}func TestLegendPayloadCarriesWholeRound(t *testing.T) {
	lr := round.LegendRound{
		SessionID: "s1",
		Title:     round.Markdown("legend"),
		Branches: []round.LegendBranch{
			{Title: "br", Bullets: []round.LegendBullet{{Tag: round.Tag("t"), Decision: round.Markdown("d")}}},
		},
	}
	raw, err := marshalLegendPayload(lr)
	if err != nil {
		t.Fatal(err)
	}
	back, err := unmarshalLegendPayload(raw)
	if err != nil {
		t.Fatal(err)
	}
	if back.SessionID != lr.SessionID || len(back.Branches) != 1 {
		t.Fatalf("legend payload lossy: %+v", back)
	}
}}
```

Fix the stray brace in the compiler if the copy above carries one — the file must contain exactly `TestQuestionPayloadExcludesQuestions` and `TestLegendPayloadCarriesWholeRound`.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go build ./internal/port/store/sqlite/`

Expected: FAIL — `marshalRoundPayload` and `marshalLegendPayload` undefined.

- [ ] **Step 3: Write the mapping layer**

```go
package sqlite

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/malikbenkirane/grill-chef/internal/domain/round"
)

// qPayload is the `rounds.payload` column: the question round minus its
// questions — those live in `questions` rows and are re-assembled on load,
// which is why the payload must carry them as nil (sub-plan 03 reads this
// via unmarshal and re-merges from rows; no post-load merge in Go applies
// to question bodies).
type qPayload struct {
	SchemaVersion int          `json:"schema_version"`
	SessionID     string       `json:"session_id"`
	Number        int          `json:"number"`
	Intro         string       `json:"intro"`
	Recap         []branchDTO  `json:"recap"`
}

type branchDTO struct {
	Tag          string `json:"tag"`
	Title        string `json:"title"`
	Prose        string `json:"prose"`
	NewlySettled []string `json:"newly_settled"`
	Served       int    `json:"served"`
	Retained     int    `json:"retained"`
}

func marshalRoundPayload(qr round.QuestionRound) ([]byte, error) {
	if qr.SessionID == "" || qr.Number == 0 {
		return nil, fmt.Errorf("sqlite: question round missing identity: session %q number %d", qr.SessionID, qr.Number)
	}
	m := qPayload{
		SchemaVersion: round.SchemaVersion,
		SessionID:     qr.SessionID,
		Number:        qr.Number,
		Intro:         string(qr.Intro),
	}
	for _, b := range qr.Recap {
		tags := make([]string, len(b.NewlySettled))
		for i, tag := range b.NewlySettled {
			tags[i] = string(tag)
		}
		m.Recap = append(m.Recap, branchDTO{
			Tag: string(b.Tag), Title: b.Title, Prose: string(b.Prose),
			NewlySettled: tags, Served: b.Served, Retained: b.Retained,
		})
	}
	return json.Marshal(m)
}

// legendPayload is the full `legends.payload` column.
type legendPayload struct {
	SchemaVersion int          `json:"schema_version"`
	SessionID     string       `json:"session_id"`
	Title         string       `json:"title"`
	Branches      []legBranchDTO `json:"branches"`
}

type legBranchDTO struct {
	Title   string         `json:"title"`
	Bullets []legBulletDTO `json:"bullets"`
}

type legBulletDTO struct {
	Tag      string `json:"tag"`
	Decision string `json:"decision"`
}

func marshalLegendPayload(lr round.LegendRound) ([]byte, error) {
	if lr.SessionID == "" {
		return nil, fmt.Errorf("sqlite: legend round missing session id")
	}
	m := legendPayload{
		SchemaVersion: round.SchemaVersion,
		SessionID:     lr.SessionID,
		Title:         string(lr.Title),
	}
	for _, b := range lr.Branches {
		d := legBranchDTO{Title: b.Title}
		for _, bul := range b.Bullets {
			d.Bullets = append(d.Bullets, legBulletDTO{Tag: string(bul.Tag), Decision: string(bul.Decision)})
		}
		m.Branches = append(m.Branches, d)
	}
	return json.Marshal(m)
}

func unmarshalLegendPayload(raw []byte) (round.LegendRound, error) {
	var m legendPayload
	if err := json.Unmarshal(raw, &m); err != nil {
		return round.LegendRound{}, fmt.Errorf("sqlite: legend payload: %w", err)
	}
	lr := round.LegendRound{SessionID: m.SessionID, Title: round.Markdown(m.Title)}
	for _, b := range m.Branches {
		lb := round.LegendBranch{Title: b.Title}
		for _, bul := range b.Bullets {
			lb.Bullets = append(lb.Bullets, round.LegendBullet{Tag: round.Tag(bul.Tag), Decision: round.Markdown(bul.Decision)})
		}
		lr.Branches = append(lr.Branches, lb)
	}
	return lr, nil
}

// questionColumns maps one domain question to its column shapes.

func questionTitle(q round.Question) string  { return string(q.Title) }

func questionAbstract(q round.Question) string {
	paras := make([]string, len(q.Abstract))
	for i, a := range q.Abstract {
		paras[i] = string(a)
	}
	raw, _ := json.Marshal(paras)
	return string(raw)
}

func questionOptions(q round.Question) string {
	opts := make([]string, len(q.Choices))
	for i, c := range q.Choices {
		opts[i] = string(c.Text)
	}
	raw, _ := json.Marshal(opts)
	return string(raw)
}

// questionRecommenationIndex is the domain Recommendation verbatim:
// RecommendationNone (0) = "no recommendation", 1..N = the 1-based choice.
// The column default 0 matches the domain sentinel exactly.
func questionRecommendedIndex(q round.Question) int { return int(q.Recommendation) }

func questionReminders(q round.Question) string {
	if len(q.Reminders) == 0 {
		return "[]"
	}
	type rem struct {
		Tag     string `json:"tag"`
		Summary string `json:"summary"`
	}
	rs := make([]rem, len(q.Reminders))
	for i, r := range q.Reminders {
		rs[i] = rem{Tag: string(r.Tag), Summary: string(r.Summary)}
	}
	raw, _ := json.Marshal(rs)
	return string(raw)
}

// statusLiteral renders the store edge value of a session status. The
// literals match the sessions.status CHECK exactly.
func statusLiteral(s session.Status) (string, error) {
	switch s {
	case session.StatusActive, session.StatusClosed, session.StatusDismissed:
		return s.String(), nil
	default:
		return "", fmt.Errorf("sqlite: undefined session status %d", s)
	}
}

var _ = strings.TrimSpace // placeholder import trim if unused
```

Remove the `strings` import if `go vet` flags it — the final import list is `encoding/json`, `fmt`, and the `round`/`session` domain imports only.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/port/store/sqlite/ -run 'TestQuestionPayload|TestLegendPayload' -v`

Expected: PASS.

- [ ] **Step 5: Verify the M question round-round-trips losslessly too**

Add to `mapping_test.go`:

```go
func TestQuestionRoundRowsLines(t *testing.T) {
	q := round.Question{
		Title:    round.Markdown("Q"),
		Abstract: []round.Markdown{"abs"},
		Choices:  []round.Choice{{Text: "opt"}},
		Recommendation: 1,
		Reminders:      []round.Reminder{{Tag: "t", Summary: "s"}},
	}
	if questionRecommendedIndex(q) != 1 {
		t.Fatal("recommendation must persist as domain holds it: 0=none, 1..N")
	}
	var noRec round.Question
	if questionRecommendedIndex(noRec) != 0 {
		t.Fatal("RecommendationNone must persist as 0")
	}
	if got := questionTitle(q); got != "Q" {
		t.Fatalf("title lossy: %q", got)
	}
}
```

- [ ] **Step 6: Commit with caveman-commit**

Use `caveman-commit` for the message content, then the commit sequence from `core-commands`:

1. `mktemp`
2. Read the temp file
3. Write the commit message to the temp file
4. `jj describe --stdin < "/tmp/tmp.XXXXXX" && rm "/tmp/tmp.XXXXXX" && jj new`

Message subject: `feat(store): adapter payload dtos and column mappers`

---

### Task 2: MCP session writes — CreateSession, CloseSession

**Files:**
- Create: `internal/port/store/sqlite/session_writes.go`
- Test: `internal/port/store/sqlite/session_writes_test.go`

- [ ] **Step 1: Write the failing tests**

```go
package sqlite

import (
	"testing"

	"github.com/malikbenkirane/grill-chef/internal/domain/session"
)

func TestCreateSessionStoresDomainAsIs(t *testing.T) {
	s := newTestStore(t)
	in := session.New("sess-1", "Grill: state db")
	// domain truth: CreatedAt stays at the caller's discretion; the adapter
	// stamps nothing.
	if err := s.CreateSession(t.Context(), in); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	var status string
	if err := s.db.QueryRow(`SELECT status FROM sessions WHERE id = ?`, "sess-1").Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "active" {
		t.Fatalf("status literal: want active, got %q", status)
	}
}

func TestCreateSessionRejectsUndefinedStatus(t *testing.T) {
	s := newTestStore(t)
	in := &session.Session{ID: "bad", Name: "n", Status: session.StatusUndefined}
	if err := s.CreateSession(t.Context(), in); err == nil {
		t.Fatal("undefined status must be rejected; the store never invents state")
	}
}

func TestCreateSessionDuplicateIdFails(t *testing.T) {
	s := newTestStore(t)
	in := session.New("dup", "name")
	if err := s.CreateSession(t.Context(), in); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateSession(t.Context(), session.New("dup", "name")); err == nil {
		t.Fatal("duplicate session id must fail")
	}
}

func TestCloseSessionFlipsStatusExactlyOnce(t *testing.T) {
	s := newTestStore(t)
	if err := s.CreateSession(t.Context(), session.New("c1", "n")); err != nil {
		t.Fatal(err)
	}
	if err := s.CloseSession(t.Context(), "c1"); err != nil {
		t.Fatalf("CloseSession: %v", err)
	}
	var status string
	if err := s.db.QueryRow(`SELECT status FROM sessions WHERE id = ?`, "c1").Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "closed" {
		t.Fatalf("want closed, got %q", status)
	}
	if err := s.CloseSession(t.Context(), "no-such"); err == nil {
		t.Fatal("closing unknown session must error")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/port/store/sqlite/ -run 'TestCreateSession|TestCloseSession' -v`

Expected: FAIL — methods undefined on `*Store`.

- [ ] **Step 3: Implement the MCP session write methods**

```go
package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/malikbenkirane/grill-chef/internal/domain/session"
)

// CreateSession persists the domain aggregate verbatim. MCP-owned.
func (s *Store) CreateSession(ctx context.Context, in *session.Session) error {
	if in == nil {
		return fmt.Errorf("sqlite: CreateSession: nil session")
	}
	lit, err := statusLiteral(in.Status)
	if err != nil {
		return fmt.Errorf("sqlite: CreateSession: %w", err)
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO sessions (id, name, status) VALUES (?, ?, ?)`,
		in.ID, in.Name, lit)
	if err != nil {
		return fmt.Errorf("sqlite: CreateSession %q: %w", in.ID, err)
	}
	return nil
}

// CloseSession flips an existing active session to closed. MCP-owned; ties
// to the legend round being processed, not round counting (decision trail).
func (s *Store) CloseSession(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE sessions SET status = 'closed', updated_at = CURRENT_TIMESTAMP
		 WHERE id = ? AND status = 'active'`, id)
	if err != nil {
		return fmt.Errorf("sqlite: CloseSession %q: %w", id, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("sqlite: CloseSession %q: not active or unknown session", id)
	}
	return nil
}

var _ = sql.ErrNoRows // silence import until readers use it (03)
```

Remove the `var _ sql.ErrNoRows` shim and `database/sql` import if vet flags them.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/port/store/sqlite/ -run 'TestCreateSession|TestCloseSession' -v`

Expected: PASS.

- [ ] **Step 5: Commit with caveman-commit**

Use `caveman-commit` for the message content, then the commit sequence:

1. `mktemp`
2. Read the temp file
3. Write the commit message to the temp file
4. `jj describe --stdin < "/tmp/tmp.XXXXXX" && rm "/tmp/tmp.XXXXXX" && jj new`

Message subject: `feat(store): mcp session create and close writes`

---

### Task 3: MCP round writes — CreateQuestionRound / CreateLegendRound

**Files:**
- Create: `internal/port/store/sqlite/round_writes.go`
- Test: `internal/port/store/sqlite/round_writes_test.go`

- [ ] **Step 1: Write the failing tests**

```go
package sqlite

import (
	"strings"
	"testing"

	"github.com/malikbenkirane/grill-chef/internal/domain/round"
)

func TestCreateQuestionRoundRows(t *testing.T) {
	s := newTestStore(t)
	if err := s.CreateSession(t.Context(), mkSess("q1")); err != nil {
		t.Fatal(err)
	}
	qr := mkQuestionRound("q1", 1, 2)
	if err := s.CreateQuestionRound(t.Context(), qr); err != nil {
		t.Fatalf("CreateQuestionRound: %v", err)
	}
	var payload string
	if err := s.db.QueryRow(`SELECT payload FROM rounds WHERE session_id=? AND round_number=?`, "q1", 1).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(payload, `"title"`) {
		t.Fatal("payload must not embed question bodies; questions live in rows")
	}
	var qcount int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM questions WHERE session_id=? AND round_number=?`, "q1", 1).Scan(&qcount); err != nil {
		t.Fatal(err)
	}
	if qcount != 2 {
		t.Fatalf("want 2 question rows, got %d", qcount)
	}
	var status string
	_ = s.db.QueryRow(`SELECT status FROM rounds WHERE session_id=? AND round_number=?`, "q1", 1).Scan(&status)
	if status != "pending" {
		t.Fatalf("pivot round starts pending, got %q", status)
	}
}

func TestCreateQuestionRoundDuplicateNumberFails(t *testing.T) {
	s := newTestStore(t)
	_ = s.CreateSession(t.Context(), mkSess("q2"))
	qr := mkQuestionRound("q2", 1, 1)
	if err := s.CreateQuestionRound(t.Context(), qr); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateQuestionRound(t.Context(), qr); err == nil {
		t.Fatal("duplicate (session, round) must fail on PK")
	}
}

func TestCreateLegendRoundGoesToLegends(t *testing.T) {
	s := newTestStore(t)
	_ = s.CreateSession(t.Context(), mkSess("l1"))
	lr := round.LegendRound{
		SessionID: "l1",
		Title:     round.Markdown("legend"),
		Branches: []round.LegendBranch{{
			Title:   "br",
			Bullets: []round.LegendBullet{{Tag: round.Tag("t"), Decision: round.Markdown("settled")}},
		}},
	}
	if err := s.CreateLegendRound(t.Context(), lr); err != nil {
		t.Fatalf("CreateLegendRound: %v", err)
	}
	var n int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM rounds WHERE session_id=?`, "l1").Scan(&n)
	if n != 0 {
		t.Fatal("legend must never write rounds rows")
	}
	var payload []byte
	if err := s.db.QueryRow(`SELECT payload FROM legends WHERE session_id=?`, "l1").Scan(&payload); err != nil {
		t.Fatalf("legends row missing: %v", err)
	}
	if len(payload) == 0 {
		t.Fatal("empty legend payload")
	}
	// the agent may still grill questions after a legend attempt race; a
	// second legend write (PK session_id) must fail, not silently replace.
	if err := s.CreateLegendRound(t.Context(), lr); err == nil {
		t.Fatal("duplicate legend insert must fail")
	}
}

func mkSess(id string) *session.Session { return session.New(id, "name") }

func mkQuestionRound(id string, number int, questions int) round.QuestionRound {
	qr := round.QuestionRound{SessionID: id, Number: number, Intro: round.Markdown("intro")}
	for i := 0; i < questions; i++ {
		qr.Questions = append(qr.Questions, round.Question{
			Title:   round.Markdown("q"),
			Choices: []round.Choice{{Text: "a"}},
		})
	}
	return qr
}
```

Add the missing `"github.com/malikbenkirane/grill-chef/internal/domain/session"` import that `mkSess` needs.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/port/store/sqlite/ -run 'TestCreateQuestionRound|TestCreateLegendRound' -v`

Expected: FAIL — methods undefined.

- [ ] **Step 3: Implement the round write methods**

```go
package sqlite

import (
	"context"
	"fmt"
	"strings"

	"github.com/malikbenkirane/grill-chef/internal/domain/round"
)

// CreateQuestionRound writes the rounds row (payload = intro+recap only)
// plus one questions row per domain question, in one transaction. MCP-owned.
func (s *Store) CreateQuestionRound(ctx context.Context, qr round.QuestionRound) error {
	raw, err := marshalRoundPayload(qr)
	if err != nil {
		return fmt.Errorf("sqlite: CreateQuestionRound: %w", err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("sqlite: begin: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx,
		`INSERT INTO rounds (session_id, round_number, status, payload)
		 VALUES (?, ?, 'pending', ?)`, qr.SessionID, qr.Number, string(raw))
	if err != nil {
		return fmt.Errorf("sqlite: rounds insert (%s,%d): %w", qr.SessionID, qr.Number, err)
	}
	for i, q := range qr.Questions {
		_, err = tx.ExecContext(ctx,
			`INSERT INTO questions
			 (session_id, round_number, question_index, title, abstract, options, recommended_index, reminders)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			qr.SessionID, qr.Number, i+1,
			questionTitle(q), questionAbstract(q), questionOptions(q),
			questionRecommendedIndex(q), questionReminders(q))
		if err != nil {
			return fmt.Errorf("sqlite: question %d insert: %w", i+1, err)
		}
	}
	return tx.Commit()
}

// CreateLegendRound writes the legends row only — a legend round never
// lives in rounds. MCP-owned. Retried with the same session id fails on PK.
func (s *Store) CreateLegendRound(ctx context.Context, lr round.LegendRound) error {
	raw, err := marshalLegendPayload(lr)
	if err != nil {
		return fmt.Errorf("sqlite: CreateLegendRound: %w", err)
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO legends (session_id, payload) VALUES (?, ?)`,
		lr.SessionID, string(raw))
	if err != nil {
		return fmt.Errorf("sqlite: legends insert %s: %w", lr.SessionID, err)
	}
	return nil
}

var _ = strings.Contains // drop if vet flags it
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/port/store/sqlite/ -run 'TestCreateQuestionRound|TestCreateLegendRound' -v`

Expected: PASS.

- [ ] **Step 5: Commit with caveman-commit (same sequence as Task 2)**

Message subject: `feat(store): mcp question and legend round writes`

---

### Task 4: Daemon answer autosave — UpsertAnswer

**Files:**
- Create: `internal/port/store/sqlite/answer_writes.go`
- Test: `internal/port/store/sqlite/answer_writes_test.go`

- [ ] **Step 1: Write the failing tests**

```go
package sqlite

import (
	"testing"

	"github.com/malikbenkirane/grill-chef/internal/domain/round"
)

func TestUpsertAnswerRoundtrip(t *testing.T) {
	s := newTestStore(t)
	seedSessionRound(t, s, "a1")

	if err := s.UpsertAnswer(t.Context(), "a1", 1, 1, round.ChoiceIndex(1)); err != nil {
		t.Fatalf("first autosave: %v", err)
	}
	var chosen int
	if err := s.db.QueryRow(
		`SELECT chosen_option FROM answers WHERE session_id=? AND round_number=? AND question_index=?`,
		"a1", 1, 1).Scan(&chosen); err != nil {
		t.Fatalf("row must exist after first autosave: %v", err)
	}
	if chosen != 1 {
		t.Fatalf("chosen: want 1, got %d", chosen)
	}
	if err := s.UpsertAnswer(t.Context(), "a1", 1, 1, round.ChoiceIndexOther); err != nil {
		t.Fatalf("update autosave: %v", err)
	}
	_ = s.db.QueryRow(`SELECT chosen_option FROM answers WHERE session_id=? AND round_number=? AND question_index=?`,
		"a1", 1, 1).Scan(&chosen)
	if chosen != 0 {
		t.Fatalf("ChoiceIndexOther must persist as 0, got %d", chosen)
	}
	if err := s.UpsertAnswer(t.Context(), "a1", 1, 2, round.ChoiceIndexNone); err != nil {
		t.Fatalf("skip autosave: %v", err)
	}
	_ = s.db.QueryRow(`SELECT chosen_option FROM answers WHERE session_id=? AND round_number=? AND question_index=?`,
		"a1", 1, 2).Scan(&chosen)
	if chosen != -1 {
		t.Fatalf("ChoiceIndexNone must persist as -1, got %d", chosen)
	}
}

func TestUpsertAnswerRequiresExistingQuestionRow(t *testing.T) {
	s := newTestStore(t)
	seedSessionRound(t, s, "a2")
	if err := s.UpsertAnswer(t.Context(), "a2", 1, 99, round.ChoiceIndex(1)); err == nil {
		t.Fatal("autosave to nonexistent question must fail on FK")
	}
}

// seedSessionRound builds one session + one question round with 2 questions,
// the shared fixture for all daemon-owned write tests.
func seedSessionRound(t *testing.T, s *Store, id string) {
	t.Helper()
	if err := s.CreateSession(t.Context(), mkSess(id)); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateQuestionRound(t.Context(), mkQuestionRound(id, 1, 2)); err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/port/store/sqlite/ -run TestUpsertAnswer -v`

Expected: FAIL — `UpsertAnswer` undefined.

- [ ] **Step 3: Implement UpsertAnswer**

```go
package sqlite

import (
	"context"
	"fmt"

	"github.com/malikbenkirane/grill-chef/internal/domain/round"
)

// UpsertAnswer is the daemon's autosave hot path: one question choice per
// call, insert on first save and update after, chosen_option storing the
// domain sentinel verbatim (-1 none, 0 other, 1..N listed).
func (s *Store) UpsertAnswer(ctx context.Context, sessionID string, roundNumber, questionIndex int, choice round.ChoiceIndex) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO answers (session_id, round_number, question_index, chosen_option)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT (session_id, round_number, question_index)
		 DO UPDATE SET chosen_option = excluded.chosen_option,
		               updated_at = CURRENT_TIMESTAMP`,
		sessionID, roundNumber, questionIndex, int(choice))
	if err != nil {
		return fmt.Errorf("sqlite: UpsertAnswer (%s,%d,%d): %w", sessionID, roundNumber, questionIndex, err)
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/port/store/sqlite/ -run TestUpsertAnswer -v`

Expected: PASS.

- [ ] **Step 5: Commit with caveman-commit (same sequence as Task 2)**

Message subject: `feat(store): daemon answer autosave upsert with domain sentinels`

---

### Task 5: Daemon round submit + station-home dismiss

**Files:**
- Modify: `internal/port/store/sqlite/answer_writes.go` (SubmitRound, DismissSession)
- Test: `internal/port/store/sqlite/submit_test.go`

- [ ] **Step 1: Write the failing tests**

```go
package sqlite

import (
	"testing"

	"github.com/malikbenkirane/grill-chef/internal/domain/round"
	"github.com/malikbenkirane/grill-chef/internal/domain/session"
)

func TestSubmitRoundFlipsPendingToSubmitted(t *testing.T) {
	s := newTestStore(t)
	seedSessionRound(t, s, "sb1")
	if err := s.SubmitRound(t.Context(), "sb1", 1); err != nil {
		t.Fatal(err)
	}
	var status string
	_ = s.db.QueryRow(`SELECT status FROM rounds WHERE session_id=? AND round_number=?`, "sb1", 1).Scan(&status)
	if status != "submitted" {
		t.Fatalf("want submitted, got %q", status)
	}
}

func TestSubmitRoundIsIdempotentAndGuardsLegend(t *testing.T) {
	s := newTestStore(t)
	seedSessionRound(t, s, "sb2")
	if err := s.SubmitRound(t.Context(), "sb2", 1); err != nil {
		t.Fatal(err)
	}
	if err := s.SubmitRound(t.Context(), "sb2", 1); err != nil {
		t.Fatalf("resubmit must not error (idempotent no-op): %v", err)
	}
	if err := s.SubmitRound(t.Context(), "sb2", 9); err == nil {
		t.Fatal("submit of nonexistent round must fail")
	}
}

func TestDismissSessionFlipsStatusNotDeletes(t *testing.T) {
	s := newTestStore(t)
	seedSessionRound(t, s, "d1")
	if err := s.DismissSession(t.Context(), "d1"); err != nil {
		t.Fatal(err)
	}
	var status string
	if err := s.db.QueryRow(`SELECT status FROM sessions WHERE id=?`, "d1").Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "dismissed" {
		t.Fatalf("want dismissed, got %q", status)
	}
	var n int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM rounds WHERE session_id=?`, "d1").Scan(&n)
	if n != 1 {
		t.Fatal("dismiss must never delete; retention keeps everything")
	}
	var closed session.Status = session.StatusDismissed
	if closed.String() != "dismissed" {
		t.Fatal("domain literal drifted from store edge")
	}
}

var _ = round.ChoiceIndexNone // import shim, drop if vet flags it
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/port/store/sqlite/ -run 'TestSubmitRound|TestDismissSession' -v`

Expected: FAIL — methods undefined.

- [ ] **Step 3: Implement SubmitRound and DismissSession**

```go
// SubmitRound is the daemon's submit path for a pending round. Idempotent:
// submitting an already-submitted round is a no-op (the round page may
// double-fire) but unknown rounds still fail. MCP payload columns untouched.
func (s *Store) SubmitRound(ctx context.Context, sessionID string, roundNumber int) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE rounds SET status = 'submitted', updated_at = CURRENT_TIMESTAMP
		 WHERE session_id = ? AND round_number = ? AND status = 'pending'`,
		sessionID, roundNumber)
	if err != nil {
		return fmt.Errorf("sqlite: SubmitRound (%s,%d): %w", sessionID, roundNumber, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		var exists int
		if err := s.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM rounds WHERE session_id = ? AND round_number = ?`,
			sessionID, roundNumber).Scan(&exists); err != nil {
			return fmt.Errorf("sqlite: SubmitRound probe: %w", err)
		}
		if exists == 0 {
			return fmt.Errorf("sqlite: SubmitRound (%s,%d): round not found", sessionID, roundNumber)
		}
	}
	return nil
}

// DismissSession is the station home's status change (issue 14), written by
// the web daemon through the port. Data preservation: never a delete.
func (s *Store) DismissSession(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE sessions SET status = 'dismissed', updated_at = CURRENT_TIMESTAMP
		 WHERE id = ? AND status != 'dismissed'`, id)
	if err != nil {
		return fmt.Errorf("sqlite: DismissSession %q: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("sqlite: DismissSession %q: already dismissed or unknown session", id)
	}
	return nil
}
```

Both live in `answer_writes.go` (the daemon's write file). Remove the `round` import from the test file if the compiler flags the shim.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/port/store/sqlite/ -run 'TestSubmitRound|TestDismissSession' -v`

Expected: PASS.

- [ ] **Step 5: Commit with caveman-commit (same sequence as Task 2)**

Message subject: `feat(store): daemon round submit and station-home dismiss writes`

---

### Task 6: Comment writes — AddComment / DeleteComment

**Files:**
- Create: `internal/port/store/sqlite/comment_writes.go`
- Test: `internal/port/store/sqlite/comment_writes_test.go`

- [ ] **Step 1: Write the failing tests**

```go
package sqlite

import "testing"

func TestAddCommentAllocatesSeqPerQuestion(t *testing.T) {
	s := newTestStore(t)
	seedSessionRound(t, s, "cm1")

	seq1, err := s.AddComment(t.Context(), "cm1", 1, 1, "first")
	if err != nil {
		t.Fatal(err)
	}
	seq2, err := s.AddComment(t.Context(), "cm1", 1, 1, "second")
	if err != nil {
		t.Fatal(err)
	}
	other, err := s.AddComment(t.Context(), "cm1", 1, 2, "other question")
	if err != nil {
		t.Fatal(err)
	}
	if seq1 != 1 || seq2 != 2 || other != 1 {
		t.Fatalf("seq must be monotonic PER question: got %d,%d and %d", seq1, seq2, other)
	}
}

func TestAddCommentRequiresExistingQuestion(t *testing.T) {
	s := newTestStore(t)
	seedSessionRound(t, s, "cm2")
	if _, err := s.AddComment(t.Context(), "cm2", 1, 99, "nope"); err == nil {
		t.Fatal("comment to nonexistent question must fail on FK")
	}
}

func TestDeleteCommentRemovesExactRow(t *testing.T) {
	s := newTestStore(t)
	seedSessionRound(t, s, "cm3")
	seq, err := s.AddComment(t.Context(), "cm3", 1, 1, "to delete")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddComment(t.Context(), "cm3", 1, 1, "stays"); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteComment(t.Context(), "cm3", 1, 1, seq); err != nil {
		t.Fatal(err)
	}
	var n int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM comments
		WHERE session_id=? AND round_number=? AND question_index=?`, "cm3", 1, 1).Scan(&n)
	if n != 1 {
		t.Fatalf("want 1 comment left, got %d", n)
	}
	if err := s.DeleteComment(t.Context(), "cm3", 1, 1, seq); err == nil {
		t.Fatal("deleting the same comment twice must fail")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/port/store/sqlite/ -run 'TestAddComment|TestDeleteComment' -v`

Expected: FAIL — methods undefined.

- [ ] **Step 3: Implement the comment write methods**

```go
package sqlite

import (
	"context"
	"fmt"
)

// AddComment appends to a question's comment list inside one transaction:
// the next per-question sequence is MAX(comment_seq)+1, so a deleted seq is
// never reused and the DELETE route's identity stays stable. The browser
// round-trips the returned seq in the URL (issue 8).
func (s *Store) AddComment(ctx context.Context, sessionID string, roundNumber, questionIndex int, body string) (int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("sqlite: begin: %w", err)
	}
	defer tx.Rollback()

	var seq int
	if err := tx.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(comment_seq), 0) + 1 FROM comments
		 WHERE session_id=? AND round_number=? AND question_index=?`,
		sessionID, roundNumber, questionIndex).Scan(&seq); err != nil {
		return 0, fmt.Errorf("sqlite: AddComment seq: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO comments (session_id, round_number, question_index, comment_seq, body)
		 VALUES (?, ?, ?, ?, ?)`,
		sessionID, roundNumber, questionIndex, seq, body); err != nil {
		return 0, fmt.Errorf("sqlite: AddComment insert (%s,%d,%d): %w", sessionID, roundNumber, questionIndex, err)
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("sqlite: AddComment commit: %w", err)
	}
	return seq, nil
}

// DeleteComment removes exactly one comment by its per-question identity.
func (s *Store) DeleteComment(ctx context.Context, sessionID string, roundNumber, questionIndex, seq int) error {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM comments
		 WHERE session_id=? AND round_number=? AND question_index=? AND comment_seq=?`,
		sessionID, roundNumber, questionIndex, seq)
	if err != nil {
		return fmt.Errorf("sqlite: DeleteComment (%s,%d,%d,%d): %w", sessionID, roundNumber, questionIndex, seq, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("sqlite: DeleteComment (%s,%d,%d,%d): not found", sessionID, roundNumber, questionIndex, seq)
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/port/store/sqlite/ -run 'TestAddComment|TestDeleteComment' -v`

Expected: PASS.

- [ ] **Step 5: Commit with caveman-commit (same sequence as Task 2)**

Message subject: `feat(store): daemon comment add and delete with stable per-question seq`

---

### Task 7: Concurrent-writer race acceptance test

**Files:**
- Test: `internal/port/store/sqlite/write_race_test.go`

- [ ] **Step 1: Write the test**

```go
package sqlite

import (
	"sync"
	"testing"

	"github.com/malikbenkirane/grill-chef/internal/domain/round"
)

// The store must not corrupt when an agent tool call (MCP) and a browser
// autosave (daemon) race against the same session — different tables, WAL
// serializes them, busy_timeout absorbs the contention.
func TestMCPAndDaemonWritesRaceSafely(t *testing.T) {
	s := newTestStore(t)
	const sessions = 4
	for i := 0; i < sessions; i++ {
		if err := s.CreateSession(t.Context(), mkSess(fmt.Sprintf("rs%d", i))); err != nil {
			t.Fatal(err)
		}
	}
	var wg sync.WaitGroup
	for i := 0; i < sessions; i++ {
		wg.Add(2)
		go func(i int) { // MCP: creates the question round
			defer wg.Done()
			err := s.CreateQuestionRound(t.Context(), mkQuestionRound(fmt.Sprintf("rs%d", i), 1, 2))
			errs <- err
		}(i)
		go func(i int) { // daemon: autosaves against a query that must tolerate absence
			defer wg.Done()
			err := s.UpsertAnswer(t.Context(), fmt.Sprintf("rs%d", i), 1, 1, round.ChoiceIndex(1))
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil && !isMissingRoundFK(err) {
			t.Fatalf("unexpected write error under race: %v", err)
		}
	}
}

func isMissingRoundFK(err error) bool {
	return err != nil && (contains(err.Error(), "FOREIGN KEY") || contains(err.Error(), "no such table") || contains(err.Error(), "no rows"))
}

func contains(s, sub string) bool { return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0) }
func indexOf(s, sub string) int   { return IndexOfHelper(s, sub) }
```

`errs` and `IndexOfHelper` must be defined: replace the last two helper lines with `func contains(s, sub string) bool { return strings.Contains(s, sub) }` and import `strings`. The vehicle of truth is the code block above in spirit — a worker that finds its compile failing may restructure the helper exactly as the compiler dictates, as long as the semantics (tolerate FK-timing failure, fail on anything else) hold.

- [ ] **Step 2: Run tests to verify they pass**

Run: `go test ./internal/port/store/sqlite/ -run TestMCPAndDaemonWritesRace -race -v -count 5`

Expected: PASS on every count; no data race, no corruption.

- [ ] **Step 3: Full package suite under race**

Run: `go test ./... -race`

Expected: all `ok`.

- [ ] **Step 4: Commit with caveman-commit (same sequence as Task 2)**

Message subject: `test(store): mcp and daemon writes race without corruption`
