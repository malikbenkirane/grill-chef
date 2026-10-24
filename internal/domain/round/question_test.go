package round

import "testing"

func TestRecommendedChoice(t *testing.T) {
	choices := []Choice{{Text: "**A**"}, {Text: "B"}}
	bold := Question{Title: "Which one?", Choices: choices, Recommendation: 1}

	if c, ok := bold.RecommendedChoice(); !ok || c.Text != "**A**" {
		t.Fatalf("RecommendedChoice() = (%+v, %v), want first choice, true", c, ok)
	}

	none := Question{Title: "Which one?", Choices: choices, Recommendation: RecommendationNone}
	if _, ok := none.RecommendedChoice(); ok {
		t.Fatal("RecommendedChoice() = true for RecommendationNone, want false")
	}

	outOfRange := Question{Choices: choices, Recommendation: 3}
	if _, ok := outOfRange.RecommendedChoice(); ok {
		t.Fatal("RecommendedChoice() = true for out-of-range recommendation, want false")
	}
}
