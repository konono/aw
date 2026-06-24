package inject

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// OpenCodeInjector injects MCP configuration for OpenCode.
//
// OpenCode reads its configuration from opencode.json in its config
// directory (~/.config/opencode/).  MCP servers live in a top-level
// "mcp" object keyed by server name.
//
// OpenCode does not support hooks, so InjectHook is a no-op.
type OpenCodeInjector struct{}

// InjectMCP merges the aw-msg MCP server into opencode.json in the
// staging directory.
func (o *OpenCodeInjector) InjectMCP(cfg InjectorConfig) error {
	configPath := filepath.Join(cfg.StagingDir, "opencode.json")

	data, err := os.ReadFile(configPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading opencode.json: %w", err)
	}

	var root map[string]interface{}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &root); err != nil {
			return fmt.Errorf("parsing opencode.json: %w", err)
		}
	}
	if root == nil {
		root = make(map[string]interface{})
	}

	mcpServers, _ := root["mcp"].(map[string]interface{})
	if mcpServers == nil {
		mcpServers = make(map[string]interface{})
	}

	mcpServers["aw-msg"] = map[string]interface{}{
		"command": cfg.MCPBinary,
		"args":    []string{"internal-mcp-msg", "--db", cfg.DBPath, "--agent", cfg.AgentName, "--team", cfg.TeamName},
	}
	root["mcp"] = mcpServers

	return writeJSONFile(configPath, root)
}

// InjectHook is a no-op for OpenCode (hooks are not supported).
func (o *OpenCodeInjector) InjectHook(_ InjectorConfig) error {
	return nil
}

// Verify interface compliance.
var _ Injector = (*OpenCodeInjector)(nil)
