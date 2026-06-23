package inject

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// CursorInjector injects MCP configuration for Cursor.
//
// Cursor reads MCP server registration from mcp.json in its config
// directory (~/.cursor/).  The staging directory maps to that location.
//
// Cursor does not support hooks, so InjectHook is a no-op.
type CursorInjector struct{}

// InjectMCP writes or merges the aw-msg MCP server into mcp.json in
// the staging directory.
func (c *CursorInjector) InjectMCP(cfg InjectorConfig) error {
	mcpPath := filepath.Join(cfg.StagingDir, "mcp.json")

	data, err := os.ReadFile(mcpPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading cursor mcp.json: %w", err)
	}

	var root map[string]interface{}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &root); err != nil {
			return fmt.Errorf("parsing cursor mcp.json: %w", err)
		}
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

// InjectHook is a no-op for Cursor (hooks are not supported).
func (c *CursorInjector) InjectHook(_ InjectorConfig) error {
	return nil
}

// Verify interface compliance.
var _ Injector = (*CursorInjector)(nil)
