# State database & session storage — 03-reads-recovery Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use supervised-plan-execution to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement load-side recovery: full-session rehydration as a single indexed read, the gap-based `IsSessionConsistent` correctness check, staleness surfacing through the domain's `UpdatedAt` fields, and the `daemon.json` marker file that recovery tooling and the human `stop` CLI read.

**Architecture:** Load paths are shared readers: `LoadSession` reassembles the domain aggregate (`*session.Session` with its `round.QuestionRound`/`round.LegendRound` rounds, carrying `UpdatedAt` and positionally aligned `Answers`) from rows — question bodies come from rows, never re-parsed from payload JSON. `IsSessionConsistent` is domain policy at the store edge: contiguous 1..N round numbers, full question set on every `pending` round. The marker file lives outside the db (`os.UserCacheDir()+"/grill-chef/daemon.json"`), rebuilt atomically on listener bind, read without touching the db or its WAL sidecars.

**Tech Stack:** Go 1.22+, stdlib `database/sql`, `encoding/json`, `os`, `go test -race`.

**Normative sources:** `docs/issues/1.md` (consistency + recovery requirements), `docs/drafts/state.md`, the port in `internal/port/store/port.go` (sub-plan 01), the write paths and `seedSessionRound`/`mkQuestionRound` fixtures of sub-plan 02.

---

### Task 1: Domain Gallagher — aligned answers on the question round

**Files:**
- Modify: `internal/domain/round/round.go`
- Test: `internal/domain/round/answers_field_test.go`

- [x] **Step 1: Write the failing test**

```go
package round

import "testing"

func TestQuestionRoundCarriesAlignedAnswers(t *testing.T) {
	qr := QuestionRound{SessionID: "s", Number: 1}
	if len(qr.Answers) != 0 {
		t.Fatal("fresh round must have no answers")
	}
	qr.Answers = []Answer{{ChoiceIndex: ChoiceIndexOther, Comments: []Markdown{"note"}}}
	if qr.Answers[0].ChoiceIndex != ChoiceIndexOther {
		t.Fatal("answers field not assignable")
	}
}
```

- [x] **Step 2: Run test to verify it fails**

Run: `go test ./internal/domain/round/ -run TestQuestionRoundCarriesAlignedAnswers -v`

Expected: FAIL — field undefined.

- [x] **Step 3: Add the field**

In `internal/domain/round/round.go`, inside `QuestionRound`, add after `UpdatedAt`:

```go
	// Answers holds one rehydrated result per question, positionally
	// aligned 1..len(Questions) Answer values mapped by question_index
	// from the store; a nil/absent slot means the question was never
	// autosaved. AnswerSheet is the write-side sibling for submit
	// semantics; this field is the rehydrate-side home on the round.
	Answers []Answer
```

- [x] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/domain/... -race`

Expected: PASS.

- [x] **Step 5: Commit with caveman-commit**

Use `caveman-commit` for the message content, then the commit sequence from `core-commands`:

1. `mktemp`
2. Read the temp file
3. Write the commit message to the temp file
4. `jj describe --stdin < "/tmp/tmp.XXXXXX" && rm "/tmp/tmp.XXXXXX" && jj new`

Message subject: `feat(domain): aligned rehydrated answers on question round`

---

### Task 2: Single-read LoadSession rehydration

**Files:**
- Create: `internal/port/store/sqlite/readers.go`
- Test: `internal/port/store/sqlite/readers_test.go`

- [x] **Step 1: Write the failing test**

```go
package sqlite

import (
	"testing"

	"github.com/malikbenkirane/grill-chef/internal/domain/round"
)

