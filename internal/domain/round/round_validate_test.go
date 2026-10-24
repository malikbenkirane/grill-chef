package round

import (
	"errors"
	"testing"
)

func TestQuestionRoundValidate(t *testing.T) {
	valid := QuestionRound{
		SessionID: "s1",
		Number:    1,
		Intro:     "Intro.",
		Questions: []Question{{Choices: []Choice{{Text: "a"}}, Recommendation: 1}},
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}

	noSession := valid
	noSession.SessionID = ""
	if err := noSession.Validate(); !errors.Is(err, ErrEmptySessionID) {
		t.Fatalf("Validate() = %v, want ErrEmptySessionID", err)
	}

	badNumber := valid
	badNumber.Number = 0
	if err := badNumber.Validate(); !errors.Is(err, ErrBadRoundNumber) {
		t.Fatalf("Validate() = %v, want ErrBadRoundNumber", err)
	}

	noQuestions := valid
	noQuestions.Questions = nil
	if err := noQuestions.Validate(); !errors.Is(err, ErrNoQuestions) {
		t.Fatalf("Validate() = %v, want ErrNoQuestions", err)
	}

	badQuestion := valid
	badQuestion.Questions = []Question{{}}
	if err := badQuestion.Validate(); !errors.Is(err, ErrNoChoices) {
		t.Fatalf("Validate() = %v, want ErrNoChoices", err)
	}
}
