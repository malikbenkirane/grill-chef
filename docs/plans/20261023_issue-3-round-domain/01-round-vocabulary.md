# 01 — Round Vocabulary Types

> **For agentic workers:** REQUIRED SUB-SKILL: Use supervised-plan-execution to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Bootstrap the Go module and create the base vocabulary types of `internal/domain/round` — the pure domain types every later issue builds on.

**Architecture:** DDD clean architecture. The domain package never imports `encoding/json` or any transport concern. Sum-typed rounds (`QuestionRound` / `LegendRound` behind a `Round` interface) make illegal phase/field states unrepresentable. Module path: `github.com/malikbenkirane/grill-chef`.

**Tech Stack:** Go stdlib only (`fmt`, `strings` in the domain; no external deps).

Settled decisions implemented here (from the grill legend): options are 1-based, 0 is the shared sentinel for "other"/"no recommendation", `type Markdown string` for every renderable blob, strong `Kind`, separate `QuestionRound` and `LegendRound` types, `SessionID` carried on rounds so payloads are self-describing.

---

### Task 1: Module init + Markdown, Tag, Kind, SchemaVersion

**Files:**
- Create: `go.mod`
- Create: `internal/domain/round/types.go`
- Test: `internal/domain/round/types_test.go`

- [ ] **Step 1: Run test to verify it fails (no module yet)**

Run: `go test ./internal/domain/round/`
Expected: FAIL — no `go.mod` / package not found

- [ ] **Step 2: Initialize the module**

Run: `go mod init github.com/malikbenkirane/grill-chef`
Expected: output `go: creating new go.mod: module github.com/malikbenkirane/grill-chef`

- [ ] **Step 3: Write the failing test**

```go
package round

import "testing"

func TestKindString(t *testing.T) {
	cases := []struct {
		name string
		kind Kind
		want string
	}{
		{"undefined", KindUndefined, "undefined"},
		{"questions", KindQuestions, "questions"},
		{"legend", KindLegend, "legend"},
		{"unknown", Kind(99), "unknown"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.kind.String(); got != tc.want {
				t.Fatalf("Kind(%d).String() = %q, want %q", tc.kind, got, tc.want)
			}
		})
	}
}

func TestSchemaVersionIsOne(t *testing.T) {
	if SchemaVersion != 1 {
		t.Fatalf("SchemaVersion = %d, want 1", SchemaVersion)
	}
}
```

- [ ] **Step 4: Run test to verify it fails**

Run: `go test ./internal/domain/round/`
Expected: FAIL — undefined: `Kind`, `SchemaVersion`

- [ ] **Step 5: Write minimal implementation**

```go
// Package round holds the quiz/domain model of a grill session: rounds,
// questions, choices, and answers. It is transport-agnostic: JSON mapping
// lives in the adapter layer (internal/port/round/json).
package round

// Markdown is any text blob the server renders to HTML. Using a distinct
// type makes the structural-vs-markdown boundary compiler-checked.
type Markdown string

// Tag is a short plain identifier for a design-tree branch. Tags are never
// markdown.
type Tag string

// Kind distinguishes the phase a round belongs to.
type Kind int

const (
	// KindUndefined marks a malformed or unset round; never valid in memory.
	KindUndefined Kind = iota
	// KindQuestions is a normal grill round carrying questions.
	KindQuestions
	// KindLegend is the final optional round carrying the closing legend.
	KindLegend
)

func (k Kind) String() string {
	switch k {
	case KindQuestions:
		return "questions"
	case KindLegend:
		return "legend"
	case KindUndefined:
		return "undefined"
	default:
		return "unknown"
	}
}

// SchemaVersion is the version stamped on wire payloads by the DTO adapter.
// The first breaking shape change bumps it and adds a state.db validator
// (see docs/issues/19.md).
const SchemaVersion = 1
```

- [ ] **Step 6: Run test to verify it passes**

Run: `go test ./internal/domain/round/`
Expected: PASS

- [ ] **Step 7: Commit with caveman-commit**

Use `caveman-commit` for the message content, then the commit sequence from `core-commands`:

1. `mktemp`
2. Read the temp file (required even though empty)
3. Write the commit message to the temp file (`docs: ignore` format below)
4. `jj describe --stdin < "/tmp/tmp.XXXXXX" && rm "/tmp/tmp.XXXXXX" && jj new`

