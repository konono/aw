package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/konono/aw/internal/messaging"
)

func runMsg(args []string) int {
	if len(args) == 0 {
		printMsgHelp()
		return 1
	}

	switch args[0] {
	case "send":
		return runMsgSend(args[1:])
	case "inbox":
		return runMsgInbox(args[1:])
	case "history":
		return runMsgHistory(args[1:])
	case "watch":
		return runMsgWatch(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "Unknown msg command: %s\n", args[0])
		printMsgHelp()
		return 1
	}
}

func printMsgHelp() {
	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprintln(os.Stderr, "  aw msg send --team <team> <from> <to> <body>   Send a message")
	fmt.Fprintln(os.Stderr, "  aw msg inbox --team <team> <agent>             Show unread messages")
	fmt.Fprintln(os.Stderr, "  aw msg history --team <team> [--agent <name>]  Show message history")
	fmt.Fprintln(os.Stderr, "  aw msg watch --team <team>                     Watch all messages in real-time")
}

func openMsgStore() (*messaging.Store, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	dbPath := filepath.Join(home, ".config", "aw", "messaging", "messages.db")
	return messaging.OpenStore(dbPath)
}

// extractTeamFlag extracts --team value from args, returning the team name
// and remaining args.
func extractTeamFlag(args []string) (string, []string) {
	var team string
	var rest []string
	for i := 0; i < len(args); i++ {
		if args[i] == "--team" && i+1 < len(args) {
			team = args[i+1]
			i++
		} else {
			rest = append(rest, args[i])
		}
	}
	return team, rest
}

func runMsgSend(args []string) int {
	team, args := extractTeamFlag(args)
	if team == "" || len(args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: aw msg send --team <team> <from> <to> <body>")
		return 1
	}
	from, to, body := args[0], args[1], args[2]

	store, err := openMsgStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	defer func() { _ = store.Close() }()

	id, ts, err := store.Send(team, from, to, body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error sending message: %v\n", err)
		return 1
	}
	fmt.Printf("Message #%d sent to %s at %s\n", id, to, ts.Format("15:04:05"))
	return 0
}

func runMsgInbox(args []string) int {
	team, args := extractTeamFlag(args)
	if team == "" || len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: aw msg inbox --team <team> <agent>")
		return 1
	}
	agent := args[0]

	store, err := openMsgStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	defer func() { _ = store.Close() }()

	previews, err := store.ReadInbox(team, agent)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	if len(previews) == 0 {
		fmt.Printf("No unread messages for %s\n", agent)
		return 0
	}

	fmt.Printf("Unread messages for %s:\n", agent)
	for _, p := range previews {
		fmt.Printf("  #%d [%s] %s: %s\n", p.ID, p.CreatedAt.Format("15:04:05"), p.From, p.Preview)
	}
	return 0
}

func runMsgHistory(args []string) int {
	team, args := extractTeamFlag(args)
	if team == "" {
		fmt.Fprintln(os.Stderr, "Usage: aw msg history --team <team> [--agent <name>] [--limit N]")
		return 1
	}

	var agent string
	limit := 50
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--agent":
			if i+1 < len(args) {
				agent = args[i+1]
				i++
			}
		case "--limit":
			if i+1 < len(args) {
				n, err := strconv.Atoi(args[i+1])
				if err == nil {
					limit = n
				}
				i++
			}
		}
	}

	store, err := openMsgStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	defer func() { _ = store.Close() }()

	msgs, err := store.History(team, agent, limit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	if len(msgs) == 0 {
		fmt.Println("No messages")
		return 0
	}

	for i := len(msgs) - 1; i >= 0; i-- {
		m := msgs[i]
		fmt.Printf("%s %s → %s: %s\n", m.CreatedAt.Format("15:04:05"), m.From, m.To, messaging.FormatPreview(m.Body))
	}
	return 0
}

func runMsgWatch(args []string) int {
	team, _ := extractTeamFlag(args)
	if team == "" {
		fmt.Fprintln(os.Stderr, "Usage: aw msg watch --team <team>")
		return 1
	}

	store, err := openMsgStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	defer func() { _ = store.Close() }()

	fmt.Println("Watching messages... (Ctrl+C to stop)")
	if err := store.WatchAll(team, 2*time.Second, func(m messaging.Message) {
		fmt.Printf("%s %s → %s: %s\n", m.CreatedAt.Format("15:04:05"), m.From, m.To,
			messaging.FormatPreview(m.Body))
	}); err != nil {
		fmt.Fprintf(os.Stderr, "Error watching messages: %v\n", err)
		return 1
	}
	return 0
}
