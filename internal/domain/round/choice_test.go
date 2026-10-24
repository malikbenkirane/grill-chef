package round

import "testing"

func TestChoiceIndexConstants(t *testing.T) {
	if ChoiceIndexNone != -1 {
		t.Fatalf("ChoiceIndexNone = %d, want -1", ChoiceIndexNone)
	}
	if ChoiceIndexOther != 0 {
		t.Fatalf("ChoiceIndexOther = %d, want 0", ChoiceIndexOther)
	}
	if RecommendationNone != 0 {
		t.Fatalf("RecommendationNone = %d, want 0", RecommendationNone)
	}
}
