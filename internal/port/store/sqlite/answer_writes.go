package sqlite

import (
	"context"
	"fmt"

	"github.com/malikbenkirane/grill-chef/internal/domain/round"
)

// UpsertAnswer is the daemon's autosave hot path: one question choice per
// call, insert on first save and update after, chosen_option storing the
// domain sentinel verbatim (-1 none, 0 other, 1..N listed).
func (s *Store) UpsertAnswer(ctx context.Context, sessionID string, roundNumber, questionIndex int, choice round.ChoiceIndex) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO answers (session_id, round_number, question_index, chosen_option)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT (session_id, round_number, question_index)
		 DO UPDATE SET chosen_option = excluded.chosen_option,
		               updated_at = CURRENT_TIMESTAMP`,
		sessionID, roundNumber, questionIndex, int(choice))
	if err != nil {
		return fmt.Errorf("sqlite: UpsertAnswer (%s,%d,%d): %w", sessionID, roundNumber, questionIndex, err)
	}
	return nil
}

// SubmitRound is the daemon's submit path for a pending round. Idempotent:
// submitting an already-submitted round is a no-op (the round page may
// double-fire) but unknown rounds still fail. MCP payload columns untouched.
func (s *Store) SubmitRound(ctx context.Context, sessionID string, roundNumber int) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE rounds SET status = 'submitted', updated_at = CURRENT_TIMESTAMP
		 WHERE session_id = ? AND round_number = ? AND status = 'pending'`,
		sessionID, roundNumber)
	if err != nil {
		return fmt.Errorf("sqlite: SubmitRound (%s,%d): %w", sessionID, roundNumber, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		var exists int
		if err := s.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM rounds WHERE session_id = ? AND round_number = ?`,
			sessionID, roundNumber).Scan(&exists); err != nil {
			return fmt.Errorf("sqlite: SubmitRound probe: %w", err)
		}
		if exists == 0 {
			return fmt.Errorf("sqlite: SubmitRound (%s,%d): round not found", sessionID, roundNumber)
		}
	}
	return nil
}

// DismissSession is the station home's status change (issue 14), written by
// the web daemon through the port. Data preservation: never a delete.
func (s *Store) DismissSession(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE sessions SET status = 'dismissed', updated_at = CURRENT_TIMESTAMP
		 WHERE id = ? AND status != 'dismissed'`, id)
	if err != nil {
		return fmt.Errorf("sqlite: DismissSession %q: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("sqlite: DismissSession %q: already dismissed or unknown session", id)
	}
	return nil
}
