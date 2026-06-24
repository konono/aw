package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/konono/aw/internal/messaging"
)

func openMsgStore() (*messaging.Store, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	dbPath := filepath.Join(home, ".config", "aw", "messaging", "messages.db")
	return messaging.OpenStore(dbPath)
}

// Run handles msg send.
func (m *MsgSendCmd) Run() error {
	store, err := openMsgStore()
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()

	id, ts, err := store.Send(m.Team, m.From, m.To, m.Body)
	if err != nil {
		return fmt.Errorf("sending message: %w", err)
	}
	fmt.Printf("Message #%d sent to %s at %s\n", id, m.To, ts.Format("15:04:05"))
	return nil
}

// Run handles msg inbox.
func (m *MsgInboxCmd) Run() error {
	store, err := openMsgStore()
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()

	previews, err := store.ReadInbox(m.Team, m.Agent)
	if err != nil {
		return err
	}

	if len(previews) == 0 {
		fmt.Printf("No unread messages for %s\n", m.Agent)
		return nil
	}

	fmt.Printf("Unread messages for %s:\n", m.Agent)
	for _, p := range previews {
		fmt.Printf("  #%d [%s] %s: %s\n", p.ID, p.CreatedAt.Format("15:04:05"), p.From, p.Preview)
	}
	return nil
}

// Run handles msg history.
func (m *MsgHistoryCmd) Run() error {
	store, err := openMsgStore()
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()

	msgs, err := store.History(m.Team, m.Agent, m.Limit)
	if err != nil {
		return err
	}

	if len(msgs) == 0 {
		fmt.Println("No messages")
		return nil
	}

	for i := len(msgs) - 1; i >= 0; i-- {
		msg := msgs[i]
		fmt.Printf("%s %s → %s: %s\n", msg.CreatedAt.Format("15:04:05"), msg.From, msg.To, messaging.FormatPreview(msg.Body))
	}
	return nil
}

// Run handles msg watch.
func (m *MsgWatchCmd) Run() error {
	store, err := openMsgStore()
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()

	fmt.Println("Watching messages... (Ctrl+C to stop)")
	return store.WatchAll(m.Team, 2*time.Second, func(msg messaging.Message) {
		fmt.Printf("%s %s → %s: %s\n", msg.CreatedAt.Format("15:04:05"), msg.From, msg.To,
			messaging.FormatPreview(msg.Body))
	})
}

// Run handles msg clear.
func (m *MsgClearCmd) Run() error {
	if m.List {
		return runMsgClearList()
	}

	if !m.All && m.Team == "" && m.Before == "" {
		fmt.Fprintln(os.Stderr, "Usage: aw msg clear [--team <team>] [--all] [--before <duration>] [--list]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Options:")
		fmt.Fprintln(os.Stderr, "  --team <scope>     Delete messages for a specific team scope")
		fmt.Fprintln(os.Stderr, "  --all              Delete all messages")
		fmt.Fprintln(os.Stderr, "  --before <dur>     Delete messages older than duration (e.g. 7d, 24h, 30m)")
		fmt.Fprintln(os.Stderr, "  --list             Show available team scopes")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "  --team and --before can be combined (but not with --all):")
		fmt.Fprintln(os.Stderr, "  aw msg clear --team <scope> --before 7d   Delete old messages for a team")
		return ExitError{Code: 1}
	}

	var d time.Duration
	if m.Before != "" {
		var parseErr error
		d, parseErr = parseDuration(m.Before)
		if parseErr != nil {
			return fmt.Errorf("invalid duration %q: %w", m.Before, parseErr)
		}
	}

	store, err := openMsgStore()
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()

	var count int64

	switch {
	case m.All:
		count, err = store.ClearAll()
	case m.Team != "" && m.Before != "":
		count, err = store.ClearTeamBefore(m.Team, d)
	case m.Team != "":
		count, err = store.ClearTeam(m.Team)
	case m.Before != "":
		count, err = store.ClearBefore(d)
	}

	if err != nil {
		return err
	}

	fmt.Printf("Deleted %d message(s)\n", count)
	return nil
}

func runMsgClearList() error {
	store, err := openMsgStore()
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()

	teams, err := store.ListTeams()
	if err != nil {
		return err
	}

	if len(teams) == 0 {
		fmt.Println("No messages in database")
		return nil
	}

	fmt.Println("Team scopes:")
	for _, t := range teams {
		fmt.Printf("  %s\n", t)
	}
	return nil
}

func parseDuration(s string) (time.Duration, error) {
	if len(s) < 2 {
		return 0, fmt.Errorf("too short (use e.g. 7d, 24h, 30m)")
	}
	unit := s[len(s)-1]
	numStr := s[:len(s)-1]
	n, err := strconv.Atoi(numStr)
	if err != nil {
		return 0, fmt.Errorf("invalid number %q", numStr)
	}
	if n <= 0 {
		return 0, fmt.Errorf("duration must be positive, got %d", n)
	}
	switch unit {
	case 'd':
		return time.Duration(n) * 24 * time.Hour, nil
	case 'h':
		return time.Duration(n) * time.Hour, nil
	case 'm':
		return time.Duration(n) * time.Minute, nil
	default:
		return 0, fmt.Errorf("unknown unit %q (use d, h, m)", string(unit))
	}
}
