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
		Title:     "legend",
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
	for i := range n {
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

	for i := range n {
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
