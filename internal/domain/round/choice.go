package round

// Choice is one selectable option of a question. Text may carry inline
// markdown (bold, links), never structural markers such as counts or a
// recommended marker.
type Choice struct {
	Text Markdown
}

// ChoiceIndex addresses a choice of a question by 1-based position.
type ChoiceIndex int

const (
	// ChoiceIndexNone means the user skipped the question.
	ChoiceIndexNone ChoiceIndex = -1
	// ChoiceIndexOther means the user picked no listed choice and typed
	// free text instead; at least one comment is required.
	ChoiceIndexOther ChoiceIndex = 0
	// Listed choices are indexes 1..N (1-based).
)

// Recommendation is an optional hint pointing at a 1-based choice index.
type Recommendation int

const (
	// RecommendationNone means the question carries no recommendation.
	RecommendationNone Recommendation = 0
	// A recommendation of 1..N points at the listed choice.
)
