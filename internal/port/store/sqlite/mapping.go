package sqlite

import (
	"encoding/json"
	"fmt"

	"github.com/malikbenkirane/grill-chef/internal/domain/round"
	"github.com/malikbenkirane/grill-chef/internal/domain/session"
)

// qPayload is the `rounds.payload` column: the question round minus its
// questions — those live in `questions` rows and are re-assembled on load,
// which is why the payload must carry them as nil (sub-plan 03 reads this
// via unmarshal and re-merges from rows; no post-load merge in Go applies
// to question bodies).
type qPayload struct {
	SchemaVersion int         `json:"schema_version"`
	SessionID     string      `json:"session_id"`
	Number        int         `json:"number"`
	Intro         string      `json:"intro"`
	Recap         []branchDTO `json:"recap"`
}

type branchDTO struct {
	Tag          string   `json:"tag"`
	Title        string   `json:"title"`
	Prose        string   `json:"prose"`
	NewlySettled []string `json:"newly_settled"`
	Served       int      `json:"served"`
	Retained     int      `json:"retained"`
}

func marshalRoundPayload(qr round.QuestionRound) ([]byte, error) {
	if qr.SessionID == "" || qr.Number == 0 {
		return nil, fmt.Errorf("sqlite: question round missing identity: session %q number %d", qr.SessionID, qr.Number)
	}
	m := qPayload{
		SchemaVersion: round.SchemaVersion,
		SessionID:     qr.SessionID,
		Number:        qr.Number,
		Intro:         string(qr.Intro),
	}
	for _, b := range qr.Recap {
		tags := make([]string, len(b.NewlySettled))
		for i, tag := range b.NewlySettled {
			tags[i] = string(tag)
		}
		m.Recap = append(m.Recap, branchDTO{
			Tag: string(b.Tag), Title: b.Title, Prose: string(b.Prose),
			NewlySettled: tags, Served: b.Served, Retained: b.Retained,
		})
	}
	return json.Marshal(m)
}

// legendPayload is the full `legends.payload` column.
type legendPayload struct {
	SchemaVersion int            `json:"schema_version"`
	SessionID     string         `json:"session_id"`
	Title         string         `json:"title"`
	Branches      []legBranchDTO `json:"branches"`
}

type legBranchDTO struct {
	Title   string         `json:"title"`
	Bullets []legBulletDTO `json:"bullets"`
}

type legBulletDTO struct {
	Tag      string `json:"tag"`
	Decision string `json:"decision"`
}

func marshalLegendPayload(lr round.LegendRound) ([]byte, error) {
	if lr.SessionID == "" {
		return nil, fmt.Errorf("sqlite: legend round missing session id")
	}
	m := legendPayload{
		SchemaVersion: round.SchemaVersion,
		SessionID:     lr.SessionID,
		Title:         lr.Title,
	}
	for _, b := range lr.Branches {
		d := legBranchDTO{Title: b.Title}
		for _, bul := range b.Bullets {
			d.Bullets = append(d.Bullets, legBulletDTO{Tag: string(bul.Tag), Decision: string(bul.Decision)})
		}
		m.Branches = append(m.Branches, d)
	}
	return json.Marshal(m)
}

func unmarshalLegendPayload(raw []byte) (round.LegendRound, error) {
	var m legendPayload
	if err := json.Unmarshal(raw, &m); err != nil {
		return round.LegendRound{}, fmt.Errorf("sqlite: legend payload: %w", err)
	}
	lr := round.LegendRound{SessionID: m.SessionID, Title: m.Title}
	for _, b := range m.Branches {
		lb := round.LegendBranch{Title: b.Title}
		for _, bul := range b.Bullets {
			lb.Bullets = append(lb.Bullets, round.LegendBullet{Tag: round.Tag(bul.Tag), Decision: round.Markdown(bul.Decision)})
		}
		lr.Branches = append(lr.Branches, lb)
	}
	return lr, nil
}

// questionColumns map one domain question to its column shapes.

func questionTitle(q round.Question) string { return string(q.Title) }

func questionAbstract(q round.Question) string {
	paras := make([]string, len(q.Abstract))
	for i, a := range q.Abstract {
		paras[i] = string(a)
	}
	raw, _ := json.Marshal(paras)
	return string(raw)
}

func questionOptions(q round.Question) string {
	opts := make([]string, len(q.Choices))
	for i, c := range q.Choices {
		opts[i] = string(c.Text)
	}
	raw, _ := json.Marshal(opts)
	return string(raw)
}

// questionRecommendedIndex is the domain Recommendation verbatim:
// RecommendationNone (0) = "no recommendation", 1..N = the 1-based choice.
// The column default 0 matches the domain sentinel exactly.
func questionRecommendedIndex(q round.Question) int { return int(q.Recommendation) }

func questionReminders(q round.Question) string {
	if len(q.Reminders) == 0 {
		return "[]"
	}
	type rem struct {
		Tag     string `json:"tag"`
		Summary string `json:"summary"`
	}
	rs := make([]rem, len(q.Reminders))
	for i, r := range q.Reminders {
		rs[i] = rem{Tag: string(r.Tag), Summary: string(r.Summary)}
	}
	raw, _ := json.Marshal(rs)
	return string(raw)
}

// statusLiteral renders the store edge value of a session status. The
// literals match the sessions.status CHECK exactly.
func statusLiteral(s session.Status) (string, error) {
	switch s {
	case session.StatusActive, session.StatusClosed, session.StatusDismissed:
		return s.String(), nil
	default:
		return "", fmt.Errorf("sqlite: undefined session status %d", s)
	}
}
