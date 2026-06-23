package inject

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// MemberInfo describes a team member visible to an agent.
type MemberInfo struct {
	AgentName string
	Role      string
}

// DeliveryMode controls how an agent receives message notifications.
type DeliveryMode string

const (
	DeliveryTurn    DeliveryMode = "turn"    // Stop hook notifies of unread messages
	DeliveryMonitor DeliveryMode = "monitor" // SessionStart watcher streams messages in background
	DeliveryOff     DeliveryMode = "off"     // MCP pull only, no automatic notification
)

// InjectorConfig carries all parameters needed to inject MCP and hook
// configuration for inter-agent messaging.
type InjectorConfig struct {
	AgentName    string       // this agent's name (e.g. "lead-1")
	TeamName     string       // team scope key (e.g. "review-team-a1b2c3-uuid")
	Role         string       // role of this agent (e.g. "lead", "developer")
	Members      []MemberInfo // all team members (including self)
	StagingDir   string       // host staging dir (e.g. ~/.agent-workspace/claude/)
	WorkDir      string       // workspace dir mounted into container
	MCPBinary    string       // path to aw binary inside container
	DBPath       string       // path to messages.db inside container
	DeliveryMode DeliveryMode // how to notify the agent of new messages
}

// Injector writes tool-specific MCP server registration and hook
// configuration files so that an AI coding agent can use inter-agent
// messaging once the container starts.
type Injector interface {
	// InjectMCP registers the aw-msg MCP server with the tool.
	InjectMCP(cfg InjectorConfig) error
	// InjectHook installs a stop/post-response hook that checks the inbox.
	InjectHook(cfg InjectorConfig) error
}

// ForTool returns the Injector for the given tool name, or an error if
// the tool is not supported.
func ForTool(tool string) (Injector, error) {
	switch tool {
	case "claude":
		return &ClaudeInjector{}, nil
	case "codex":
		return &CodexInjector{}, nil
	case "opencode":
		return &OpenCodeInjector{}, nil
	case "cursor":
		return &CursorInjector{}, nil
	default:
		return nil, fmt.Errorf("unsupported tool for messaging injection: %s", tool)
	}
}

// ---------- helpers shared across injectors ----------

// writeFileAtomic writes data to path, creating parent directories as needed.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating parent dir for %s: %w", path, err)
	}
	return os.WriteFile(path, data, perm)
}

// readJSONFile reads a JSON file into dst. If the file does not exist,
// dst is left untouched and nil is returned.
func readJSONFile(path string, dst interface{}) error {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dst)
}

// writeJSONFile marshals v as indented JSON and writes it to path.
func writeJSONFile(path string, v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return writeFileAtomic(path, data, 0644)
}
