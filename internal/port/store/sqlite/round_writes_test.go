package sqlite

import (
	"strings"
	"testing"

	"github.com/malikbenkirane/grill-chef/internal/domain/round"
	"github.com/malikbenkirane/grill-chef/internal/domain/session"
)

func TestCreateQuestionRoundRows(t *testing.T) {
	s := newTestStore(t)
	if err := s.CreateSession(t.Context(), mkSess("q1")); err != nil {
		t.Fatal(err)
	}
	qr := mkQuestionRound("q1", 1, 2)
	if err := s.CreateQuestionRound(t.Context(), qr); err != nil {
		t.Fatalf("CreateQuestionRound: %v", err)
	}
	var payload string
	if err := s.db.QueryRow(`SELECT payload FROM rounds WHERE session_id=? AND round_number=?`, "q1", 1).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(payload, `"title"`) {
		t.Fatal("payload must not embed question bodies; questions live in rows")
	}
	var qcount int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM questions WHERE session_id=? AND round_number=?`, "q1", 1).Scan(&qcount); err != nil {
		t.Fatal(err)
	}
	if qcount != 2 {
		t.Fatalf("want 2 question rows, got %d", qcount)
	}
	var status string
	_ = s.db.QueryRow(`SELECT status FROM rounds WHERE session_id=? AND round_number=?`, "q1", 1).Scan(&status)
	if status != "pending" {
		t.Fatalf("pivot round starts pending, got %q", status)
	}
}

func TestCreateQuestionRoundDuplicateNumberFails(t *testing.T) {
	s := newTestStore(t)
	_ = s.CreateSession(t.Context(), mkSess("q2"))
	qr := mkQuestionRound("q2", 1, 1)
	if err := s.CreateQuestionRound(t.Context(), qr); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateQuestionRound(t.Context(), qr); err == nil {
		t.Fatal("duplicate (session, round) must fail on PK")
	}
}

func TestCreateLegendRoundGoesToLegends(t *testing.T) {
	s := newTestStore(t)
	_ = s.CreateSession(t.Context(), mkSess("l1"))
	lr := round.LegendRound{
		SessionID: "l1",
		Title:     "legend",
		Branches: []round.LegendBranch{{
			Title:   "br",
			Bullets: []round.LegendBullet{{Tag: round.Tag("t"), Decision: round.Markdown("settled")}},
		}},
	}
	if err := s.CreateLegendRound(t.Context(), lr); err != nil {
		t.Fatalf("CreateLegendRound: %v", err)
	}
	var n int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM rounds WHERE session_id=?`, "l1").Scan(&n)
	if n != 0 {
		t.Fatal("legend must never write rounds rows")
	}
	var payload []byte
	if err := s.db.QueryRow(`SELECT payload FROM legends WHERE session_id=?`, "l1").Scan(&payload); err != nil {
		t.Fatalf("legends row missing: %v", err)
	}
	if len(payload) == 0 {
		t.Fatal("empty legend payload")
	}
	// the agent may still grill questions after a legend attempt race; a
	// second legend write (PK session_id) must fail, not silently replace.
	if err := s.CreateLegendRound(t.Context(), lr); err == nil {
		t.Fatal("duplicate legend insert must fail")
	}
}

func mkSess(id string) *session.Session { return session.New(id, "name") }

func mkQuestionRound(id string, number int, questions int) round.QuestionRound {
	qr := round.QuestionRound{SessionID: id, Number: number, Intro: round.Markdown("intro")}
	for i := 0; i < questions; i++ {
		qr.Questions = append(qr.Questions, round.Question{
			Title:   round.Markdown("q"),
			Choices: []round.Choice{{Text: "a"}},
		})
	}
	return qr
}
