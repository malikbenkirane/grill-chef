package round

import "strings"

// Answer is one user answer to a question: a 1-based choice index (or a
// sentinel for skip/other) plus optional markdown comments collected by
// the browser UI.
type Answer struct {
	ChoiceIndex ChoiceIndex
	Comments    []Markdown
}

// String renders the answer for the agent's chat history: the chosen
// choice text, then comments, joined by blank lines. Skipped renders as
// the empty string; "Other" renders comments only.
func (a Answer) String(choices []Choice) string {
	if a.ChoiceIndex == ChoiceIndexNone {
		return ""
	}
	var parts []string
	if a.ChoiceIndex != ChoiceIndexOther {
		i := int(a.ChoiceIndex) - 1
		if 0 <= i && i < len(choices) {
			parts = append(parts, string(choices[i].Text))
		}
	}
	for _, c := range a.Comments {
		parts = append(parts, string(c))
	}
	return strings.Join(parts, "\n\n")
}
