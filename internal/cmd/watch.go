package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/konono/aw/v4/internal/messaging"
)

// Run handles internal watch.
func (w *InternalWatchCmd) Run() error {
	if w.Agent == "" || w.DB == "" || w.Team == "" {
		fmt.Fprintln(os.Stderr, "AW_MSG_DB, AW_AGENT_NAME, AW_TEAM_NAME required")
		return ExitError{Code: 1}
	}

	store, err := messaging.OpenStore(w.DB)
	if err != nil {
		return fmt.Errorf("opening store: %w", err)
	}
	defer func() { _ = store.Close() }()

	return store.Watch(w.Team, w.Agent, 2*time.Second, func(m messaging.Message) {
		fmt.Printf("[msg from %s] %s\n", m.From, m.Body)
	})
}
