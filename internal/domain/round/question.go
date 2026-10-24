package round

// Reminder re-states a design-tree branch the question leaned on.
type Reminder struct {
	Tag     Tag
	Summary Markdown
}

// Question is the prompt side of one grill question: title, optional
// abstract paragraphs (markdown), 1..N choices, optional recommendation
// and reminders. Answers are stored in AnswerSheet, not here.
type Question struct {
	Title          Markdown
	Abstract       []Markdown
	Choices        []Choice
	Recommendation Recommendation
	Reminders      []Reminder
}

// RecommendedChoice returns the choice the question recommends, if the
// recommendation is set and inside the choice range.
func (q Question) RecommendedChoice() (Choice, bool) {
	if q.Recommendation == RecommendationNone {
		return Choice{}, false
	}
	i := int(q.Recommendation) - 1
	if i < 0 || i >= len(q.Choices) {
		return Choice{}, false
	}
	return q.Choices[i], true
}
