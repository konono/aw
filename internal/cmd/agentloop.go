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

	store, err := messaging.OpenStore(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening store: %v\n", err)
		return 1
	}
	defer func() { _ = store.Close() }()

	fmt.Fprintf(os.Stderr, "[agent-loop] %s waiting for messages...\n", agentName)

	err = store.Watch(teamName, agentName, 2*time.Second, func(m messaging.Message) {
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

		_ = store.MarkRead(m.ID)
		fmt.Fprintf(os.Stderr, "[agent-loop] message #%d processed, waiting for next...\n", m.ID)
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "[agent-loop] watch error: %v\n", err)
		return 1
	}
	return 0
}
