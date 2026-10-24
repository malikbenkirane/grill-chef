package round

import "testing"

func TestRoundKinds(t *testing.T) {
	qr := QuestionRound{SessionID: "s1", Number: 1, Intro: "Intro.", Questions: []Question{}}
	if qr.Kind() != KindQuestions {
		t.Fatalf("QuestionRound.Kind() = %v, want KindQuestions", qr.Kind())
	}

	lr := LegendRound{SessionID: "s1", Title: "Grill session"}
	if lr.Kind() != KindLegend {
		t.Fatalf("LegendRound.Kind() = %v, want KindLegend", lr.Kind())
	}

	var r Round = qr
	if got := r.Kind().String(); got != "questions" {
		t.Fatalf("interface Kind().String() = %q, want %q", got, "questions")
	}
}
