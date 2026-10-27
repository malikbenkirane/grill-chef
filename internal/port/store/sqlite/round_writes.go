package sqlite

import (
	"context"
	"fmt"

	"github.com/malikbenkirane/grill-chef/internal/domain/round"
)

// CreateQuestionRound writes the rounds row (payload = intro+recap only)
// plus one questions row per domain question, in one transaction. MCP-owned.
func (s *Store) CreateQuestionRound(ctx context.Context, qr round.QuestionRound) error {
	raw, err := marshalRoundPayload(qr)
	if err != nil {
		return fmt.Errorf("sqlite: CreateQuestionRound: %w", err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("sqlite: begin: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx,
		`INSERT INTO rounds (session_id, round_number, status, payload)
		 VALUES (?, ?, 'pending', ?)`, qr.SessionID, qr.Number, string(raw))
	if err != nil {
		return fmt.Errorf("sqlite: rounds insert (%s,%d): %w", qr.SessionID, qr.Number, err)
	}
	for i, q := range qr.Questions {
		_, err = tx.ExecContext(ctx,
			`INSERT INTO questions
			 (session_id, round_number, question_index, title, abstract, options, recommended_index, reminders)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			qr.SessionID, qr.Number, i+1,
			questionTitle(q), questionAbstract(q), questionOptions(q),
			questionRecommendedIndex(q), questionReminders(q))
		if err != nil {
			return fmt.Errorf("sqlite: question %d insert: %w", i+1, err)
		}
	}
	return tx.Commit()
}

// CreateLegendRound writes the legends row only — a legend round never
// lives in rounds. MCP-owned. Retried with the same session id fails on PK.
func (s *Store) CreateLegendRound(ctx context.Context, lr round.LegendRound) error {
	raw, err := marshalLegendPayload(lr)
	if err != nil {
		return fmt.Errorf("sqlite: CreateLegendRound: %w", err)
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO legends (session_id, payload) VALUES (?, ?)`,
		lr.SessionID, string(raw))
	if err != nil {
		return fmt.Errorf("sqlite: legends insert %s: %w", lr.SessionID, err)
	}
	return nil
}
