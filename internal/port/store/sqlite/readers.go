package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/malikbenkirane/grill-chef/internal/domain/round"
	"github.com/malikbenkirane/grill-chef/internal/domain/session"
)

// LoadSession rehydrates the whole aggregate in one transaction: the
// sessions row, every round payload + question/answer/comment row, and the
// legend row. One indexed read set; the assembly below finishes before the
// commit — no post-load merge.
func (s *Store) LoadSession(ctx context.Context, id string) (*session.Session, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("sqlite: LoadSession begin: %w", err)
	}
	defer tx.Rollback()

	var sess session.Session
	var statusLit, sessUpdated string
	if err := tx.QueryRowContext(ctx,
		`SELECT id, name, status, updated_at FROM sessions WHERE id = ?`, id).
		Scan(&sess.ID, &sess.Name, &statusLit, &sessUpdated); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("sqlite: LoadSession %q: not found", id)
		}
		return nil, fmt.Errorf("sqlite: LoadSession %q: %w", id, err)
	}
	if sess.Status, err = statusFromLiteral(statusLit); err != nil {
		return nil, fmt.Errorf("sqlite: LoadSession %q: %w", id, err)
	}
	sess.UpdatedAt, _ = parseStamp(sessUpdated)

	type rrow struct {
		number  int
		payload string
		updated string
	}
	rows, err := tx.QueryContext(ctx,
		`SELECT round_number, payload, updated_at
		 FROM rounds WHERE session_id = ? ORDER BY round_number`, id)
	if err != nil {
		return nil, fmt.Errorf("sqlite: rounds read: %w", err)
	}
	var rrows []rrow
	for rows.Next() {
		var r rrow
		if err := rows.Scan(&r.number, &r.payload, &r.updated); err != nil {
			rows.Close()
			return nil, fmt.Errorf("sqlite: rounds scan: %w", err)
		}
		rrows = append(rrows, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: rounds iter: %w", err)
	}
	for _, rr := range rrows {
		qr, err := loadQuestionRound(ctx, tx, id, rr.number, rr.payload, rr.updated)
		if err != nil {
			return nil, fmt.Errorf("sqlite: round %d: %w", rr.number, err)
		}
		sess.Rounds = append(sess.Rounds, qr)
	}

	var legPayload, legUpdated string
	err = tx.QueryRowContext(ctx,
		`SELECT payload, updated_at FROM legends WHERE session_id = ?`, id).
		Scan(&legPayload, &legUpdated)
	switch {
	case err == nil:
		lr, err := unmarshalLegendPayload([]byte(legPayload))
		if err != nil {
			return nil, fmt.Errorf("sqlite: legend payload: %w", err)
		}
		lr.UpdatedAt, _ = parseStamp(legUpdated)
		sess.Rounds = append(sess.Rounds, lr)
	case err == sql.ErrNoRows:
	default:
		return nil, fmt.Errorf("sqlite: legend read: %w", err)
	}

	return &sess, tx.Commit()
}

