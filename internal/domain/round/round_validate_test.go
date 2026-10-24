package round

import (
	"errors"
	"testing"
)

func TestLegendRoundValidate(t *testing.T) {
	valid := LegendRound{SessionID: "s1", Title: "Grill", Branches: []LegendBranch{{Title: "B", Bullets: []LegendBullet{{Tag: "a", Decision: "done"}}}}}
	if err := valid.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}

	noSession := valid
	noSession.SessionID = ""
	if err := noSession.Validate(); !errors.Is(err, ErrEmptySessionID) {
		t.Fatalf("Validate() = %v, want ErrEmptySessionID", err)
	}

	empty := LegendRound{SessionID: "s1", Title: "Grill"}
	if err := empty.Validate(); !errors.Is(err, ErrEmptyLegend) {
		t.Fatalf("Validate() = %v, want ErrEmptyLegend", err)
	}
}

func TestAnswerSheetValidate(t *testing.T) {
	submitted := QuestionRound{
		SessionID: "s1",
		Number:    1,
		Questions: []Question{
			{Choices: []Choice{{Text: "a"}, {Text: "b"}}},
			{Choices: []Choice{{Text: "c"}}},
		},
	}
	sheet := AnswerSheet{SessionID: "s1", Number: 1, Answers: []Answer{{ChoiceIndex: 2}, {ChoiceIndex: ChoiceIndexNone}}}
	if err := sheet.Validate(submitted); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}

	otherNoComment := AnswerSheet{SessionID: "s1", Number: 1, Answers: []Answer{{ChoiceIndex: ChoiceIndexNone}, {ChoiceIndex: ChoiceIndexOther}}}
	if err := otherNoComment.Validate(submitted); !errors.Is(err, ErrOtherNeedsComment) {
		t.Fatalf("Validate() = %v, want ErrOtherNeedsComment", err)
	}

	mismatch := AnswerSheet{SessionID: "s1", Number: 1, Answers: []Answer{{ChoiceIndex: 1}}}
	if err := mismatch.Validate(submitted); !errors.Is(err, ErrAnswerCountMismatch) {
		t.Fatalf("Validate() = %v, want ErrAnswerCountMismatch", err)
	}

	outOfRange := AnswerSheet{SessionID: "s1", Number: 1, Answers: []Answer{{ChoiceIndex: ChoiceIndexNone}, {ChoiceIndex: 2}}}
	if err := outOfRange.Validate(submitted); !errors.Is(err, ErrBadAnswerChoice) {
		t.Fatalf("Validate() = %v, want ErrBadAnswerChoice (question 2 has one choice)", err)
	}

	badSession := sheet
	badSession.SessionID = "s2"
	if err := badSession.Validate(submitted); !errors.Is(err, ErrSheetMismatch) {
		t.Fatalf("Validate() = %v, want ErrSheetMismatch (session mismatch)", err)
	}
}
