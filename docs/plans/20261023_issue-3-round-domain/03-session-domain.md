> **For agentic workers:** REQUIRED SUB-SKILL: Use supervised-plan-execution to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

# 03 — Session Domain

**Goal:** Add `internal/domain/session` — the pure grill-describing session: id, name, rounds, two-state status with a `Close()` transition.

**Architecture:** The session describes the grill, never the process hosting it — pid/port stay in issue 1's adapter (one stateless daemon hosts all sessions; the db is the real stateful host). Status is `Active | Closed`, closed exactly when the legend round's answers are stored; no observable "closing" state exists (grill-me's shared-understanding confirmation stays in chat).

**Tech Stack:** Go stdlib only.

**Requires:** sub-plan 01 (uses `round.Round` for the history). Concurrent-eligible with sub-plan 02 — touches only `internal/domain/session`.

Related settled facts to honor: session status values map verbatim to issue 1's `active | closed` columns and issue 4's `active` status at creation; `grill_recover` (issue 6) treats a closed session as "return legend or closed message".

---

### Task 1: Status and Session types

**Files:**
- Create: `internal/domain/session/session.go`
- Test: `internal/domain/session/session_test.go`

- [ ] **Step 1: Write the failing test**

```go
package session

import (
	"errors"
	"testing"
)

func TestStatusString(t *testing.T) {
	cases := []struct {
		name string
		s    Status
		want string
	}{
		{"undefined", StatusUndefined, "undefined"},
		{"active", StatusActive, "active"},
		{"closed", StatusClosed, "closed"},
		{"unknown", Status(9), "unknown"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.s.String(); got != tc.want {
				t.Fatalf("Status(%d).String() = %q, want %q", tc.s, got, tc.want)
			}
		})
	}
}

func TestClose(t *testing.T) {
	s := New("s1", "auth refactor")
	if s.Status != StatusActive {
		t.Fatalf("New().Status = %v, want StatusActive", s.Status)
	}
	if s.ID != "s1" || s.Name != "auth refactor" {
		t.Fatalf("New() = %+v, want id s1, name auth refactor", s)
	}

	if err := s.Close(); err != nil {
		t.Fatalf("Close() = %v, want nil", err)
	}
	if s.Status != StatusClosed {
		t.Fatalf("Status = %v, want StatusClosed", s.Status)
	}

	if err := s.Close(); !errors.Is(err, ErrAlreadyClosed) {
		t.Fatalf("second Close() = %v, want ErrAlreadyClosed", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/domain/session/`
Expected: FAIL — no package yet

- [ ] **Step 3: Write minimal implementation**

```go
// Package session is the pure domain of a grill session: the grill
// itself, not the process hosting it. Persistence and daemon mechanics
// live in the storage adapter (issue 1).
package session

import (
	"errors"

	"github.com/malikbenkirane/grill-chef/internal/domain/round"
)

// ErrAlreadyClosed is returned when closing a closed session.
var ErrAlreadyClosed = errors.New("session: already closed")

// Status of a session. Two states only: there is no observable
// "closing" phase in the grill-me flow — the shared-understanding
// confirmation happens in chat, outside the server.
type Status int

const (
	// StatusUndefined marks a malformed or unset status.
	StatusUndefined Status = iota
	// StatusActive means the grill is open (maps to "active" in state).
	StatusActive
	// StatusClosed means the legend round was submitted (maps to "closed").
	StatusClosed
)

func (s Status) String() string {
	switch s {
	case StatusActive:
		return "active"
	case StatusClosed:
		return "closed"
	case StatusUndefined:
		return "undefined"
	default:
		return "unknown"
	}
}

// Session describes the grill: an id, a human-readable name
// (rendered as tab/heading), the ordered round history, and status.
// The daemon is stateless; the db is the real host of this state.
type Session struct {
	ID     string
	Name   string
	Rounds []round.Round
	Status Status
}

// New opens an active session with the given id and name.
func New(id, name string) *Session {
	return &Session{ID: id, Name: name, Status: StatusActive}
}

// Close marks the session closed. Called by the tool layer when the
// legend round's answers are stored; closing twice is an error so a
// double legend submission cannot corrupt the archive rules.
func (s *Session) Close() error {
	if s.Status == StatusClosed {
		return ErrAlreadyClosed
	}
	s.Status = StatusClosed
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/domain/session/`
Expected: PASS

- [ ] **Step 5: Commit with caveman-commit**

Commit sequence from `core-commands` (mktemp / Read / Write / `jj describe --stdin < <path> && rm <path> && jj new`), message:

```
feat(session): add pure grill session domain

Session describes the grill only: active/closed with a guarded
Close transition; daemon and persistence stay in adapters.
```

---

### Task 2: Sub-plan boundary check

- [ ] **Step 1: Run the whole module's tests and go vet**

Run: `go test ./... && go vet ./...`
Expected: `ok  github.com/malikbenkirane/grill-chef/internal/domain/round` and `ok  github.com/malikbenkirane/grill-chef/internal/domain/session`, clean vet output
