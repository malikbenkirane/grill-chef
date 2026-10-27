package round

import "time"

// Round is one grill round sent by the agent. The two concrete types are
// gated by their kind; no single struct holds both phases' fields, so an
// illegal phase/field combination is unrepresentable.
type Round interface {
	Kind() Kind
}

// Branch is one recap branch of a question round: what was settled and
// where the frontier stands, in counts the agent recomputed.
type Branch struct {
	Tag          Tag
	Title        string
	Prose        Markdown
	NewlySettled []Tag
	Served       int // questions served below in this round
	Retained     int // frontier questions kept for later rounds
}

// QuestionRound sends the current round's questions. Earlier rounds live
// in state; screens render from state, never from recap prose alone.
type QuestionRound struct {
	SessionID string
	Number    int
	Intro     Markdown
	Recap     []Branch
	Questions []Question

	// UpdatedAt is the store's staleness commodity (issue 1): it rides the
	// aggregate after rehydration, but is never consulted for session
	// consistency correctness. Duck-typed by sessions with pending rounds
	// that sat idle for days.
	UpdatedAt time.Time

	// Answers holds one rehydrated result per question, positionally
	// aligned 1..len(Questions) Answer values mapped by question_index
	// from the store; a nil/absent slot means the question was never
	// autosaved. AnswerSheet is the write-side sibling for submit
	// semantics; this field is the rehydrate-side home on the round.
	Answers []Answer
}

func (QuestionRound) Kind() Kind { return KindQuestions }

// LegendBullet is one tagged decision of the closing legend.
type LegendBullet struct {
	Tag      Tag
	Decision Markdown
}

// LegendBranch groups the legend bullets of one design-tree branch.
type LegendBranch struct {
	Title   string
	Bullets []LegendBullet
}

// LegendRound is the optional final round closing the session.
type LegendRound struct {
	SessionID string
	Title     string
	Branches  []LegendBranch

	// UpdatedAt is the store's staleness commodity (issue 1): it rides the
	// aggregate after rehydration, but is never consulted for session
	// consistency correctness. Duck-typed by sessions with pending rounds
	// that sat idle for days.
	UpdatedAt time.Time
}

func (LegendRound) Kind() Kind { return KindLegend }
