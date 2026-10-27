package sqlite

import (
	"context"
	"encoding/json"
	"fmt"
)

// IsSessionConsistent is the correctness check consumed by grill_recover
// (issue 4): gap-based, never timestamp-based. Round numbers must be
// contiguous 1..N over the session's question rounds, and every pending
// round must carry its full question set (payload question_count).
func (s *Store) IsSessionConsistent(ctx context.Context, id string) (bool, error) {
	var sessExists int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sessions WHERE id = ?`, id).Scan(&sessExists); err != nil {
		return false, fmt.Errorf("sqlite: IsSessionConsistent probe: %w", err)
	}
	if sessExists == 0 {
		return false, fmt.Errorf("sqlite: IsSessionConsistent %q: not found", id)
	}

	var maxRound int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(round_number), 0) FROM rounds WHERE session_id = ?`, id).
		Scan(&maxRound); err != nil {
		return false, fmt.Errorf("sqlite: max round: %w", err)
	}
	if maxRound == 0 {
		return true, nil // no rounds written yet: trivially contiguous 1..0
	}

	// Void probe: the PK makes duplicate (session, round) rows impossible,
	// so DISTINCT count == max is exactly "contiguous 1..N".
	var distinct int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(DISTINCT round_number) FROM rounds
		 WHERE session_id = ? AND round_number <= ?`, id, maxRound).Scan(&distinct); err != nil {
		return false, fmt.Errorf("sqlite: void probe: %w", err)
	}
	if distinct != maxRound {
		return false, nil
	}

	// Every pending round carries its full question set.
	pRows, err := s.db.QueryContext(ctx,
		`SELECT round_number, payload FROM rounds WHERE session_id = ? AND status = 'pending'`, id)
	if err != nil {
		return false, fmt.Errorf("sqlite: pending probe: %w", err)
	}
	defer pRows.Close()
	for pRows.Next() {
		var number int
		var payload string
		if err := pRows.Scan(&number, &payload); err != nil {
			return false, fmt.Errorf("sqlite: pending scan: %w", err)
		}
		var raw qPayload
		if err := json.Unmarshal([]byte(payload), &raw); err != nil {
			return false, fmt.Errorf("sqlite: pending payload: %w", err)
		}
		var qn int
		if err := s.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM questions WHERE session_id = ? AND round_number = ?`,
			id, number).Scan(&qn); err != nil {
			return false, fmt.Errorf("sqlite: pending question count: %w", err)
		}
		if qn != raw.QuestionCount {
			return false, nil
		}
	}
	return true, pRows.Err()
}
