package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/konono/aw/v4/internal/messaging"
	"github.com/konono/aw/v4/internal/messaging/mcp"
)

// Run handles the internal MCP message server.
func (m *InternalMCPMsgCmd) Run() error {
	if m.DB == "" {
		return fmt.Errorf("AW_MSG_DB or --db is required")
	}
	if m.Agent == "" {
		return fmt.Errorf("AW_AGENT_NAME or --agent is required")
	}
	if m.Team == "" {
		return fmt.Errorf("AW_TEAM_NAME or --team is required")
	}

	return mcp.RunStdio(m.DB, m.Agent, m.Team)
}

const defaultCheckInterval = 60

// Run handles internal check-inbox.
func (m *InternalCheckInboxCmd) Run() error {
	if m.Agent == "" || m.DB == "" || m.Team == "" {
		return nil
	}

	// Cooldown: skip if last check was within the interval
	interval := defaultCheckInterval
	if v := os.Getenv("AW_MSG_CHECK_INTERVAL"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			interval = n
		}
	}
	markerPath := filepath.Join(filepath.Dir(m.DB), ".lastcheck-"+m.Agent)
	if info, err := os.Stat(markerPath); err == nil {
		if time.Since(info.ModTime()) < time.Duration(interval)*time.Second {
			return nil
		}
	}

	store, err := messaging.OpenStore(m.DB)
	if err != nil {
		fmt.Fprintf(os.Stderr, "check-inbox: %v\n", err)
		return nil
	}
	defer func() { _ = store.Close() }()

	count, err := store.UnreadCount(m.Team, m.Agent)
	if err != nil || count == 0 {
		// Update marker even on zero so we don't re-check immediately
		_ = os.WriteFile(markerPath, nil, 0644)
		return nil
	}

	// Update marker
	_ = os.WriteFile(markerPath, nil, 0644)

	fmt.Println(checkInboxResponse(count))
	return nil
}

type hookResponse struct {
	Decision string `json:"decision"`
	Reason   string `json:"reason"`
}

func checkInboxResponse(count int) string {
	if count <= 0 {
		return ""
	}
	resp := hookResponse{
		Decision: "block",
		Reason:   fmt.Sprintf("%d unread message(s). Use read_inbox tool to check.", count),
	}
	data, _ := json.Marshal(resp)
	return string(data)
}
