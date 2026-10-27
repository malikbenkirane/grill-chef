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
