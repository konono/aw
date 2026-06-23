package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/konono/aw/internal/launcher"
	"github.com/konono/aw/internal/messaging"
)

func runInternalAgentLoop(args []string) int {
	dbPath := os.Getenv("AW_MSG_DB")
	agentName := os.Getenv("AW_AGENT_NAME")
	teamName := os.Getenv("AW_TEAM_NAME")
	tool := os.Getenv("AW_TOOL")

	if dbPath == "" || agentName == "" || teamName == "" {
		fmt.Fprintln(os.Stderr, "AW_MSG_DB, AW_AGENT_NAME, AW_TEAM_NAME required")
		return 1
	}

	printCmd := launcher.ToolPrintCommand(tool)
	if printCmd == nil {
		fmt.Fprintf(os.Stderr, "tool %q does not support agent loop (print mode)\n", tool)
		return 1
	}

	fmt.Fprintf(os.Stderr, "[agent-loop] %s waiting for messages...\n", agentName)

	var lastID int64
	for {
		msgs := pollNewMessages(dbPath, teamName, agentName, lastID)
		for _, m := range msgs {
			fmt.Fprintf(os.Stderr, "[agent-loop] received message #%d from %s\n", m.ID, m.From)

			prompt := fmt.Sprintf(
				"You received a message from %s:\n\n%s\n\n"+
					"Read CLAUDE.md in the workspace for your role and team context. "+
					"Take the appropriate action and use the send_message MCP tool to respond.",
				m.From, m.Body)

			cmdArgs := append(printCmd, prompt)
			c := exec.Command(cmdArgs[0], cmdArgs[1:]...)
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			if runErr := c.Run(); runErr != nil {
				fmt.Fprintf(os.Stderr, "[agent-loop] tool exited with error: %v\n", runErr)
			}

			markRead(dbPath, m.ID)
			lastID = m.ID
			fmt.Fprintf(os.Stderr, "[agent-loop] message #%d processed, waiting for next...\n", m.ID)
		}
		time.Sleep(2 * time.Second)
	}
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
