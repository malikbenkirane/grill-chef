package sqlite

import (
	"testing"

	"github.com/malikbenkirane/grill-chef/internal/domain/round"
)

func TestQuestionPayloadExcludesQuestions(t *testing.T) {
	qr := round.QuestionRound{
		SessionID: "s1",
		Number:    2,
		Intro:     round.Markdown("intro text"),
		Recap: []round.Branch{
			{Tag: round.Tag("br"), Title: "Branch", Prose: round.Markdown("prose")},
		},
		Questions: []round.Question{
			{Title: round.Markdown("Q")},
		},
	}
	raw, err := marshalRoundPayload(qr)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) == 0 {
		t.Fatal("empty payload")
	}
}

func TestLegendPayloadCarriesWholeRound(t *testing.T) {
	lr := round.LegendRound{
		SessionID: "s1",
		Title:     "legend",
		Branches: []round.LegendBranch{
			{Title: "br", Bullets: []round.LegendBullet{{Tag: round.Tag("t"), Decision: round.Markdown("d")}}},
		},
	}
	raw, err := marshalLegendPayload(lr)
	if err != nil {
		t.Fatal(err)
	}
	back, err := unmarshalLegendPayload(raw)
	if err != nil {
		t.Fatal(err)
	}
	if back.SessionID != lr.SessionID || len(back.Branches) != 1 {
		t.Fatalf("legend payload lossy: %+v", back)
	}
}

func TestQuestionRoundRowsLines(t *testing.T) {
	q := round.Question{
		Title:          round.Markdown("Q"),
		Abstract:       []round.Markdown{"abs"},
		Choices:        []round.Choice{{Text: "opt"}},
		Recommendation: 1,
		Reminders:      []round.Reminder{{Tag: "t", Summary: "s"}},
	}
	if questionRecommendedIndex(q) != 1 {
		t.Fatal("recommendation must persist as domain holds it: 0=none, 1..N")
	}
	var noRec round.Question
	if questionRecommendedIndex(noRec) != 0 {
		t.Fatal("RecommendationNone must persist as 0")
	}
	if got := questionTitle(q); got != "Q" {
		t.Fatalf("title lossy: %q", got)
	}
}
