package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/konono/aw/internal/messaging"
	"github.com/konono/aw/internal/messaging/mcp"
)

func runInternalMCPMsg(args []string) int {
	var dbPath, agentName, teamName string

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
		case "--team":
			if i+1 < len(args) {
				teamName = args[i+1]
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
	if teamName == "" {
		teamName = os.Getenv("AW_TEAM_NAME")
	}

	if dbPath == "" {
		fmt.Fprintln(os.Stderr, "AW_MSG_DB or --db is required")
		return 1
	}
	if agentName == "" {
		fmt.Fprintln(os.Stderr, "AW_AGENT_NAME or --agent is required")
		return 1
	}
	if teamName == "" {
		fmt.Fprintln(os.Stderr, "AW_TEAM_NAME or --team is required")
		return 1
	}

	if err := mcp.RunStdio(dbPath, agentName, teamName); err != nil {
		fmt.Fprintf(os.Stderr, "MCP server error: %v\n", err)
		return 1
	}
	return 0
}

const defaultCheckInterval = 60

func runInternalCheckInbox(args []string) int {
	var dbPath, agentName, teamName string

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
		case "--team":
			if i+1 < len(args) {
				teamName = args[i+1]
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
	if teamName == "" {
		teamName = os.Getenv("AW_TEAM_NAME")
	}

	if agentName == "" || dbPath == "" || teamName == "" {
		return 0
	}

	// Cooldown: skip if last check was within the interval
	interval := defaultCheckInterval
	if v := os.Getenv("AW_MSG_CHECK_INTERVAL"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			interval = n
		}
	}
	markerPath := filepath.Join(filepath.Dir(dbPath), ".lastcheck-"+agentName)
	if info, err := os.Stat(markerPath); err == nil {
		if time.Since(info.ModTime()) < time.Duration(interval)*time.Second {
			return 0
		}
	}

	store, err := messaging.OpenStore(dbPath)
	if err != nil {
		return 0
	}
	defer func() { _ = store.Close() }()

	count, err := store.UnreadCount(teamName, agentName)
	if err != nil || count == 0 {
		// Update marker even on zero so we don't re-check immediately
		_ = os.WriteFile(markerPath, nil, 0644)
		return 0
	}

	// Update marker
	_ = os.WriteFile(markerPath, nil, 0644)

	fmt.Printf("%d unread message(s). Use read_inbox tool to check.\n", count)
	return 0
}
