# 02 — Answer and Validation

> **For agentic workers:** REQUIRED SUB-SKILL: Use supervised-plan-execution to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `Answer` and `AnswerSheet`, the `Validate()` methods on every round type, and the `Answer.String()` rendering — the domain invariants the tool layer can rely on.

**Architecture:** Pure domain, no mocks, table-driven tests. Sentinels for every violated invariant so tests assert with `errors.Is`. Invariants implemented here (grill legend): index in range, Other ⇒ ≥1 comment, skip ⇒ `ChoiceIndexNone`, answer sheet aligned 1:1 with the questions of the round. The 5-question/5-option caps are tool policy (issue 5), not domain invariants — the domain enforces structural reality only.

**Tech Stack:** Go stdlib (`errors`, `strings`).

**Requires:** sub-plan 01 completed (types exist). Concurrent-eligible with sub-plan 03 — this sub-plan touches only `internal/domain/round` files `answer.go`, `validate.go` and their tests.

---

### Task 1: Answer type and String rendering

**Files:**
- Create: `internal/domain/round/answer.go`
- Test: `internal/domain/round/answer_test.go`

`Answer.String()` renders what the agent pastes back into its chat history: the skipped answer renders empty, an "Other" answer renders the comments, a listed choice renders its text plus any extra comments joined by blank lines. Choices come from the question, so they are passed in.

- [ ] **Step 1: Write the failing test**

```go
package round

import "testing"

func TestAnswerString(t *testing.T) {
	choices := []Choice{{Text: "Postgres"}, {Text: "SQLite"}}
	cases := []struct {
		name string
		ans  Answer
		want string
	}{
		{
			name: "skipped",
			ans:  Answer{ChoiceIndex: ChoiceIndexNone},
			want: "",
		},
		{
			name: "other with comments",
			ans:  Answer{ChoiceIndex: ChoiceIndexOther, Comments: []Markdown{"I want **BoltDB**", "actually twice"}},
			want: "I want **BoltDB**\n\nactually twice",
		},
		{
			name: "listed choice",
			ans:  Answer{ChoiceIndex: 2},
			want: "SQLite",
		},
		{
			name: "listed choice with comments",
			ans:  Answer{ChoiceIndex: 1, Comments: []Markdown{"vendored", "and WAL only"}},
			want: "Postgres\n\nvendored\n\nand WAL only",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.ans.String(choices); got != tc.want {
				t.Fatalf("String() = %q, want %q", got, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/domain/round/`
Expected: FAIL — undefined: `Answer`

- [ ] **Step 3: Write minimal implementation**

```go
package round

import "strings"

// Answer is one user answer to a question: a 1-based choice index (or a
// sentinel for skip/other) plus optional markdown comments collected by
// the browser UI.
type Answer struct {
	ChoiceIndex ChoiceIndex
	Comments    []Markdown
}

// String renders the answer for the agent's chat history: the chosen
// choice text, then comments, joined by blank lines. Skipped renders as
// the empty string; "Other" renders comments only.
func (a Answer) String(choices []Choice) string {
	if a.ChoiceIndex == ChoiceIndexNone {
		return ""
	}
	var parts []string
	if a.ChoiceIndex != ChoiceIndexOther {
		i := int(a.ChoiceIndex) - 1
		if 0 <= i && i < len(choices) {
			parts = append(parts, string(choices[i].Text))
		}
	}
	for _, c := range a.Comments {
		parts = append(parts, string(c))
	}
	return strings.Join(parts, "\n\n")
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/domain/round/`
Expected: PASS

- [ ] **Step 5: Commit with caveman-commit**

Commit sequence from `core-commands` (mktemp / Read / Write / `jj describe --stdin < <path> && rm <path> && jj new`), message:

```
feat(round): add answer type with rendering

Renders choice text plus comments for chat history; skip renders
empty, other renders comments only.
```

---

### Task 2: Sentinels and question validation

**Files:**
- Create: `internal/domain/round/validate.go`
- Test: `internal/domain/round/validate_test.go`

- [ ] **Step 1: Write the failing test**

```go
package round

import (
	"errors"
	"testing"
)

func TestQuestionValidate(t *testing.T) {
	ok := Question{Choices: []Choice{{Text: "a"}}, Recommendation: 1}
	if err := ok.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}

	noRec := Question{Choices: []Choice{{Text: "a"}}}
	if err := noRec.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}

	if err := (Question{}).Validate(); !errors.Is(err, ErrNoChoices) {
		t.Fatalf("Validate() = %v, want ErrNoChoices", err)
	}

	bad := Question{Choices: []Choice{{Text: "a"}}, Recommendation: 2}
	if err := bad.Validate(); !errors.Is(err, ErrRecommendationOutOfRange) {
		t.Fatalf("Validate() = %v, want ErrRecommendationOutOfRange", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/domain/round/`
Expected: FAIL — undefined: `ErrNoChoices`, `ErrRecommendationOutOfRange`

- [ ] **Step 3: Write minimal implementation**

