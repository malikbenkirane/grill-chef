package round

import (
	"testing"
	"time"
)

func TestRoundsCarryUpdatedAt(t *testing.T) {
	now := time.Now()
	q := QuestionRound{}
	if !q.UpdatedAt.IsZero() {
		t.Fatal("fresh QuestionRound must carry a zero UpdatedAt")
	}
	q.UpdatedAt = now
	if !q.UpdatedAt.Equal(now) {
		t.Fatal("QuestionRound.UpdatedAt not assignable")
	}
	l := LegendRound{}
	if !l.UpdatedAt.IsZero() {
		t.Fatal("fresh LegendRound must carry a zero UpdatedAt")
	}
}
