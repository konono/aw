package inject

import "path/filepath"

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
	return injectMCPServer(filepath.Join(cfg.StagingDir, "mcp.json"), "mcpServers", cfg)
}

// InjectHook is a no-op for Cursor (hooks are not supported).
func (c *CursorInjector) InjectHook(_ InjectorConfig) error {
	return nil
}

// Verify interface compliance.
var _ Injector = (*CursorInjector)(nil)
