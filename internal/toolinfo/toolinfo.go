package toolinfo

import (
	"os"
	"path/filepath"

	"github.com/konono/aw/internal/containerenv"
)

// ToolSpec holds static per-tool metadata.
type ToolSpec struct {
	Binary            string
	DisplayName       string
	DevboxPkg         string
	HomeEnvVar        string
	DefaultHomeSubdir string
	ContainerDir      string
	DataSymlinks      string
	InstallHint       string
}

var tools = map[string]ToolSpec{
	"claude": {
		Binary:            "claude",
		DisplayName:       "Claude Code",
		DevboxPkg:         "claude-code",
		HomeEnvVar:        "CLAUDE_HOME",
		DefaultHomeSubdir: ".claude",
		ContainerDir:      "/home/agent/.claude",
		InstallHint:       "Install Claude Code: https://claude.ai/install.sh",
	},
	"codex": {
		Binary:            "codex",
		DisplayName:       "Codex",
		DevboxPkg:         "codex",
		HomeEnvVar:        "CODEX_HOME",
		DefaultHomeSubdir: ".codex",
		ContainerDir:      "/home/agent/.codex",
		InstallHint:       "Install Codex CLI: npm i -g @openai/codex",
	},
	"opencode": {
		Binary:            "opencode",
		DisplayName:       "OpenCode",
		DevboxPkg:         "opencode",
		HomeEnvVar:        "OPENCODE_CONFIG_DIR",
		DefaultHomeSubdir: filepath.Join(".config", "opencode"),
		ContainerDir:      "/home/agent/.config/opencode",
		DataSymlinks:      "/home/agent/.local/share/opencode:/home/agent/.config/opencode/data",
		InstallHint:       "Install via: devbox add opencode",
	},
}

// Lookup returns the ToolSpec for the given tool name.
func Lookup(tool string) (ToolSpec, bool) {
	spec, ok := tools[tool]
	return spec, ok
}

func DevboxPkg(tool string) string {
	if spec, ok := Lookup(tool); ok {
		return spec.DevboxPkg
	}
	return ""
}

func HomePath(tool, homeDir string) string {
	spec, ok := Lookup(tool)
	if !ok {
		return ""
	}
	if spec.HomeEnvVar != "" {
		if v := os.Getenv(spec.HomeEnvVar); v != "" {
			return v
		}
	}
	return filepath.Join(homeDir, spec.DefaultHomeSubdir)
}

func ContainerDir(tool string) string {
	return ContainerDirFor(tool, containerenv.Default())
}

func ContainerDirFor(tool string, cenv containerenv.Config) string {
	return cenv.ToolDir(tool)
}

func DataSymlinks(tool string) string {
	return DataSymlinksFor(tool, containerenv.Default())
}

func DataSymlinksFor(tool string, cenv containerenv.Config) string {
	return cenv.ToolDataSymlinks(tool)
}

// ContainerEnvVars returns the base set of tool-specific container environment
// variables (AW_CONTAINER_CONFIG_DIR, AW_DATA_SYMLINKS). Callers add their own
// context-specific variables on top (e.g. HOST_WORKSPACE, SSH_AUTH_SOCK).
func ContainerEnvVars(baseEnvVars map[string]string, tool string) map[string]string {
	return ContainerEnvVarsFor(baseEnvVars, tool, containerenv.Default())
}

func ContainerEnvVarsFor(baseEnvVars map[string]string, tool string, cenv containerenv.Config) map[string]string {
	envVars := make(map[string]string, len(baseEnvVars)+4)
	for k, v := range baseEnvVars {
		envVars[k] = v
	}

	if dir := ContainerDirFor(tool, cenv); dir != "" {
		envVars["AW_CONTAINER_CONFIG_DIR"] = dir
	}
	if symlinks := DataSymlinksFor(tool, cenv); symlinks != "" {
		envVars["AW_DATA_SYMLINKS"] = symlinks
	}

	return envVars
}