```go
package round

import "errors"

// Invariants violated by malformed rounds or answers.
var (
	ErrEmptySessionID           = errors.New("round: empty session id")
	ErrBadRoundNumber           = errors.New("round: number must be >= 1")
	ErrNoQuestions               = errors.New("round: at least one question required")
	ErrNoChoices                 = errors.New("round: at least one choice required")
	ErrRecommendationOutOfRange  = errors.New("round: recommendation out of range")
	ErrBadAnswerChoice           = errors.New("round: answer choice out of range")
	ErrOtherNeedsComment        = errors.New("round: other requires at least one comment")
	ErrAnswerCountMismatch      = errors.New("round: answers do not match questions")
	ErrSheetMismatch             = errors.New("round: answer sheet session/number mismatch")
	ErrEmptyLegend              = errors.New("round: legend requires at least one branch")
)
```

- [ ] **Step 4: Run test to verify it fails**

Run: `go test ./internal/domain/round/`
Expected: FAIL — `Validate()` not defined

- [ ] **Step 5: Add the question validator (append to `validate.go`)**

```go
// Validate checks that the question's choices and recommendation are
// structural reality: 1..N choices and a recommendation either absent
// (0) or inside the range.
func (q Question) Validate() error {
	if len(q.Choices) < 1 {
		return ErrNoChoices
	}
	if q.Recommendation != RecommendationNone &&
		(q.Recommendation < 1 || int(q.Recommendation) > len(q.Choices)) {
		return ErrRecommendationOutOfRange
	}
	return nil
}
```

- [ ] **Step 6: Run test to verify it passes**

Run: `go test ./internal/domain/round/`
Expected: PASS

- [ ] **Step 7: Commit with caveman-commit**

Commit sequence from `core-commands`, message:

```
feat(round): add round invariants and question validate

Structural invariants in the domain; product caps stay tool policy.
```

---

### Task 3: QuestionRound.Validate

**Files:**
- Modify: `internal/domain/round/validate.go` (append)
- Test: `internal/domain/round/round_validate_test.go`

- [ ] **Step 1: Write the failing test**

```go
package round

import (
	"errors"
	"testing"
)

func TestQuestionRoundValidate(t *testing.T) {
	valid := QuestionRound{
		SessionID: "s1",
		Number:    1,
		Intro:     "Intro.",
		Questions: []Question{{Choices: []Choice{{Text: "a"}}, Recommendation: 1}},
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}

	noSession := valid
	noSession.SessionID = ""
	if err := noSession.Validate(); !errors.Is(err, ErrEmptySessionID) {
		t.Fatalf("Validate() = %v, want ErrEmptySessionID", err)
	}

	badNumber := valid
	badNumber.Number = 0
	if err := badNumber.Validate(); !errors.Is(err, ErrBadRoundNumber) {
		t.Fatalf("Validate() = %v, want ErrBadRoundNumber", err)
	}

	noQuestions := valid
	noQuestions.Questions = nil
	if err := noQuestions.Validate(); !errors.Is(err, ErrNoQuestions) {
		t.Fatalf("Validate() = %v, want ErrNoQuestions", err)
	}

	badQuestion := valid
	badQuestion.Questions = []Question{{}}
	if err := badQuestion.Validate(); !errors.Is(err, ErrNoChoices) {
		t.Fatalf("Validate() = %v, want ErrNoChoices", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/domain/round/`
Expected: FAIL — no method `Validate` on `QuestionRound`

- [ ] **Step 3: Write minimal implementation (append to `validate.go`)**

```go
// Validate checks the round's own invariants and then every question.
// First error wins; per-question errors are wrapped with the 1-based
// question position for debugging agent payloads.
func (r QuestionRound) Validate() error {
	if r.SessionID == "" {
		return ErrEmptySessionID
	}
	if r.Number < 1 {
		return ErrBadRoundNumber
	}
	if len(r.Questions) < 1 {
		return ErrNoQuestions
	}
	for i, q := range r.Questions {
		if err := q.Validate(); err != nil {
			return fmt.Errorf("question %d: %w", i+1, err)
		}
	}
	return nil
}
```

and add `"fmt"` to the import block:

```go
import (
	"errors"
	"fmt"
)
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/domain/round/`
Expected: PASS

- [ ] **Step 5: Commit with caveman-commit**

Commit sequence from `core-commands`, message:

```
feat(round): validate question rounds

Checks session id, round number and every question with a
positioned error context for debugging agent payloads.
```

---

### Task 4: LegendRound and AnswerSheet validation

**Files:**
- Modify: `internal/domain/round/validate.go` (append)
- Test: `internal/domain/round/round_validate_test.go` (append)

- [ ] **Step 1: Write the failing tests**