func loadQuestionRound(ctx context.Context, tx *sql.Tx, id string,
	number int, payload, updated string) (round.QuestionRound, error) {

	var raw qPayload
	if err := json.Unmarshal([]byte(payload), &raw); err != nil {
		return round.QuestionRound{}, fmt.Errorf("payload: %w", err)
	}
	qr := round.QuestionRound{
		SessionID: raw.SessionID,
		Number:    raw.Number,
		Intro:     round.Markdown(raw.Intro),
		UpdatedAt: mustStamp(updated),
	}
	for _, b := range raw.Recap {
		rb := round.Branch{Tag: round.Tag(b.Tag), Title: b.Title, Prose: round.Markdown(b.Prose), Served: b.Served, Retained: b.Retained}
		for _, tag := range b.NewlySettled {
			rb.NewlySettled = append(rb.NewlySettled, round.Tag(tag))
		}
		qr.Recap = append(qr.Recap, rb)
	}

	qRows, err := tx.QueryContext(ctx,
		`SELECT question_index, title, abstract, options, recommended_index, reminders
		 FROM questions WHERE session_id = ? AND round_number = ? ORDER BY question_index`,
		id, number)
	if err != nil {
		return round.QuestionRound{}, fmt.Errorf("questions: %w", err)
	}
	var qs []round.Question
	for qRows.Next() {
		var idx, rec int
		var title, abstract, options, reminders string
		if err := qRows.Scan(&idx, &title, &abstract, &options, &rec, &reminders); err != nil {
			qRows.Close()
			return round.QuestionRound{}, fmt.Errorf("questions scan: %w", err)
		}
		q := round.Question{Title: round.Markdown(title), Recommendation: round.Recommendation(rec)}
		for _, a := range jsonStrings(abstract) {
			q.Abstract = append(q.Abstract, round.Markdown(a))
		}
		for _, o := range jsonStrings(options) {
			q.Choices = append(q.Choices, round.Choice{Text: round.Markdown(o)})
		}
		for _, rem := range jsonReminders(reminders) {
			q.Reminders = append(q.Reminders, round.Reminder{Tag: round.Tag(rem.Tag), Summary: round.Markdown(rem.Summary)})
		}
		qs = append(qs, q)
	}
	qRows.Close()
	if err := qRows.Err(); err != nil {
		return round.QuestionRound{}, fmt.Errorf("questions iter: %w", err)
	}
	qr.Questions = qs

	// Answers: one indexed query, rehydrated positionally aligned.
	answers := make([]round.Answer, len(qs))
	for i := range answers {
		answers[i] = round.Answer{ChoiceIndex: round.ChoiceIndexNone}
	}
	aRows, err := tx.QueryContext(ctx,
		`SELECT question_index, chosen_option FROM answers
		 WHERE session_id = ? AND round_number = ? ORDER BY question_index`, id, number)
	if err != nil {
		return round.QuestionRound{}, fmt.Errorf("answers: %w", err)
	}
	for aRows.Next() {
		var qi int
		var chosen sql.NullInt64
		if err := aRows.Scan(&qi, &chosen); err != nil {
			aRows.Close()
			return round.QuestionRound{}, fmt.Errorf("answers scan: %w", err)
		}
		if 1 <= qi && qi <= len(answers) {
			answers[qi-1].ChoiceIndex = round.ChoiceIndex(chosen.Int64)
		}
	}
	aRows.Close()
	if err := aRows.Err(); err != nil {
		return round.QuestionRound{}, fmt.Errorf("answers iter: %w", err)
	}

	// Comments: one indexed query in comment_seq order, attached to the
	// Answer of their question (issue 8's per-comment identity).
	cRows, err := tx.QueryContext(ctx,
		`SELECT question_index, comment_seq, body FROM comments
		 WHERE session_id = ? AND round_number = ? ORDER BY question_index, comment_seq`, id, number)
	if err != nil {
		return round.QuestionRound{}, fmt.Errorf("comments: %w", err)
	}
	for cRows.Next() {
		var qi, cseq int
		var body string
		if err := cRows.Scan(&qi, &cseq, &body); err != nil {
			cRows.Close()
			return round.QuestionRound{}, fmt.Errorf("comments scan: %w", err)
		}
		if 1 <= qi && qi <= len(answers) {
			answers[qi-1].Comments = append(answers[qi-1].Comments, round.Markdown(body))
		}
	}
	cRows.Close()
	if err := cRows.Err(); err != nil {
		return round.QuestionRound{}, fmt.Errorf("comments iter: %w", err)
	}

	qr.Answers = answers
	return qr, nil
}

// statusFromLiteral is the exact inverse of statusLiteral (mapping.go).
func statusFromLiteral(lit string) (session.Status, error) {
	switch lit {
	case "active":
		return session.StatusActive, nil
	case "closed":
		return session.StatusClosed, nil
	case "dismissed":
		return session.StatusDismissed, nil
	default:
		return session.StatusUndefined, fmt.Errorf("unknown session status %q", lit)
	}
}

// parseStamp parses the DATETIME rendering. Try RFC3339 first, then the
// SQLite CURRENT_TIMESTAMP default form. A zero time is a legitimate
// fallback only when the column read is empty — never for present rows.
func parseStamp(raw string) (time.Time, error) {
	if raw == "" {
		return time.Time{}, fmt.Errorf("empty stamp")
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unparseable stamp %q", raw)
}

func mustStamp(raw string) time.Time { t, _ := parseStamp(raw); return t }

func jsonStrings(raw string) []string {
	var out []string
	_ = json.Unmarshal([]byte(raw), &out)
	return out
}

type reminderDTO struct {
	Tag     string `json:"tag"`
	Summary string `json:"summary"`
}

func jsonReminders(raw string) []reminderDTO {
	var out []reminderDTO
	_ = json.Unmarshal([]byte(raw), &out)
	return out
}