func TestLoadSessionRehydrates(t *testing.T) {
	s := newTestStore(t)
	id := "ld1"
	if err := s.CreateSession(t.Context(), mkSess(id)); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateQuestionRound(t.Context(), mkQuestionRound(id, 1, 2)); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertAnswer(t.Context(), id, 1, 1, round.ChoiceIndexOther); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddComment(t.Context(), id, 1, 1, "note"); err != nil {
		t.Fatal(err)
	}

	loaded, err := s.LoadSession(t.Context(), id)
	if err != nil {
		t.Fatalf("LoadSession: %v", err)
	}
	if loaded.ID != id || loaded.Status != session.StatusActive {
		t.Fatalf("header wrong: %+v", loaded)
	}
	if len(loaded.Rounds) != 1 {
		t.Fatalf("want 1 round, got %d", len(loaded.Rounds))
	}
	qr, ok := loaded.Rounds[0].(round.QuestionRound)
	if !ok {
		t.Fatalf("want QuestionRound, got %T", loaded.Rounds[0])
	}
	if qr.Number != 1 || string(qr.Intro) != "intro" || len(qr.Questions) != 2 {
		t.Fatalf("question round lossy: %+v", qr)
	}
	if qr.UpdatedAt.IsZero() {
		t.Fatal("round UpdatedAt must rehydrate from rows")
	}
	if len(qr.Answers) != 2 {
		t.Fatalf("answers must align 1..2, got %d", len(qr.Answers))
	}
	if qr.Answers[0].ChoiceIndex != round.ChoiceIndexOther {
		t.Fatalf("answer 1 choice: want Other, got %v", qr.Answers[0].ChoiceIndex)
	}
	if qr.Answers[1].ChoiceIndex != round.ChoiceIndexNone {
		t.Fatalf("unanswered slot must rehydrate as ChoiceIndexNone, got %v", qr.Answers[1].ChoiceIndex)
	}
	if len(qr.Answers[0].Comments) != 1 || string(qr.Answers[0].Comments[0]) != "note" {
		t.Fatalf("comment rehydrate lost: %+v", qr.Answers[0].Comments)
	}
}

func TestLoadSessionUnknownIdErrors(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.LoadSession(t.Context(), "ghost"); err == nil {
		t.Fatal("unknown id must error, not return empty")
	}
}
```

Imports: add `"github.com/malikbenkirane/grill-chef/internal/domain/session"`.

- [x] **Step 2: Run test to verify it fails**

Run: `go test ./internal/port/store/sqlite/ -run TestLoadSession -v`

Expected: FAIL — `LoadSession` undefined.

- [x] **Step 3: Implement the reader (complete, no placeholders)**

```go
package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/malikbenkirane/grill-chef/internal/domain/round"
	"github.com/malikbenkirane/grill-chef/internal/domain/session"
)

// LoadSession rehydrates the whole aggregate in one transaction: the
// sessions row, every round payload + question/answer/comment row, and the
// legend row. One indexed read set; the assembly below finishes before the
// commit — no post-load merge.
func (s *Store) LoadSession(ctx context.Context, id string) (*session.Session, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("sqlite: LoadSession begin: %w", err)
	}
	defer tx.Rollback()

	var sess session.Session
	var statusLit, sessUpdated string
	if err := tx.QueryRowContext(ctx,
		`SELECT id, name, status, updated_at FROM sessions WHERE id = ?`, id).
		Scan(&sess.ID, &sess.Name, &statusLit, &sessUpdated); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("sqlite: LoadSession %q: not found", id)
		}
		return nil, fmt.Errorf("sqlite: LoadSession %q: %w", id, err)
	}
	if sess.Status, err = statusFromLiteral(statusLit); err != nil {
		return nil, fmt.Errorf("sqlite: LoadSession %q: %w", id, err)
	}
	sess.UpdatedAt, _ = parseStamp(sessUpdated)

	type qrow struct {
		number, status int
		payload, updated string
	}
	rows, err := tx.QueryContext(ctx,
		`SELECT round_number, status, payload, updated_at
		 FROM rounds WHERE session_id = ? ORDER BY round_number`, id)
	if err != nil {
		return nil, fmt.Errorf("sqlite: rounds read: %w", err)
	}
	var qrows []qrow
	for rows.Next() {
		var r qrow
		if err := rows.Scan(&r.number, &r.status, &r.payload, &r.updated); err != nil {
			rows.Close()
			return nil, fmt.Errorf("sqlite: rounds scan: %w", err)
		}
		qrows = append(qrows, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: rounds iter: %w", err)
	}
	for _, rr := range qrows {
		qr, err := loadQuestionRound(ctx, tx, id, rr.number, rr.payload, rr.updated)
		if err != nil {
			return nil, fmt.Errorf("sqlite: round %d: %w", rr.number, err)
		}
		sess.Rounds = append(sess.Rounds, qr)
	}

	var legPayload, legUpdated string
	err = tx.QueryRowContext(ctx,
		`SELECT payload, updated_at FROM legends WHERE session_id = ?`, id).
		Scan(&legPayload, &legUpdated)
	switch {
	case err == nil:
		lr, err := unmarshalLegendPayload([]byte(legPayload))
		if err != nil {
			return nil, fmt.Errorf("sqlite: legend payload: %w", err)
		}
		lr.UpdatedAt, _ = parseStamp(legUpdated)
		sess.Rounds = append(sess.Rounds, lr)
	case err == sql.ErrNoRows:
	default:
		return nil, fmt.Errorf("sqlite: legend read: %w", err)
	}

	return &sess, tx.Commit()
}

