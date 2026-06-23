package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/konono/aw/internal/messaging"
)

func runInternalOnCommit(args []string) int {
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

	reviewers := os.Getenv("AW_REVIEWERS")
	if reviewers == "" {
		return 0
	}

	commitMsg, commitHash := latestCommitInfo()
	if commitHash == "" {
		return 0
	}

	// Dedup: skip if we already notified about this commit
	markerPath := filepath.Join(filepath.Dir(dbPath), ".lastcommit-"+agentName)
	if prev, err := os.ReadFile(markerPath); err == nil && strings.TrimSpace(string(prev)) == commitHash {
		return 0
	}
	_ = os.WriteFile(markerPath, []byte(commitHash), 0644)

	store, err := messaging.OpenStore(dbPath)
	if err != nil {
		return 0
	}
	defer func() { _ = store.Close() }()

	shortHash := commitHash
	if len(shortHash) > 8 {
		shortHash = shortHash[:8]
	}
	body := fmt.Sprintf("New commit by %s: %s\n\n%s", agentName, shortHash, commitMsg)

	for _, reviewer := range strings.Split(reviewers, ",") {
		reviewer = strings.TrimSpace(reviewer)
		if reviewer != "" {
			_, _, _ = store.Send(teamName, agentName, reviewer, body)
		}
	}
	return 0
}

func latestCommitInfo() (message string, hash string) {
	cmd := exec.Command("git", "log", "-1", "--format=%H%n%B")
	out, err := cmd.Output()
	if err != nil {
		return "", ""
	}
	parts := strings.SplitN(strings.TrimSpace(string(out)), "\n", 2)
	if len(parts) == 0 {
		return "", ""
	}
	hash = parts[0]
	if len(parts) > 1 {
		message = strings.TrimSpace(parts[1])
	}
	return message, hash
}
