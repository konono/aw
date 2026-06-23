package inject

import (
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
// messages. Claude Code hooks use a matcher+hooks structure:
//
//	{"hooks": {"Stop": [{"hooks": [{"type":"command","command":"..."}]}]}}
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

	command := cfg.MCPBinary + " --internal-check-inbox"

	// Check if our hook is already present in any Stop matcher group.
	stopGroups, _ := hooks["Stop"].([]interface{})
	if claudeStopHookExists(stopGroups, command) {
		return nil
	}

	// Append a new matcher group with our hook.
	stopGroups = append(stopGroups, map[string]interface{}{
		"hooks": []interface{}{
			map[string]interface{}{
				"type":    "command",
				"command": command,
			},
		},
	})

	hooks["Stop"] = stopGroups
	root["hooks"] = hooks

	return writeJSONFile(settingsPath, root)
}

// claudeStopHookExists checks whether the command already exists in any
// Stop matcher group's hooks array.
func claudeStopHookExists(groups []interface{}, command string) bool {
	for _, g := range groups {
		group, ok := g.(map[string]interface{})
		if !ok {
			continue
		}
		innerHooks, _ := group["hooks"].([]interface{})
		for _, h := range innerHooks {
			m, ok := h.(map[string]interface{})
			if !ok {
				continue
			}
			if c, _ := m["command"].(string); c == command {
				return true
			}
		}
	}
	return false
}

// Verify interface compliance.
var _ Injector = (*ClaudeInjector)(nil)
