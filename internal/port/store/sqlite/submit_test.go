package sqlite

import (
	"testing"

	"github.com/malikbenkirane/grill-chef/internal/domain/session"
)

func TestSubmitRoundFlipsPendingToSubmitted(t *testing.T) {
	s := newTestStore(t)
	seedSessionRound(t, s, "sb1")
	if err := s.SubmitRound(t.Context(), "sb1", 1); err != nil {
		t.Fatal(err)
	}
	var status string
	_ = s.db.QueryRow(`SELECT status FROM rounds WHERE session_id=? AND round_number=?`, "sb1", 1).Scan(&status)
	if status != "submitted" {
		t.Fatalf("want submitted, got %q", status)
	}
}

func TestSubmitRoundIsIdempotentAndGuardsLegend(t *testing.T) {
	s := newTestStore(t)
	seedSessionRound(t, s, "sb2")
	if err := s.SubmitRound(t.Context(), "sb2", 1); err != nil {
		t.Fatal(err)
	}
	if err := s.SubmitRound(t.Context(), "sb2", 1); err != nil {
		t.Fatalf("resubmit must not error (idempotent no-op): %v", err)
	}
	if err := s.SubmitRound(t.Context(), "sb2", 9); err == nil {
		t.Fatal("submit of nonexistent round must fail")
	}
}

func TestDismissSessionFlipsStatusNotDeletes(t *testing.T) {
	s := newTestStore(t)
	seedSessionRound(t, s, "d1")
	if err := s.DismissSession(t.Context(), "d1"); err != nil {
		t.Fatal(err)
	}
	var status string
	if err := s.db.QueryRow(`SELECT status FROM sessions WHERE id=?`, "d1").Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "dismissed" {
		t.Fatalf("want dismissed, got %q", status)
	}
	var n int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM rounds WHERE session_id=?`, "d1").Scan(&n)
	if n != 1 {
		t.Fatal("dismiss must never delete; retention keeps everything")
	}
	var closed session.Status = session.StatusDismissed
	if closed.String() != "dismissed" {
		t.Fatal("domain literal drifted from store edge")
	}
}
