package sqlite

import (
	"context"
	"fmt"
)

// AddComment appends to a question's comment list inside one transaction:
// the next per-question sequence is MAX(comment_seq)+1, so a deleted seq is
// never reused and the DELETE route's identity stays stable. The browser
// round-trips the returned seq in the URL (issue 8).
func (s *Store) AddComment(ctx context.Context, sessionID string, roundNumber, questionIndex int, body string) (int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("sqlite: begin: %w", err)
	}
	defer tx.Rollback()

	var seq int
	if err := tx.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(comment_seq), 0) + 1 FROM comments
		 WHERE session_id=? AND round_number=? AND question_index=?`,
		sessionID, roundNumber, questionIndex).Scan(&seq); err != nil {
		return 0, fmt.Errorf("sqlite: AddComment seq: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO comments (session_id, round_number, question_index, comment_seq, body)
		 VALUES (?, ?, ?, ?, ?)`,
		sessionID, roundNumber, questionIndex, seq, body); err != nil {
		return 0, fmt.Errorf("sqlite: AddComment insert (%s,%d,%d): %w", sessionID, roundNumber, questionIndex, err)
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("sqlite: AddComment commit: %w", err)
	}
	return seq, nil
}

// DeleteComment removes exactly one comment by its per-question identity.
func (s *Store) DeleteComment(ctx context.Context, sessionID string, roundNumber, questionIndex, seq int) error {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM comments
		 WHERE session_id=? AND round_number=? AND question_index=? AND comment_seq=?`,
		sessionID, roundNumber, questionIndex, seq)
	if err != nil {
		return fmt.Errorf("sqlite: DeleteComment (%s,%d,%d,%d): %w", sessionID, roundNumber, questionIndex, seq, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("sqlite: DeleteComment (%s,%d,%d,%d): not found", sessionID, roundNumber, questionIndex, seq)
	}
	return nil
}
