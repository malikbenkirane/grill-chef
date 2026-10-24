package session

import (
	"errors"
	"testing"
)

func TestStatusString(t *testing.T) {
	cases := []struct {
		name string
		s    Status
		want string
	}{
		{"undefined", StatusUndefined, "undefined"},
		{"active", StatusActive, "active"},
		{"closed", StatusClosed, "closed"},
		{"unknown", Status(9), "unknown"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.s.String(); got != tc.want {
				t.Fatalf("Status(%d).String() = %q, want %q", tc.s, got, tc.want)
			}
		})
	}
}

func TestClose(t *testing.T) {
	s := New("s1", "auth refactor")
	if s.Status != StatusActive {
		t.Fatalf("New().Status = %v, want StatusActive", s.Status)
	}
	if s.ID != "s1" || s.Name != "auth refactor" {
		t.Fatalf("New() = %+v, want id s1, name auth refactor", s)
	}

	if err := s.Close(); err != nil {
		t.Fatalf("Close() = %v, want nil", err)
	}
	if s.Status != StatusClosed {
		t.Fatalf("Status = %v, want StatusClosed", s.Status)
	}

	if err := s.Close(); !errors.Is(err, ErrAlreadyClosed) {
		t.Fatalf("second Close() = %v, want ErrAlreadyClosed", err)
	}
}
