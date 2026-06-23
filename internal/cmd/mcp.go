package cmd

import (
	"fmt"
	"os"

	"github.com/konono/aw/internal/messaging/mcp"
)

func runInternalMCPMsg() int {
	dbPath := os.Getenv("AW_MSG_DB")
	if dbPath == "" {
		fmt.Fprintln(os.Stderr, "AW_MSG_DB environment variable is required")
		return 1
	}

	agentName := os.Getenv("AW_AGENT_NAME")
	if agentName == "" {
		fmt.Fprintln(os.Stderr, "AW_AGENT_NAME environment variable is required")
		return 1
	}

	if err := mcp.RunStdio(dbPath, agentName); err != nil {
		fmt.Fprintf(os.Stderr, "MCP server error: %v\n", err)
		return 1
	}
	return 0
}
