package store

import (
	"context"

	"github.com/malikbenkirane/grill-chef/internal/domain/round"
	"github.com/malikbenkirane/grill-chef/internal/domain/session"
)

// Store is the domain's persistence port. Every method belongs to exactly one
// writer process (docs/issues/1.md ownership matrix):
//
//	MCP-owned:      CreateSession, CloseSession, CreateQuestionRound, CreateLegendRound
//	daemon-owned:   UpsertAnswer, SubmitRound, AddComment, DeleteComment, DismissSession
//	shared readers: ListSessions, LoadSession, IsSessionConsistent
//
// The port speaks only in domain aggregates; the SQLite adapter owns local
// types and converts at the edge. Persistence stores domain conventions
// verbatim (1-based choice indexes, sentinels) with no translation.
type Store interface {
	// MCP-owned: sessions and rounds payload agents (stdio tool calls)
	CreateSession(ctx context.Context, s *session.Session) error
	CloseSession(ctx context.Context, id string) error
	CreateQuestionRound(ctx context.Context, r round.QuestionRound) error
	CreateLegendRound(ctx context.Context, r round.LegendRound) error

	// daemon-owned: browser autosave / submit / comment / station-home dismiss
	UpsertAnswer(ctx context.Context, sessionID string, roundNumber, questionIndex int, choice round.ChoiceIndex) error
	SubmitRound(ctx context.Context, sessionID string, roundNumber int) error
	AddComment(ctx context.Context, sessionID string, roundNumber, questionIndex int, body string) (seq int, err error)
	DeleteComment(ctx context.Context, sessionID string, roundNumber, questionIndex, seq int) error
	DismissSession(ctx context.Context, id string) error

	// shared readers
	ListSessions(ctx context.Context) ([]session.Session, error)
	LoadSession(ctx context.Context, id string) (*session.Session, error)
	IsSessionConsistent(ctx context.Context, id string) (bool, error)
}
