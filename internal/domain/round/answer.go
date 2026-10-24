package round

import (
	"fmt"
	"strings"
)

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
