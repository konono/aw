package inject

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// CodexInjector injects MCP and hook configuration for Codex CLI.
//
// Codex uses a TOML config file (~/.codex/config.toml) for both MCP
// server registration and hooks.  Because the project does not depend
// on a TOML library we use simple string manipulation.
type CodexInjector struct{}

// InjectMCP appends an MCP server section to config.toml in the
// staging directory.
func (c *CodexInjector) InjectMCP(cfg InjectorConfig) error {
	configPath := filepath.Join(cfg.StagingDir, "config.toml")

	existing, err := os.ReadFile(configPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading config.toml: %w", err)
	}

	content := string(existing)

	section := codexMCPSection(cfg)
	if strings.Contains(content, "[[tools.aw-msg.mcp]]") {
		return nil // already present
	}

	content = ensureTrailingNewline(content) + section

	return writeFileAtomic(configPath, []byte(content), 0644)
}

// InjectHook appends a hook section to config.toml in the staging
// directory.
func (c *CodexInjector) InjectHook(cfg InjectorConfig) error {
	configPath := filepath.Join(cfg.StagingDir, "config.toml")

	existing, err := os.ReadFile(configPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading config.toml: %w", err)
	}

	content := string(existing)

	section := codexHookSection()
	if strings.Contains(content, "[hooks.aw-msg]") {
		return nil // already present
	}

	content = ensureTrailingNewline(content) + section

	return writeFileAtomic(configPath, []byte(content), 0644)
}

func codexMCPSection(cfg InjectorConfig) string {
	return fmt.Sprintf(`[[tools.aw-msg.mcp]]
command = "%s"
args = ["--db", "%s", "--agent", "%s"]
`, cfg.MCPBinary, cfg.DBPath, cfg.AgentName)
}

func codexHookSection() string {
	return `[hooks.aw-msg]
event = "stop"
command = "bash /home/agent/.aw-msg/check-inbox.sh"
`
}

// ensureTrailingNewline returns s with exactly one trailing newline.
func ensureTrailingNewline(s string) string {
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return ""
	}
	return s + "\n"
}

// Verify interface compliance.
var _ Injector = (*CodexInjector)(nil)
