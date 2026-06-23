package inject

import (
	"encoding/json"
	"fmt"
	"path/filepath"
)

// ClaudeInjector injects MCP and hook configuration for Claude Code.
//
// MCP:  writes .mcp.json into the workspace directory (WorkDir on the
//
//	host maps to the project root inside the container).
//
// Hook: patches settings.json in the staging directory to add a Stop
//
//	hook entry that runs "aw --internal-check-inbox".
type ClaudeInjector struct{}

// InjectMCP writes .mcp.json to cfg.WorkDir with the aw-msg MCP server.
func (c *ClaudeInjector) InjectMCP(cfg InjectorConfig) error {
	mcpPath := filepath.Join(cfg.WorkDir, ".mcp.json")

	// Load existing .mcp.json if present.
	var root map[string]interface{}
	if err := readJSONFile(mcpPath, &root); err != nil {
		return fmt.Errorf("reading existing .mcp.json: %w", err)
	}
	if root == nil {
		root = make(map[string]interface{})
	}

	servers, _ := root["mcpServers"].(map[string]interface{})
	if servers == nil {
		servers = make(map[string]interface{})
	}

	servers["aw-msg"] = map[string]interface{}{
		"command": cfg.MCPBinary,
		"args":    []string{"--internal-mcp-msg", "--db", cfg.DBPath, "--agent", cfg.AgentName},
	}
	root["mcpServers"] = servers

	return writeJSONFile(mcpPath, root)
}

// InjectHook patches settings.json in StagingDir to include a Stop hook
// that runs "aw --internal-check-inbox" to notify the agent of unread
// messages. If settings.json already exists, the hooks.Stop array is
// merged (appended if not already present) rather than overwritten.
func (c *ClaudeInjector) InjectHook(cfg InjectorConfig) error {
	settingsPath := filepath.Join(cfg.StagingDir, "settings.json")

	var root map[string]interface{}
	if err := readJSONFile(settingsPath, &root); err != nil {
		return fmt.Errorf("reading existing settings.json: %w", err)
	}
	if root == nil {
		root = make(map[string]interface{})
	}

	hooks, _ := root["hooks"].(map[string]interface{})
	if hooks == nil {
		hooks = make(map[string]interface{})
	}

	hookEntry := map[string]interface{}{
		"type":    "command",
		"command": cfg.MCPBinary + " --internal-check-inbox",
	}

	// Merge into the existing Stop array.
	var stopHooks []interface{}
	switch existing := hooks["Stop"].(type) {
	case []interface{}:
		stopHooks = existing
	case nil:
		// no existing Stop hooks
	default:
		// Unexpected type; wrap in a slice to preserve it.
		stopHooks = []interface{}{existing}
	}

	// Avoid duplicating the hook if it is already present.
	if !claudeStopHookExists(stopHooks, hookEntry) {
		stopHooks = append(stopHooks, hookEntry)
	}

	hooks["Stop"] = stopHooks
	root["hooks"] = hooks

	return writeJSONFile(settingsPath, root)
}

// claudeStopHookExists checks whether an equivalent hook entry already
// exists in the Stop array. It compares by the "command" field.
func claudeStopHookExists(hooks []interface{}, entry map[string]interface{}) bool {
	cmd, _ := entry["command"].(string)
	if cmd == "" {
		return false
	}
	for _, h := range hooks {
		m, ok := h.(map[string]interface{})
		if !ok {
			// After round-tripping through JSON unmarshal the type might
			// be map[string]json.RawMessage or similar; try re-marshal.
			data, err := json.Marshal(h)
			if err != nil {
				continue
			}
			m = make(map[string]interface{})
			if json.Unmarshal(data, &m) != nil {
				continue
			}
		}
		if c, _ := m["command"].(string); c == cmd {
			return true
		}
	}
	return false
}

// Verify interface compliance.
var _ Injector = (*ClaudeInjector)(nil)
