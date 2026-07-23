package inject

import "path/filepath"

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
	return injectMCPServer(filepath.Join(cfg.StagingDir, "opencode.json"), "mcp", cfg)
}

// InjectHook is a no-op for OpenCode (hooks are not supported).
func (o *OpenCodeInjector) InjectHook(_ InjectorConfig) error {
	return nil
}

// Verify interface compliance.
var _ Injector = (*OpenCodeInjector)(nil)
