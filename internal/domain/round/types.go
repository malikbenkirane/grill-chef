// Package round holds the quiz/domain model of a grill session: rounds,
// questions, choices, and answers. It is transport-agnostic: JSON mapping
// lives in the adapter layer (internal/port/round/json).
package round

// Markdown is any text blob the server renders to HTML. Using a distinct
// type makes the structural-vs-markdown boundary compiler-checked.
type Markdown string

// Tag is a short plain identifier for a design-tree branch. Tags are never
// markdown.
type Tag string

// Kind distinguishes the phase a round belongs to.
type Kind int

const (
	// KindUndefined marks a malformed or unset round; never valid in memory.
	KindUndefined Kind = iota
	// KindQuestions is a normal grill round carrying questions.
	KindQuestions
	// KindLegend is the final optional round carrying the closing legend.
	KindLegend
)

func (k Kind) String() string {
	switch k {
	case KindQuestions:
		return "questions"
	case KindLegend:
		return "legend"
	case KindUndefined:
		return "undefined"
	default:
		return "unknown"
	}
}

// SchemaVersion is the version stamped on wire payloads by the DTO adapter.
// The first breaking shape change bumps it and adds a state.db validator
// (see docs/issues/20.md).
const SchemaVersion = 1