```go
package round

import (
	"errors"
	"testing"
)

func TestLegendRoundValidate(t *testing.T) {
	valid := LegendRound{SessionID: "s1", Title: "Grill", Branches: []LegendBranch{{Title: "B", Bullets: []LegendBullet{{Tag: "a", Decision: "done"}}}}}
	if err := valid.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}

	noSession := valid
	noSession.SessionID = ""
	if err := noSession.Validate(); !errors.Is(err, ErrEmptySessionID) {
		t.Fatalf("Validate() = %v, want ErrEmptySessionID", err)
	}

	empty := LegendRound{SessionID: "s1", Title: "Grill"}
	if err := empty.Validate(); !errors.Is(err, ErrEmptyLegend) {
		t.Fatalf("Validate() = %v, want ErrEmptyLegend", err)
	}
}

func TestAnswerSheetValidate(t *testing.T) {
	submitted := QuestionRound{
		SessionID: "s1",
		Number:    1,
		Questions: []Question{
			{Choices: []Choice{{Text: "a"}, {Text: "b"}}},
			{Choices: []Choice{{Text: "c"}}},
		},
	}
	sheet := AnswerSheet{SessionID: "s1", Number: 1, Answers: []Answer{{ChoiceIndex: 2}, {ChoiceIndex: ChoiceIndexNone}}}
	if err := sheet.Validate(submitted); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}

	otherNoComment := AnswerSheet{SessionID: "s1", Number: 1, Answers: []Answer{{ChoiceIndex: ChoiceIndexOther}}}
	if err := otherNoComment.Validate(submitted); !errors.Is(err, ErrOtherNeedsComment) {
		t.Fatalf("Validate() = %v, want ErrOtherNeedsComment", err)
	}

	mismatch := AnswerSheet{SessionID: "s1", Number: 1, Answers: []Answer{{ChoiceIndex: 1}}}
	if err := mismatch.Validate(submitted); !errors.Is(err, ErrAnswerCountMismatch) {
		t.Fatalf("Validate() = %v, want ErrAnswerCountMismatch", err)
	}

	outOfRange := AnswerSheet{SessionID: "s1", Number: 1, Answers: []Answer{{ChoiceIndex: 2}}}
	if err := outOfRange.Validate(submitted); !errors.Is(err, ErrBadAnswerChoice) {
		t.Fatalf("Validate() = %v, want ErrBadAnswerChoice (question 2 has one choice)", err)
	}

	badSession := sheet
	badSession.SessionID = "s2"
	if err := badSession.Validate(submitted); !errors.Is(err, ErrSheetMismatch) {
		t.Fatalf("Validate() = %v, want ErrSheetMismatch (session mismatch)", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/domain/round/`
Expected: FAIL — undefined: `AnswerSheet`

- [ ] **Step 3: Write minimal implementation (append to `answer.go`)**

```go
// AnswerSheet is the ordered result of one submitted question round: one
// answer per question, aligned by position.
type AnswerSheet struct {
	SessionID string
	Number    int
	Answers   []Answer
}

// Validate checks the sheet against the submitted round: session and
// number must agree, one answer per question, and every answer must be
// structural reality for ITS OWN question's choice list (skip; other
// with >= 1 comment; or an index inside that question's choices).
func (s AnswerSheet) Validate(submitted QuestionRound) error {
	if s.SessionID != submitted.SessionID || s.Number != submitted.Number {
		return ErrSheetMismatch
	}
	if len(s.Answers) != len(submitted.Questions) {
		return ErrAnswerCountMismatch
	}
	for i, a := range s.Answers {
		n := len(submitted.Questions[i].Choices)
		switch {
		case a.ChoiceIndex == ChoiceIndexNone:
		case a.ChoiceIndex == ChoiceIndexOther:
			if len(a.Comments) == 0 {
				return fmt.Errorf("question %d: %w", i+1, ErrOtherNeedsComment)
			}
		case a.ChoiceIndex >= 1 && int(a.ChoiceIndex) <= n:
		default:
			return fmt.Errorf("question %d: %w", i+1, ErrBadAnswerChoice)
		}
	}
	return nil
}
```

- [ ] **Step 4: Append LegendRound.Validate (to `validate.go`)**

```go
// Validate checks the legend round's own invariants: a session id and at
// least one branch.
func (r LegendRound) Validate() error {
	if r.SessionID == "" {
		return ErrEmptySessionID
	}
	if len(r.Branches) < 1 {
		return ErrEmptyLegend
	}
	return nil
}
```

Note: `fmt` may already be imported in `validate.go` from Task 3; if using it in `answer.go` too, add `"fmt"` to `answer.go`'s import block.

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/domain/round/`
Expected: PASS

- [ ] **Step 6: Run the round package's full suite once more**

Run: `go test ./internal/domain/round/ -count=1`
Expected: PASS

- [ ] **Step 7: Commit with caveman-commit**

Commit sequence from `core-commands`, message:

```
feat(round): add answer sheet and round validation

Answers align 1:1 with questions; skip/other/choice rules
validated in the domain with sentinel errors.
```

---

### Task 5: Sub-plan boundary check

- [ ] **Step 1: Run the whole module's tests and go vet**

Run: `go test ./... && go vet ./...`
Expected: `ok  github.com/malikbenkirane/grill-chef/internal/domain/round` and clean vet output