func loadQuestionRound(ctx context.Context, tx *sql.Tx, id string,
	number int, payload, updated string) (round.QuestionRound, error) {

	qr := round.QuestionRound{Number: number, UpdatedAt: mustSt(updated)}
	if err := json.Unmarshal([]byte(payload), &qr); err != nil {
		return round.QuestionRound{}, fmt.Errorf("payload: %w", err)
	}
	// qPayload round-trips via the mapping layer below; the domain fields
	// are populated through the adapter structs, not raw JSON reflection.

	qrows, err := tx.QueryContext(ctx,
		`SELECT question_index, title, abstract, options, recommended_index, reminders
		 FROM questions WHERE session_id = ? AND round_number = ? ORDER BY question_index`,
		id, number)
	if err != nil {
		return round.QuestionRound{}, fmt.Errorf("questions: %w", err)
	}
	var qs []round.Question
	for qrows.Next() {
		var idx, rec int
		var title, abstract, options, reminders string
		if err := qrows.Scan(&idx, &title, &abstract, &options, &rec, &reminders); err != nil {
			qrows.Close()
			return round.QuestionRound{}, fmt.Errorf("questions scan: %w", err)
		}
		q := round.Question{Title: round.Markdown(title), Recommendation: round.Recommendation(rec)}
		for _, a := range mustJSONStrings(abstract) {
			q.Abstract = append(q.Abstract, round.Markdown(a))
		}
		for _, o := range mustJSONStrings(options) {
			q.Choices = append(q.Choices, round.Choice{Text: round.Markdown(o)})
		}
		for _, rem := range mustReminders(reminders) {
			q.Reminders = append(q.Reminders, round.Reminder{Tag: round.Tag(rem.Tag), Summary: round.Markdown(rem.Summary)})
		}
		qs = append(qs, q)
	}
	qrows.Close()
	if err := qrows.Err(); err != nil {
		return round.QuestionRound{}, fmt.Errorf("questions iter: %w", err)
	}
	qr.Questions = qs

	// Answers: one indexed query, rehydrated positionally aligned.
	answers := make([]round.Answer, len(qs))
	for i := range answers {
		answers[i] = round.Answer{ChoiceIndex: round.ChoiceIndexNone}
	}
	aRows, err := tx.QueryContext(ctx,
		`SELECT question_index, chosen_option FROM answers
		 WHERE session_id = ? AND round_number = ? ORDER BY question_index`, id, number)
	if err != nil {
		return round.QuestionRound{}, fmt.Errorf("answers: %w", err)
	}
	for aRows.Next() {
		var qi int
		var chosen sql.NullInt64
		if err := aRows.Scan(&qi, &chosen); err != nil {
			aRows.Close()
			return round.QuestionRound{}, fmt.Errorf("answers scan: %w", err)
		}
		if 1 <= qi && qi <= len(answers) {
			answers[qi-1].ChoiceIndex = round.ChoiceIndex(chosen.Int64)
		}
	}
	aRows.Close()
	if err := aRows.Err(); err != nil {
		return round.QuestionRound{}, fmt.Errorf("answers iter: %w", err)
	}

	// Comments: one indexed query in comment_seq order, attached to the
	// Answer of their question (issue 8's per-comment identity).
	cRows, err := tx.QueryContext(ctx,
		`SELECT question_index, comment_seq, body FROM comments
		 WHERE session_id = ? AND round_number = ? ORDER BY question_index, comment_seq`, id, number)
	if err != nil {
		return round.QuestionRound{}, fmt.Errorf("comments: %w", err)
	}
	for cRows.Next() {
		var qi, cseq int
		var body string
		if err := cRows.Scan(&qi, &cseq, &body); err != nil {
			cRows.Close()
			return round.QuestionRound{}, fmt.Errorf("comments scan: %w", err)
		}
		if 1 <= qi && qi <= len(answers) {
			answers[qi-1].Comments = append(answers[qi-1].Comments, round.Markdown(body))
		}
	}
	cRows.Close()
	if err := cRows.Err(); err != nil {
		return round.QuestionRound{}, fmt.Errorf("comments iter: %w", err)
	}

	qr.Answers = answers
	return qr, nil
}

