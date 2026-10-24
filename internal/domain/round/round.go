package round

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
}

func (LegendRound) Kind() Kind { return KindLegend }
