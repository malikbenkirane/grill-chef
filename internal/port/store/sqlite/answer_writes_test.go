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
