package sqlite

import (
	"testing"

	"github.com/malikbenkirane/grill-chef/internal/domain/round"
	"github.com/malikbenkirane/grill-chef/internal/domain/session"
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
