package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/konono/aw/v4/internal/launcher"
	"github.com/konono/aw/v4/internal/messaging"
)

// Run handles internal agent loop.
func (a *InternalAgentLoopCmd) Run() error {
	dbPath := a.DB
	agentName := a.Agent
	teamName := a.Team
	tool := a.Tool

	if dbPath == "" || agentName == "" || teamName == "" {
		return fmt.Errorf("AW_MSG_DB, AW_AGENT_NAME, AW_TEAM_NAME required")
	}

	printCmd := launcher.ToolPrintCommand(tool)
	if printCmd == nil {
		return fmt.Errorf("tool %q does not support agent loop (print mode)", tool)
	}

	contextFile := "CLAUDE.md"
	if tool == "codex" || tool == "opencode" {
		contextFile = "AGENTS.md"
	}

	// Process any unread messages from previous sessions as a single summary
	lastID := processUnreadSummary(dbPath, teamName, agentName, tool, printCmd, contextFile)
	fmt.Fprintf(os.Stderr, "[agent-loop] %s waiting for messages (last_id=%d)...\n", agentName, lastID)

	for {
		msgs := pollNewMessages(dbPath, teamName, agentName, lastID)
		for _, m := range msgs {
			fmt.Fprintf(os.Stderr, "[agent-loop] received message #%d from %s\n", m.ID, m.From)

			prompt := fmt.Sprintf(
				"You received a message from %s:\n\n%s\n\n"+
					"Read %s in the workspace for your role and team context. "+
					"Take the appropriate action and use the send_message MCP tool to respond.",
				m.From, m.Body, contextFile)

			cmdArgs := append(printCmd, prompt)
			c := exec.Command(cmdArgs[0], cmdArgs[1:]...)
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			if runErr := c.Run(); runErr != nil {
				fmt.Fprintf(os.Stderr, "[agent-loop] tool exited with error: %v (message #%d will be retried)\n", runErr, m.ID)
				continue
			}

			markRead(dbPath, m.ID)
			lastID = m.ID
			fmt.Fprintf(os.Stderr, "[agent-loop] message #%d processed, waiting for next...\n", m.ID)
		}
		time.Sleep(2 * time.Second)
	}
}

func processUnreadSummary(dbPath, team, agent, tool string, printCmd []string, contextFile string) int64 {
	store, err := messaging.OpenStore(dbPath)
	if err != nil {
		return 0
	}
	defer func() { _ = store.Close() }()

	unread, err := store.ReadInbox(team, agent)
	if err != nil || len(unread) == 0 {
		return latestMessageID(dbPath, team, agent)
	}

	fmt.Fprintf(os.Stderr, "[agent-loop] found %d unread message(s) from previous session, processing summary...\n", len(unread))

	// Build summary of senders and latest preview
	senders := map[string]int{}
	var latestPreview string
	var maxID int64
	for _, p := range unread {
		senders[p.From]++
		latestPreview = p.Preview
		if p.ID > maxID {
			maxID = p.ID
		}
	}

	var senderList []string
	for name, count := range senders {
		senderList = append(senderList, fmt.Sprintf("%s (%d)", name, count))
	}

	prompt := fmt.Sprintf(
		"You have %d unread message(s) from a previous session.\n"+
			"Senders: %s\n"+
			"Latest message preview: %s\n\n"+
			"Read %s in the workspace for your role and team context. "+
			"Use read_inbox and read_message MCP tools to review important messages, "+
			"then take appropriate action and respond via send_message if needed.",
		len(unread), strings.Join(senderList, ", "), latestPreview, contextFile)

	cmdArgs := append(printCmd, prompt)
	c := exec.Command(cmdArgs[0], cmdArgs[1:]...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if runErr := c.Run(); runErr != nil {
		fmt.Fprintf(os.Stderr, "[agent-loop] summary processing failed: %v, falling back to individual processing\n", runErr)
		if unread[0].ID > 0 {
			return unread[0].ID - 1
		}
		return 0
	}

	// Mark all unread as read
	for _, p := range unread {
		markRead(dbPath, p.ID)
	}
	fmt.Fprintf(os.Stderr, "[agent-loop] %d unread message(s) marked as read\n", len(unread))

	return maxID
}

func latestMessageID(dbPath, team, agent string) int64 {
	store, err := messaging.OpenStore(dbPath)
	if err != nil {
		return 0
	}
	defer func() { _ = store.Close() }()

	var maxID int64
	_ = store.DB().QueryRow(
		`SELECT COALESCE(MAX(id), 0) FROM messages WHERE team = ? AND to_agent = ?`,
		team, agent,
	).Scan(&maxID)
	return maxID
}

func pollNewMessages(dbPath, team, agent string, afterID int64) []messaging.Message {
	store, err := messaging.OpenStore(dbPath)
	if err != nil {
		return nil
	}
	defer func() { _ = store.Close() }()

	rows, err := store.DB().Query(
		`SELECT id, from_agent, to_agent, body, created_at FROM messages
		 WHERE team = ? AND to_agent = ? AND id > ? ORDER BY id`,
		team, agent, afterID,
	)
	if err != nil {
		return nil
	}
	defer func() { _ = rows.Close() }()

	var msgs []messaging.Message
	for rows.Next() {
		var m messaging.Message
		var ts string
		if err := rows.Scan(&m.ID, &m.From, &m.To, &m.Body, &ts); err != nil {
			continue
		}
		m.CreatedAt, _ = time.Parse("2006-01-02T15:04:05Z", ts)
		msgs = append(msgs, m)
	}
	return msgs
}

func markRead(dbPath string, id int64) {
	store, err := messaging.OpenStore(dbPath)
	if err != nil {
		return
	}
	defer func() { _ = store.Close() }()
	_ = store.MarkRead(id)
}
