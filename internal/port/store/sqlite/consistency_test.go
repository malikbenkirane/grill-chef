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
