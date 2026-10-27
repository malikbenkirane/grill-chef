package round

import "testing"

func TestQuestionRoundCarriesAlignedAnswers(t *testing.T) {
	qr := QuestionRound{SessionID: "s", Number: 1}
	if len(qr.Answers) != 0 {
		t.Fatal("fresh round must have no answers")
	}
	qr.Answers = []Answer{{ChoiceIndex: ChoiceIndexOther, Comments: []Markdown{"note"}}}
	if qr.Answers[0].ChoiceIndex != ChoiceIndexOther {
		t.Fatal("answers field not assignable")
	}
}