Commit message:

```
feat(round): add markdown/kind vocabulary types

Starts the DDD domain layer for issue 3; JSON stays in the adapter
layer to keep the domain pure.
```

---

### Task 2: Choice, ChoiceIndex, Recommendation

**Files:**
- Create: `internal/domain/round/choice.go`
- Test: `internal/domain/round/choice_test.go`

Numbering contract (settled): choices are addressed by a **1-based** index; `0` is the shared sentinel (`ChoiceIndexOther`, `RecommendationNone`); skip is `-1` (`ChoiceIndexNone`, introduced with `Answer` in sub-plan 02 but the constant is defined here so the number line is complete).

- [ ] **Step 1: Write the failing test**

```go
package round

import "testing"

func TestChoiceIndexConstants(t *testing.T) {
	if ChoiceIndexNone != -1 {
		t.Fatalf("ChoiceIndexNone = %d, want -1", ChoiceIndexNone)
	}
	if ChoiceIndexOther != 0 {
		t.Fatalf("ChoiceIndexOther = %d, want 0", ChoiceIndexOther)
	}
	if RecommendationNone != 0 {
		t.Fatalf("RecommendationNone = %d, want 0", RecommendationNone)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/domain/round/`
Expected: FAIL — undefined: `ChoiceIndexNone`, `ChoiceIndexOther`, `RecommendationNone`

- [ ] **Step 3: Write minimal implementation**

```go
package round

// Choice is one selectable option of a question. Text may carry inline
// markdown (bold, links), never structural markers such as counts or a
// recommended marker.
type Choice struct {
	Text Markdown
}

// ChoiceIndex addresses a choice of a question by 1-based position.
type ChoiceIndex int

const (
	// ChoiceIndexNone means the user skipped the question.
	ChoiceIndexNone ChoiceIndex = -1
	// ChoiceIndexOther means the user picked no listed choice and typed
	// free text instead; at least one comment is required.
	ChoiceIndexOther ChoiceIndex = 0
	// Listed choices are indexes 1..N (1-based).
)

// Recommendation is an optional hint pointing at a 1-based choice index.
type Recommendation int

const (
	// RecommendationNone means the question carries no recommendation.
	RecommendationNone Recommendation = 0
	// A recommendation of 1..N points at the listed choice.
)
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/domain/round/`
Expected: PASS

- [ ] **Step 5: Commit with caveman-commit**

Commit message (temp-file sequence as in Task 1):

```
feat(round): add choice index and recommendation sentinels

One numbering domain: choices/recommendations are 1-based, 0 is the
other/none sentinel, -1 means skipped.
```

---

### Task 3: Question and reminders

**Files:**
- Create: `internal/domain/round/question.go`
- Test: `internal/domain/round/question_test.go`

A `Question` is prompt-only data authored by the agent. Answers live separately in `AnswerSheet` (sub-plan 02) so an unanswered question's zero value is never mistaken for a chosen "Other". `Index` is deliberately omitted — the position in the round's `Questions` slice is authoritative.

- [ ] **Step 1: Write the failing test**

```go
package round

import "testing"

func TestRecommendedChoice(t *testing.T) {
	choices := []Choice{{Text: "**A**"}, {Text: "B"}}
	bold := Question{Title: "Which one?", Choices: choices, Recommendation: 1}

	if c, ok := bold.RecommendedChoice(); !ok || c.Text != "**A**" {
		t.Fatalf("RecommendedChoice() = (%+v, %v), want first choice, true", c, ok)
	}

	none := Question{Title: "Which one?", Choices: choices, Recommendation: RecommendationNone}
	if _, ok := none.RecommendedChoice(); ok {
		t.Fatal("RecommendedChoice() = true for RecommendationNone, want false")
	}

	outOfRange := Question{Choices: choices, Recommendation: 3}
	if _, ok := outOfRange.RecommendedChoice(); ok {
		t.Fatal("RecommendedChoice() = true for out-of-range recommendation, want false")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/domain/round/`
Expected: FAIL — undefined: `Question`

- [ ] **Step 3: Write minimal implementation**

