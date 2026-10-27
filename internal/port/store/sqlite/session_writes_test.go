package sqlite

import (
	"testing"

	"github.com/malikbenkirane/grill-chef/internal/domain/session"
)

func TestCreateSessionStoresDomainAsIs(t *testing.T) {
	s := newTestStore(t)
	in := session.New("sess-1", "Grill: state db")
	if err := s.CreateSession(t.Context(), in); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	var status string
	if err := s.db.QueryRow(`SELECT status FROM sessions WHERE id = ?`, "sess-1").Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "active" {
		t.Fatalf("status literal: want active, got %q", status)
	}
}

func TestCreateSessionRejectsUndefinedStatus(t *testing.T) {
	s := newTestStore(t)
	in := &session.Session{ID: "bad", Name: "n", Status: session.StatusUndefined}
	if err := s.CreateSession(t.Context(), in); err == nil {
		t.Fatal("undefined status must be rejected; the store never invents state")
	}
}

func TestCreateSessionDuplicateIdFails(t *testing.T) {
	s := newTestStore(t)
	in := session.New("dup", "name")
	if err := s.CreateSession(t.Context(), in); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateSession(t.Context(), session.New("dup", "name")); err == nil {
		t.Fatal("duplicate session id must fail")
	}
}

func TestCloseSessionFlipsStatusExactlyOnce(t *testing.T) {
	s := newTestStore(t)
	if err := s.CreateSession(t.Context(), session.New("c1", "n")); err != nil {
		t.Fatal(err)
	}
	if err := s.CloseSession(t.Context(), "c1"); err != nil {
		t.Fatalf("CloseSession: %v", err)
	}
	var status string
	if err := s.db.QueryRow(`SELECT status FROM sessions WHERE id = ?`, "c1").Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "closed" {
		t.Fatalf("want closed, got %q", status)
	}
	if err := s.CloseSession(t.Context(), "no-such"); err == nil {
		t.Fatal("closing unknown session must error")
	}
}
