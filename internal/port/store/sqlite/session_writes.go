package sqlite

import (
	"context"
	"fmt"

	"github.com/malikbenkirane/grill-chef/internal/domain/session"
)

// CreateSession persists the domain aggregate verbatim. MCP-owned.
func (s *Store) CreateSession(ctx context.Context, in *session.Session) error {
	if in == nil {
		return fmt.Errorf("sqlite: CreateSession: nil session")
	}
	lit, err := statusLiteral(in.Status)
	if err != nil {
		return fmt.Errorf("sqlite: CreateSession: %w", err)
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO sessions (id, name, status) VALUES (?, ?, ?)`,
		in.ID, in.Name, lit)
	if err != nil {
		return fmt.Errorf("sqlite: CreateSession %q: %w", in.ID, err)
	}
	return nil
}

// CloseSession flips an existing active session to closed. MCP-owned; ties
// to the legend round being processed, not round counting (decision trail).
func (s *Store) CloseSession(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE sessions SET status = 'closed', updated_at = CURRENT_TIMESTAMP
		 WHERE id = ? AND status = 'active'`, id)
	if err != nil {
		return fmt.Errorf("sqlite: CloseSession %q: %w", id, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("sqlite: CloseSession %q: not active or unknown session", id)
	}
	return nil
}
