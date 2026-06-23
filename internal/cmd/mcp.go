package cmd

import (
	"fmt"
	"os"

	"github.com/konono/aw/internal/messaging"
	"github.com/konono/aw/internal/messaging/mcp"
)

func runInternalMCPMsg(args []string) int {
	var dbPath, agentName string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--db":
			if i+1 < len(args) {
				dbPath = args[i+1]
				i++
			}
		case "--agent":
			if i+1 < len(args) {
				agentName = args[i+1]
				i++
			}
		}
	}

	if dbPath == "" {
		dbPath = os.Getenv("AW_MSG_DB")
	}
	if agentName == "" {
		agentName = os.Getenv("AW_AGENT_NAME")
	}

	if dbPath == "" {
		fmt.Fprintln(os.Stderr, "AW_MSG_DB or --db is required")
		return 1
	}
	if agentName == "" {
		fmt.Fprintln(os.Stderr, "AW_AGENT_NAME or --agent is required")
		return 1
	}

	if err := mcp.RunStdio(dbPath, agentName); err != nil {
		fmt.Fprintf(os.Stderr, "MCP server error: %v\n", err)
		return 1
	}
	return 0
}

func runInternalCheckInbox(args []string) int {
	var dbPath, agentName string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--db":
			if i+1 < len(args) {
				dbPath = args[i+1]
				i++
			}
		case "--agent":
			if i+1 < len(args) {
				agentName = args[i+1]
				i++
			}
		}
	}

	if dbPath == "" {
		dbPath = os.Getenv("AW_MSG_DB")
	}
	if agentName == "" {
		agentName = os.Getenv("AW_AGENT_NAME")
	}

	if agentName == "" || dbPath == "" {
		return 0
	}

	store, err := messaging.OpenStore(dbPath)
	if err != nil {
		return 0
	}
	defer func() { _ = store.Close() }()

	count, err := store.UnreadCount(agentName)
	if err != nil || count == 0 {
		return 0
	}

	fmt.Printf("%d unread message(s). Use read_inbox tool to check.\n", count)
	return 0
}