// statusFromLiteral is the exact inverse of statusLiteral (mapping.go).
func statusFromLiteral(lit string) (session.Status, error) {
	switch lit {
	case "active":
		return session.StatusActive, nil
	case "closed":
		return session.StatusClosed, nil
	case "dismissed":
		return session.StatusDismissed, nil
	default:
		return session.StatusUndefined, fmt.Errorf("unknown session status %q", lit)
	}
}

// parseStamp parses the DATETIME rendering. Try RFC3339 first, then the
// SQLite CURRENT_TIMESTAMP default form. A zero time is a legitimate
// fallback only when the column read is empty — never for present rows.
func parseStamp(raw string) (time.Time, error) {
	if raw == "" {
		return time.Time{}, fmt.Errorf("empty stamp")
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unparseable stamp %q", raw)
}

func mustSt(raw string) time.Time { t, _ := parseStamp(raw); return t }

func mustJSONStrings(raw string) []string {
	var out []string
	_ = json.Unmarshal([]byte(raw), &out)
	return out
}

type reminderDTO struct {
	Tag     string `json:"tag"`
	Summary string `json:"summary"`
}

func mustReminders(raw string) []reminderDTO {
	var out []reminderDTO
	_ = json.Unmarshal([]byte(raw), &out)
	return out
}
```

The QuestionRound payload unmarshals straight into the domain struct? No — `qr := round.QuestionRound` cannot decode directly because `qPayload` (02) uses adapter tags and splits questions off. The correct decode lives in this task's `loadQuestionRound`: replace the naive `json.Unmarshal(&qr)` line above with a round-trip through 02's mapping shapes (the same order the adapter wrote):

```go
	var raw qPayload
	if err := json.Unmarshal([]byte(payload), &raw); err != nil {
		return round.QuestionRound{}, fmt.Errorf("payload: %w", err)
	}
	qr := round.QuestionRound{
		SessionID: raw.SessionID,
		Number:    raw.Number,
		Intro:     round.Markdown(raw.Intro),
		UpdatedAt: mustSt(updated),
	}
	for _, b := range raw.Recap {
		rb := round.Branch{Tag: round.Tag(b.Tag), Title: b.Title, Prose: round.Markdown(b.Prose), Served: b.Served, Retained: b.Retained}
		for _, tag := range b.NewlySettled {
			rb.NewlySettled = append(rb.NewlySettled, round.Tag(tag))
		}
		qr.Recap = append(qr.Recap, rb)
	}
```

The final `readers.go` must carry the second block (the qPayload round-trip) and NOT the naive unmarshal; delete the marked line before your first passing build. `qrow.status` is scanned but unused by the reader (no merge in Go) — drop the field from the struct and the SELECT list instead, keeping the read lean; the round status lives in the `rounds` table for `IsSessionConsistent` (Task 3), never consulted by reassembly.

- [x] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/port/store/sqlite/ -run TestLoadSession -v`

Expected: PASS.

- [x] **Step 5: Commit with caveman-commit**

Use `caveman-commit` for the message content, then the commit sequence from `core-commands` (temp file pattern, `jj describe --stdin`).

Message subject: `feat(store): single-read session rehydration with aligned answers`

---

### Task 3: Gap-based IsSessionConsistent

**Files:**
- Modify: `internal/port/store/sqlite/mapping.go` (payload grows `question_count`)
- Modify: 02's `mapping.go` test in place
- Create: `internal/port/store/sqlite/consistency.go`
- Test: `internal/port/store/sqlite/consistency_test.go`

- [x] **Step 1: Extend the payload contract (the prerequisite)**

`qPayload` in `mapping.go` gains:

```go
	QuestionCount int `json:"question_count"`
```

and `marshalRoundPayload` sets `m.QuestionCount = len(qr.Questions)`. Update 02's `TestQuestionPayloadExcludesQuestions` to assert `m.QuestionCount == 1` after unmarshaling `raw` (a scalar, not question bodies). The question bodies never ride the payload — the count only.

- [x] **Step 2: Write the failing tests**

```go
package sqlite

import "testing"

func TestConsistentFreshSession(t *testing.T) {
	s := newTestStore(t)
	seedSessionRound(t, s, "cs1")
	ok, err := s.IsSessionConsistent(t.Context(), "cs1")
	if err != nil || !ok {
		t.Fatalf("fresh active session: consistent=%v err=%v", ok, err)
	}
}

func TestInconsistentOnMissingRound(t *testing.T) {
	s := newTestStore(t)
	seedSessionRound(t, s, "cs2")
	if err := s.CreateQuestionRound(t.Context(), mkQuestionRound("cs2", 3, 1)); err != nil {
		t.Fatal(err)
	}
	ok, err := s.IsSessionConsistent(t.Context(), "cs2")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("gap in round numbers must fail consistency")
	}
}

func TestInconsistentOnPendingWithoutFullQuestions(t *testing.T) {
	s := newTestStore(t)
	seedSessionRound(t, s, "cs3")
	if _, err := s.db.Exec(`DELETE FROM questions WHERE session_id='cs3' AND question_index=2`); err != nil {
		t.Fatal(err)
	}
	ok, err := s.IsSessionConsistent(t.Context(), "cs3")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("pending round with missing question row must fail consistency")
	}
}

func TestConsistencyUnknownSessionIsError(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.IsSessionConsistent(t.Context(), "ghost"); err == nil {
		t.Fatal("unknown session must be an error, not a false negative")
	}
}
```

- [x] **Step 3: Run tests to verify they fail**

Run: `go test ./internal/port/store/sqlite/ -run 'TestConsistent|TestInconsistent' -v`

Expected: FAIL — `IsSessionConsistent` undefined.

- [x] **Step 4: Implement the check (complete, no broken SQL drafts)**

```go
package sqlite

import (
	"context"
	"encoding/json"
	"fmt"
)

// IsSessionConsistent is the correctness check consumed by grill_recover
// (issue 4): gap-based, never timestamp-based. Round numbers must be
// contiguous 1..N over the session's question rounds, and every pending
// round must carry its full question set (payload question_count).
func (s *Store) IsSessionConsistent(ctx context.Context, id string) (bool, error) {
	var sessExists int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sessions WHERE id = ?`, id).Scan(&sessExists); err != nil {
		return false, fmt.Errorf("sqlite: IsSessionConsistent probe: %w", err)
	}
	if sessExists == 0 {
		return false, fmt.Errorf("sqlite: IsSessionConsistent %q: not found", id)
	}

	var maxRound int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(round_number), 0) FROM rounds WHERE session_id = ?`, id).
		Scan(&maxRound); err != nil {
		return false, fmt.Errorf("sqlite: max round: %w", err)
	}
	if maxRound == 0 {
		return true, nil // no rounds written yet: trivially contiguous 1..0
	}

	// Void probe: the PK makes duplicate (session, round) rows impossible,
	// so DISTINCT count == max is exactly "contiguous 1..N".
	var distinct int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(DISTINCT round_number) FROM rounds
		 WHERE session_id = ? AND round_number <= ?`, id, maxRound).Scan(&distinct); err != nil {
		return false, fmt.Errorf("sqlite: void probe: %w", err)
	}
	if distinct != maxRound {
		return false, nil
	}

	// Every pending round carries its full question set.
	pRows, err := s.db.QueryContext(ctx,
		`SELECT round_number, payload FROM rounds WHERE session_id = ? AND status = 'pending'`, id)
	if err != nil {
		return false, fmt.Errorf("sqlite: pending probe: %w", err)
	}
	defer pRows.Close()
	for pRows.Next() {
		var number int
		var payload string
		if err := pRows.Scan(&number, &payload); err != nil {
			return false, fmt.Errorf("sqlite: pending scan: %w", err)
		}
		var raw qPayload
		if err := json.Unmarshal([]byte(payload), &raw); err != nil {
			return false, fmt.Errorf("sqlite: pending payload: %w", err)
		}
		var qn int
		if err := s.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM questions WHERE session_id = ? AND round_number = ?`,
			id, number).Scan(&qn); err != nil {
			return false, fmt.Errorf("sqlite: pending question count: %w", err)
		}
		if qn != raw.QuestionCount {
			return false, nil
		}
	}
	return true, pRows.Err()
}
```

Imports: this file needs only `context`, `encoding/json`, `fmt`.

- [x] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/port/store/sqlite/ -run 'TestConsistent|TestInconsistent' -v`

Expected: PASS.

- [x] **Step 6: Commit with caveman-commit (same sequence as Task 1)**

Message subject: `feat(store): gap-based session consistency check`

---

### Task 4: Staleness gate — every updated_at populates

**Files:**
- Test: `internal/port/store/sqlite/staleness_test.go`

- [x] **Step 1: Write the test**

```go
package sqlite

import (
	"testing"

	"github.com/malikbenkirane/grill-chef/internal/domain/round"
)

func TestStalenessFieldsPopulate(t *testing.T) {
	s := newTestStore(t)
	seedSessionRound(t, s, "st1")
	loaded, err := s.LoadSession(t.Context(), "st1")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.UpdatedAt.IsZero() {
		t.Fatal("session UpdatedAt must populate")
	}
	qr, ok := loaded.Rounds[0].(round.QuestionRound)
	if !ok {
		t.Fatalf("want QuestionRound, got %T", loaded.Rounds[0])
	}
	if qr.UpdatedAt.IsZero() {
		t.Fatal("round UpdatedAt must populate")
	}
}
```

- [x] **Step 2: Run test to verify it passes**

Run: `go test ./internal/port/store/sqlite/ -run TestStalenessFieldsPopulate -v`

Expected: PASS if Task 2's parse is complete. If it fails, the stamp parse in `readers.go` missed a path (session, round, or legend) — fix there; do not weaken to "staleness optional".

- [x] **Step 3: Commit with caveman-commit (same sequence as Task 1)**

Message subject: `test(store): staleness fields populate through rehydration`

---

### Task 5: daemon.json marker file

**Files:**
- Create: `internal/port/daemoninfo/daemoninfo.go`
- Test: `internal/port/daemoninfo/daemoninfo_test.go`

- [x] **Step 1: Write the failing tests**

```go
package daemoninfo

import "testing"

func TestWriteThenReadMarker(t *testing.T) {
	path := t.TempDir() + "/daemon.json"
	if err := Write(path, Info{PID: 42, Port: 8080}); err != nil {
		t.Fatal(err)
	}
	got, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.PID != 42 || got.Port != 8080 {
		t.Fatalf("lossy marker: %+v", got)
	}
}

func TestReadMissingMarkerIsError(t *testing.T) {
	if _, err := Read(t.TempDir() + "/nope.json"); err == nil {
		t.Fatal("missing marker must be an error: recovery treats no marker as daemon-down")
	}
}

func TestWriteIsAtomicOnSamePath(t *testing.T) {
	path := t.TempDir() + "/daemon.json"
	if err := Write(path, Info{PID: 1, Port: 2}); err != nil {
		t.Fatal(err)
	}
	if err := Write(path, Info{PID: 3, Port: 4}); err != nil {
		t.Fatal(err)
	}
	got, _ := Read(path)
	if got.PID != 3 || got.Port != 4 {
		t.Fatal("second write must replace first atomically (no torn reads)")
	}
}
```

- [x] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/port/daemoninfo/ -v`

Expected: FAIL — package undefined.

- [x] **Step 3: Implement the marker helpers**

```go
package daemoninfo

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Info is the pid + port pair of the HTTP listener (issue 1): state lives
// in a marker file next to the db, rewritten on bind, read by recovery
// tooling and the human `stop` CLI without touching the db or its WAL.
type Info struct {
	PID  int `json:"pid"`
	Port int `json:"port"`
}

// Write is tmp+rename atomic so a reader never sees a torn marker.
func Write(path string, info Info) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("daemoninfo: mkdir: %w", err)
	}
	raw, err := json.Marshal(info)
	if err != nil {
		return fmt.Errorf("daemoninfo: marshal: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return fmt.Errorf("daemoninfo: write tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("daemoninfo: rename: %w", err)
	}
	return nil
}

func Read(path string) (Info, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Info{}, fmt.Errorf("daemoninfo: read %q: %w", path, err)
	}
	var info Info
	if err := json.Unmarshal(raw, &info); err != nil {
		return Info{}, fmt.Errorf("daemoninfo: unmarshal: %w", err)
	}
	return info, nil
}
```

- [x] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/port/daemoninfo/ -v`

Expected: PASS.

- [x] **Step 5: Commit with caveman-commit (same sequence as Task 1)**

Message subject: `feat(store): daemon pid/port marker file helpers`

---

### Task 6: Full-session roundtrip + multi-session concurrency acceptance

**Files:**
- Test: `internal/port/store/sqlite/acceptance_test.go`

- [x] **Step 1: Write the acceptance tests**

```go
package sqlite

import (
	"fmt"
	"sync"
	"testing"

	"github.com/malikbenkirane/grill-chef/internal/domain/round"
	"github.com/malikbenkirane/grill-chef/internal/domain/session"
)

// Full grill lifecycle rehydrated from the id alone (issue 1 acceptance):
// sessions, rounds, answers, comments, legend, close, consistency.
func TestFullSessionRoundtrip(t *testing.T) {
	s := newTestStore(t)
	id := "acc1"
	_ = s.CreateSession(t.Context(), mkSess(id))
	_ = s.CreateQuestionRound(t.Context(), mkQuestionRound(id, 1, 2))
	_ = s.CreateQuestionRound(t.Context(), mkQuestionRound(id, 2, 3))
	_ = s.UpsertAnswer(t.Context(), id, 1, 1, round.ChoiceIndex(1))
	_ = s.UpsertAnswer(t.Context(), id, 1, 2, round.ChoiceIndexOther)
	_, _ = s.AddComment(t.Context(), id, 1, 2, "why not B")
	_ = s.SubmitRound(t.Context(), id, 1)
	_ = s.CreateLegendRound(t.Context(), round.LegendRound{
		SessionID: id,
		Title:     round.Markdown("legend"),
	})
	_ = s.CloseSession(t.Context(), id)

	loaded, err := s.LoadSession(t.Context(), id)
	if err != nil {
		t.Fatalf("rehydrate: %v", err)
	}
	if loaded.Status != session.StatusClosed {
		t.Fatalf("status drift: %v", loaded.Status)
	}
	if len(loaded.Rounds) != 3 {
		t.Fatalf("want 3 rounds (2 question + 1 legend), got %d", len(loaded.Rounds))
	}
	l, ok := loaded.Rounds[2].(round.LegendRound)
	if !ok || string(l.Title) != "legend" {
		t.Fatalf("legend round misplaced: %+v", loaded.Rounds[2])
	}
	consistent, err := s.IsSessionConsistent(t.Context(), id)
	if err != nil {
		t.Fatal(err)
	}
	if !consistent {
		t.Fatal("fully-written session must pass consistency")
	}
}

// Concurrent writes to different sessions must not corrupt (issue 1).
func TestConcurrentMultisessionWritesDoNotCorrupt(t *testing.T) {
	s := newTestStore(t)
	const n = 6
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := fmt.Sprintf("ms%d", i)
			if err := s.CreateSession(t.Context(), mkSess(id)); err != nil {
				t.Errorf("create %s: %v", id, err)
				return
			}
			if err := s.CreateQuestionRound(t.Context(), mkQuestionRound(id, 1, 2)); err != nil {
				t.Errorf("round %s: %v", id, err)
				return
			}
			for q := 1; q <= 2; q++ {
				if err := s.UpsertAnswer(t.Context(), id, 1, q, round.ChoiceIndex(q)); err != nil {
					t.Errorf("answer %s: %v", id, err)
				}
			}
			if err := s.SubmitRound(t.Context(), id, 1); err != nil {
				t.Errorf("submit %s: %v", id, err)
			}
		}(i)
	}
	wg.Wait()

	for i := 0; i < n; i++ {
		id := fmt.Sprintf("ms%d", i)
		consistent, err := s.IsSessionConsistent(t.Context(), id)
		if err != nil || !consistent {
			t.Errorf("session %s consistent=%v err=%v", id, consistent, err)
		}
		loaded, err := s.LoadSession(t.Context(), id)
		if err != nil {
			t.Fatalf("rehydrate %s: %v", id, err)
		}
		if len(loaded.Rounds) != 1 {
			t.Errorf("session %s rounds: %d", id, len(loaded.Rounds))
		}
	}
}
```

- [x] **Step 2: Run tests to verify they pass**

Run: `go test ./internal/port/store/sqlite/ -run 'TestFullSessionRoundtrip|TestConcurrentMultisessionWrites' -race -v -count 3`

Expected: PASS on every count.

- [x] **Step 3: Full suite under race**

Run: `go test ./... -race && go vet ./...`

Expected: `ok` everywhere; vet clean.

- [x] **Step 4: Commit with caveman-commit (same sequence as Task 1)**

Message subject: `test(store): full session roundtrip and multi-session concurrency acceptance`

---

### Task 7: Docs-reconciliation final gate

**Files:**
- Verification only — no new files

- [x] **Step 1: Verify docs alignment**

Re-read `docs/issues/1.md` and `docs/drafts/state.md`; confirm the implementation matches both normative sources on: six tables (incl. `legends`), single indexed read, gap-based `IsSessionConsistent`, staleness populated but never consulted for correctness, marker file outside the db, `StatusDismissed` written through `DismissSession`. List any drift found and raise it in chat **before** declaring the sub-plan done.

- [x] **Step 2: Verify the plan set is fully executed**

Run: `grep -c '\- \[ \]' docs/plans/20261024-185550_state-db/*.md`

Expected: 0 unchecked in every sub-plan file; the 00-index tick for `03-reads-recovery` happens in its own tick-only commit after this plan closes.

- [x] **Step 3: Commit only if anything was touched**

Commit via the `caveman-commit` + `core-commands` sequence. If nothing was touched, skip — do not commit empty changes.
