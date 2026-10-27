// Package session is the pure domain of a grill session: the grill
// itself, not the process hosting it. Persistence and daemon mechanics
// live in the storage adapter (issue 1).
package session

import (
	"errors"
	"time"

	"github.com/malikbenkirane/grill-chef/internal/domain/round"
)

// ErrAlreadyClosed is returned when closing a closed session.
var ErrAlreadyClosed = errors.New("session: already closed")

// Status of a session. Active while the grill runs; closed once the
// legend round is submitted; dismissed when the human clears it from the
// station home. There is no observable "closing" phase in the grill-me
// flow — the shared-understanding confirmation happens in chat, outside
// the server.
type Status int

const (
	// StatusUndefined marks a malformed or unset status.
	StatusUndefined Status = iota
	// StatusActive means the grill is open (maps to "active" in state).
	StatusActive
	// StatusClosed means the legend round was submitted (maps to "closed").
	StatusClosed
	// StatusDismissed means the human cleared the session from the station
	// home (issue 14). Dismiss is a status change, never a delete; the
	// record is preserved and rehydrates as dismissable history.
	StatusDismissed
)

func (s Status) String() string {
	switch s {
	case StatusActive:
		return "active"
	case StatusClosed:
		return "closed"
	case StatusDismissed:
		return "dismissed"
	case StatusUndefined:
		return "undefined"
	default:
		return "unknown"
	}
}

// Session describes the grill: an id, a human-readable name
// (rendered as tab/heading), the ordered round history, and status.
// The daemon is stateless; the db is the real host of this state.
type Session struct {
	ID     string
	Name   string
	Rounds []round.Round
	Status Status

	// UpdatedAt is the store's staleness commodity (issue 1): it rides
	// the aggregate after rehydration, but is never consulted for
	// session consistency correctness.
	UpdatedAt time.Time
}

// New opens an active session with the given id and name.
func New(id, name string) *Session {
	return &Session{ID: id, Name: name, Status: StatusActive}
}

// Close marks the session closed. Called by the tool layer when the
// legend round's answers are stored; closing twice is an error so a
// double legend submission cannot corrupt the archive rules.
func (s *Session) Close() error {
	if s.Status == StatusClosed {
		return ErrAlreadyClosed
	}
	s.Status = StatusClosed
	return nil
}
