package round

import (
	"errors"
	"testing"
)

func TestQuestionValidate(t *testing.T) {
	ok := Question{Choices: []Choice{{Text: "a"}}, Recommendation: 1}
	if err := ok.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}

	noRec := Question{Choices: []Choice{{Text: "a"}}}
	if err := noRec.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}

	if err := (Question{}).Validate(); !errors.Is(err, ErrNoChoices) {
		t.Fatalf("Validate() = %v, want ErrNoChoices", err)
	}

	bad := Question{Choices: []Choice{{Text: "a"}}, Recommendation: 2}
	if err := bad.Validate(); !errors.Is(err, ErrRecommendationOutOfRange) {
		t.Fatalf("Validate() = %v, want ErrRecommendationOutOfRange", err)
	}
}
