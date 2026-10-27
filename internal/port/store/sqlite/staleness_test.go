package sqlite

import (
	"testing"

	"github.com/malikbenkirane/grill-chef/internal/domain/round"
)

func TestStalenessFieldsPopulate(t *testing.T) {
	s := newTestStore(t)
	seedSessionRound(t, s, "st1")
	loaded, err := s.LoadSession(t.Context(), "st1")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.UpdatedAt.IsZero() {
		t.Fatal("session UpdatedAt must populate")
	}
	qr, ok := loaded.Rounds[0].(round.QuestionRound)
	if !ok {
		t.Fatalf("want QuestionRound, got %T", loaded.Rounds[0])
	}
	if qr.UpdatedAt.IsZero() {
		t.Fatal("round UpdatedAt must populate")
	}
}
