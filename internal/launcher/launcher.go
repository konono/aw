package launcher

import (
	"context"
	"os"
	"path/filepath"

	"github.com/konono/aw/internal/pipeline"
)

// Launcher executes the final "run something" step of the pipeline.
type Launcher interface {
	Launch(ctx context.Context, ec *pipeline.ExecutionContext) error
}

// toolDevboxPkg returns the devbox/nixpkgs package name for the tool.
func toolDevboxPkg(tool string) string {
	switch tool {
	case "claude":
		return "claude-code"
	case "codex":
		return "codex"
	case "opencode":
		return "opencode"
	default:
		return ""
	}
}

// toolConfigPaths returns the host config home and container config dir for the tool.
func toolConfigPaths(tool, homeDir string) (hostHome, containerDir string) {
	switch tool {
	case "claude":
		return claudeHomePath(homeDir), "/home/agent/.claude"
	case "codex":
		return codexHomePath(homeDir), "/home/agent/.codex"
	case "opencode":
		return opencodeHomePath(homeDir), "/home/agent/.config/opencode"
	default:
		return "", ""
	}
}

// toolDataSymlinks returns AW_DATA_SYMLINKS value for tools that store data
// separately from config. Empty string means no symlinks needed.
func toolDataSymlinks(tool string) string {
	switch tool {
	case "opencode":
		return "/home/agent/.local/share/opencode:/home/agent/.config/opencode/data"
	default:
		return ""
	}
}

// buildContainerEnvVars creates the base set of environment variables for container execution.
func buildContainerEnvVars(ec *pipeline.ExecutionContext, tool string) map[string]string {
	envVars := make(map[string]string, len(ec.EnvVars)+5)
	for k, v := range ec.EnvVars {
		envVars[k] = v
	}

	if hostHome, containerDir := toolConfigPaths(tool, ec.HomeDir); hostHome != "" {
		envVars["AW_HOST_CONFIG_HOME"] = hostHome
		envVars["AW_CONTAINER_CONFIG_DIR"] = containerDir
	}
	if symlinks := toolDataSymlinks(tool); symlinks != "" {
		envVars["AW_DATA_SYMLINKS"] = symlinks
	}

	return envVars
}

func claudeHomePath(homeDir string) string {
	if v := os.Getenv("CLAUDE_HOME"); v != "" {
		return v
	}
	return filepath.Join(homeDir, ".claude")
}

func codexHomePath(homeDir string) string {
	if v := os.Getenv("CODEX_HOME"); v != "" {
		return v
	}
	return filepath.Join(homeDir, ".codex")
}

func opencodeHomePath(homeDir string) string {
	if v := os.Getenv("OPENCODE_CONFIG_DIR"); v != "" {
		return v
	}
	return filepath.Join(homeDir, ".config", "opencode")
}
