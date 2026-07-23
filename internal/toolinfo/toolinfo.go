package toolinfo

import (
	"maps"
	"os"
	"path/filepath"
	"slices"

	"github.com/konono/aw/v4/internal/containerenv"
)

// ToolSpec holds static per-tool metadata.
type ToolSpec struct {
	Binary            string
	DisplayName       string
	InstallScript     string // shell command to install the tool in a container
	DevboxPkg         string // deprecated: used with package_manager: devbox
	HomeEnvVar        string
	DefaultHomeSubdir string
	InstallHint       string
}

var tools = map[string]ToolSpec{
	"base": {
		DisplayName: "Base",
	},
	"claude": {
		Binary:            "claude",
		DisplayName:       "Claude Code",
		InstallScript:     "curl -fsSL https://claude.ai/install.sh | bash; _rc=$?; if [ $_rc -ne 0 ]; then if [ ! -f $HOME/.local/bin/claude ] && ! command -v claude >/dev/null 2>&1; then exit $_rc; fi; fi; { [ -f $HOME/.local/bin/claude ] || { mkdir -p $HOME/.local/bin && ln -sf $(which claude) $HOME/.local/bin/claude; }; }; sudo ln -sf $HOME/.local/bin/claude /claude 2>/dev/null || true",
		DevboxPkg:         "claude-code",
		HomeEnvVar:        "CLAUDE_HOME",
		DefaultHomeSubdir: ".claude",
		InstallHint:       "Install Claude Code: curl -fsSL https://claude.ai/install.sh | bash",
	},
	"codex": {
		Binary:            "codex",
		DisplayName:       "Codex",
		InstallScript:     "curl -fsSL https://github.com/openai/codex/releases/latest/download/install.sh | CODEX_NON_INTERACTIVE=true sh && cp -L $HOME/.local/bin/codex $HOME/.local/bin/codex.tmp && mv $HOME/.local/bin/codex.tmp $HOME/.local/bin/codex",
		DevboxPkg:         "codex",
		HomeEnvVar:        "CODEX_HOME",
		DefaultHomeSubdir: ".codex",
		InstallHint:       "Install Codex CLI: curl -fsSL https://github.com/openai/codex/releases/latest/download/install.sh | sh",
	},
	"opencode": {
		Binary:            "opencode",
		DisplayName:       "OpenCode",
		InstallScript:     "curl -fsSL https://opencode.ai/install | bash -s -- --no-modify-path && mkdir -p $HOME/.local/bin && ln -sf $HOME/.opencode/bin/opencode $HOME/.local/bin/opencode",
		DevboxPkg:         "opencode",
		HomeEnvVar:        "OPENCODE_CONFIG_DIR",
		DefaultHomeSubdir: filepath.Join(".config", "opencode"),
		InstallHint:       "Install via: curl -fsSL https://opencode.ai/install | bash",
	},
	"cursor": {
		Binary:            "agent",
		DisplayName:       "Cursor",
		InstallScript:     "curl -fsSL https://cursor.com/install | bash; _rc=$?; if [ $_rc -ne 0 ]; then if [ ! -f $HOME/.local/bin/agent ]; then exit $_rc; fi; fi; { [ -f $HOME/.local/bin/agent ] || { mkdir -p $HOME/.local/bin && ln -sf $(which agent) $HOME/.local/bin/agent; }; }",
		HomeEnvVar:        "CURSOR_CONFIG_DIR",
		DefaultHomeSubdir: ".cursor",
		InstallHint:       "Install Cursor CLI: curl https://cursor.com/install -fsS | bash",
	},
}

// Names returns the sorted list of all registered tool names.
func Names() []string {
	return slices.Sorted(maps.Keys(tools))
}

// Lookup returns the ToolSpec for the given tool name.
func Lookup(tool string) (ToolSpec, bool) {
	spec, ok := tools[tool]
	return spec, ok
}

func InstallScript(tool string) string {
	if spec, ok := Lookup(tool); ok {
		return spec.InstallScript
	}
	return ""
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

func ContainerDirFor(tool string, cenv containerenv.Config) string {
	return cenv.ToolDir(tool)
}

func DataSymlinksFor(tool string, cenv containerenv.Config) string {
	return cenv.ToolDataSymlinks(tool)
}

// ImageTool returns the tool name used for official image resolution.
// AI tools return their own name; shell (empty tool) maps to "base".
func ImageTool(tool string) string {
	if tool == "" {
		return "base"
	}
	return tool
}

// GhCLIVersion is the pinned version of the GitHub CLI installed in all images.
const GhCLIVersion = "2.96.0"

// MiseVersion is the pinned version of mise installed in all images.
const MiseVersion = "2026.7.11"

// ContainerEnvVarsFor returns tool-specific container environment variables
// (AW_CONTAINER_CONFIG_DIR, AW_DATA_SYMLINKS). Callers add context-specific
// variables on top (e.g. HOST_WORKSPACE, SSH_AUTH_SOCK).
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
