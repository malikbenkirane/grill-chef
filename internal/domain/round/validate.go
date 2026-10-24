package round

import "errors"

// Invariants violated by malformed rounds or answers.
var (
	ErrEmptySessionID          = errors.New("round: empty session id")
	ErrBadRoundNumber          = errors.New("round: number must be >= 1")
	ErrNoQuestions             = errors.New("round: at least one question required")
	ErrNoChoices               = errors.New("round: at least one choice required")
	ErrRecommendationOutOfRange = errors.New("round: recommendation out of range")
	ErrBadAnswerChoice         = errors.New("round: answer choice out of range")
	ErrOtherNeedsComment       = errors.New("round: other requires at least one comment")
	ErrAnswerCountMismatch     = errors.New("round: answers do not match questions")
	ErrSheetMismatch           = errors.New("round: answer sheet session/number mismatch")
	ErrEmptyLegend             = errors.New("round: legend requires at least one branch")
)

// Validate checks that the question's choices and recommendation are
// structural reality: 1..N choices and a recommendation either absent
// (0) or inside the range.
func (q Question) Validate() error {
	if len(q.Choices) < 1 {
		return ErrNoChoices
	}
	if q.Recommendation != RecommendationNone &&
		(q.Recommendation < 1 || int(q.Recommendation) > len(q.Choices)) {
		return ErrRecommendationOutOfRange
	}
	return nil
}
