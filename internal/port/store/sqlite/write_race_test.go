package sqlite

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/malikbenkirane/grill-chef/internal/domain/round"
)

// The store must not corrupt when an agent tool call (MCP) and a browser
// autosave (daemon) race against the same session — different tables, WAL
// serializes them, busy_timeout absorbs the contention.
func TestMCPAndDaemonWritesRaceSafely(t *testing.T) {
	s := newTestStore(t)
	const sessions = 4
	for i := range sessions {
		if err := s.CreateSession(t.Context(), mkSess(fmt.Sprintf("rs%d", i))); err != nil {
			t.Fatal(err)
		}
	}
	errs := make(chan error, sessions*2)
	var wg sync.WaitGroup
	for i := range sessions {
		wg.Add(2)
		go func(i int) { // MCP: creates the question round
			defer wg.Done()
			errs <- s.CreateQuestionRound(t.Context(), mkQuestionRound(fmt.Sprintf("rs%d", i), 1, 2))
		}(i)
		go func(i int) { // daemon: autosaves against a query that must tolerate absence
			defer wg.Done()
			errs <- s.UpsertAnswer(t.Context(), fmt.Sprintf("rs%d", i), 1, 1, round.ChoiceIndex(1))
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil && !isMissingRoundFK(err) {
			t.Fatalf("unexpected write error under race: %v", err)
		}
	}
}

func isMissingRoundFK(err error) bool {
	return err != nil && strings.Contains(err.Error(), "FOREIGN KEY")
}