```go
package round

// Reminder re-states a design-tree branch the question leaned on.
type Reminder struct {
	Tag     Tag
	Summary Markdown
}

// Question is the prompt side of one grill question: title, optional
// abstract paragraphs (markdown), 1..N choices, optional recommendation
// and reminders. Answers are stored in AnswerSheet, not here.
type Question struct {
	Title          Markdown
	Abstract       []Markdown
	Choices        []Choice
	Recommendation Recommendation
	Reminders      []Reminder
}

// RecommendedChoice returns the choice the question recommends, if the
// recommendation is set and inside the choice range.
func (q Question) RecommendedChoice() (Choice, bool) {
	if q.Recommendation == RecommendationNone {
		return Choice{}, false
	}
	i := int(q.Recommendation) - 1
	if i < 0 || i >= len(q.Choices) {
		return Choice{}, false
	}
	return q.Choices[i], true
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/domain/round/`
Expected: PASS

- [ ] **Step 5: Commit with caveman-commit**

Commit message:

```
feat(round): add question prompt type

Prompt-only: answers live in the answer sheet so an unanswered
question has no fake zero answer.
```

---

### Task 4: Round sum types (QuestionRound, LegendRound)

**Files:**
- Create: `internal/domain/round/round.go`
- Test: `internal/domain/round/round_test.go`

Settled in the grill: strong `Kind` plus two concrete round types joined by the `Round` interface, so a legend round structurally cannot carry questions and vice versa. `frontier_remaining` becomes structured counts (`Served`, `Retained`) — remaining gets recomputed by the agent, prose would have to be parsed back. `SessionID` rides on each round so payloads are self-describing.

- [ ] **Step 1: Write the failing test**

```go
package round

import "testing"

func TestRoundKinds(t *testing.T) {
	qr := QuestionRound{SessionID: "s1", Number: 1, Intro: "Intro.", Questions: []Question{}}
	if qr.Kind() != KindQuestions {
		t.Fatalf("QuestionRound.Kind() = %v, want KindQuestions", qr.Kind())
	}

	lr := LegendRound{SessionID: "s1", Title: "Grill session"}
	if lr.Kind() != KindLegend {
		t.Fatalf("LegendRound.Kind() = %v, want KindLegend", lr.Kind())
	}

	var r Round = qr
	if got := r.Kind().String(); got != "questions" {
		t.Fatalf("interface Kind().String() = %q, want %q", got, "questions")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/domain/round/`
Expected: FAIL — undefined: `QuestionRound`, `LegendRound`, `Round`

- [ ] **Step 3: Write minimal implementation**

```go
package round

// Round is one grill round sent by the agent. The two concrete types are
// gated by their kind; no single struct holds both phases' fields, so an
// illegal phase/field combination is unrepresentable.
type Round interface {
	Kind() Kind
}

// Branch is one recap branch of a question round: what was settled and
// where the frontier stands, in counts the agent recomputed.
type Branch struct {
	Tag           Tag
	Title         string
	Prose         Markdown
	NewlySettled  []Tag
	Served        int // questions served below in this round
	Retained      int // frontier questions kept for later rounds
}

// QuestionRound sends the current round's questions. Earlier rounds live
// in state; screens render from state, never from recap prose alone.
type QuestionRound struct {
	SessionID string
	Number    int
	Intro     Markdown
	Recap     []Branch
	Questions []Question
}

func (QuestionRound) Kind() Kind { return KindQuestions }

// LegendBullet is one tagged decision of the closing legend.
type LegendBullet struct {
	Tag      Tag
	Decision Markdown
}

// LegendBranch groups the legend bullets of one design-tree branch.
type LegendBranch struct {
	Title   string
	Bullets []LegendBullet
}

// LegendRound is the optional final round closing the session.
type LegendRound struct {
	SessionID string
	Title     string
	Branches  []LegendBranch
}

func (LegendRound) Kind() Kind { return KindLegend }
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/domain/round/`
Expected: PASS

- [ ] **Step 5: Commit with caveman-commit**

Commit message:

```
feat(round): sum-type question and legend rounds

Two concrete round types behind a Round interface make illegal
phase/field mixtures unrepresentable; frontier stays numeric.
```

---

### Task 5: Sub-plan boundary check

- [ ] **Step 1: Run the whole sub-plan's tests and go vet**

Run: `go test ./... && go vet ./...`
Expected: `ok  github.com/malikbenkirane/grill-chef/internal/domain/round` and clean vet output

No commit step: Task 4's commit is the last of this sub-plan; `jj new` already opened the next change.
