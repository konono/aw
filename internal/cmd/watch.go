package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/konono/aw/internal/messaging"
)

func runInternalWatch(args []string) int {
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
		fmt.Fprintln(os.Stderr, "AW_MSG_DB, AW_AGENT_NAME, AW_TEAM_NAME required")
		return 1
	}

	store, err := messaging.OpenStore(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening store: %v\n", err)
		return 1
	}
	defer func() { _ = store.Close() }()

	err = store.Watch(teamName, agentName, 2*time.Second, func(m messaging.Message) {
		fmt.Printf("[msg from %s] %s\n", m.From, m.Body)
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error watching: %v\n", err)
		return 1
	}
	return 0
}
