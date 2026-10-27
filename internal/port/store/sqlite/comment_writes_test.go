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
