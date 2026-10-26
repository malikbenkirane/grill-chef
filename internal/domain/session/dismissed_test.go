package session

import "testing"

func TestStatusDismissed(t *testing.T) {
	if StatusDismissed != Status(3) {
		t.Fatalf("StatusDismissed must be the iota follower of StatusClosed, got %d", StatusDismissed)
	}
	if got := StatusDismissed.String(); got != "dismissed" {
		t.Fatalf("StatusDismissed.String(): want %q, got %q", "dismissed", got)
	}
}
