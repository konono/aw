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
		InstallScript:     "curl -fsSL https://claude.ai/install.sh -o /tmp/aw-install-claude.sh && bash /tmp/aw-install-claude.sh && rm -f /tmp/aw-install-claude.sh && { [ -f $HOME/.local/bin/claude ] || { mkdir -p $HOME/.local/bin && ln -sf $(command -v claude) $HOME/.local/bin/claude; }; }; sudo ln -sf $HOME/.local/bin/claude /claude 2>/dev/null || true",
		HomeEnvVar:        "CLAUDE_HOME",
		DefaultHomeSubdir: ".claude",
		InstallHint:       "Install Claude Code: curl -fsSL https://claude.ai/install.sh | bash",
	},
	"codex": {
		Binary:            "codex",
		DisplayName:       "Codex",
		InstallScript:     "curl -fsSL https://github.com/openai/codex/releases/latest/download/install.sh -o /tmp/aw-install-codex.sh && CODEX_NON_INTERACTIVE=true sh /tmp/aw-install-codex.sh && rm -f /tmp/aw-install-codex.sh && cp -L $HOME/.local/bin/codex $HOME/.local/bin/codex.tmp && mv $HOME/.local/bin/codex.tmp $HOME/.local/bin/codex",
		HomeEnvVar:        "CODEX_HOME",
		DefaultHomeSubdir: ".codex",
		InstallHint:       "Install Codex CLI: curl -fsSL https://github.com/openai/codex/releases/latest/download/install.sh | sh",
	},
	"opencode": {
		Binary:            "opencode",
		DisplayName:       "OpenCode",
		InstallScript:     "curl -fsSL https://opencode.ai/install -o /tmp/aw-install-opencode.sh && bash /tmp/aw-install-opencode.sh --no-modify-path && rm -f /tmp/aw-install-opencode.sh && mkdir -p $HOME/.local/bin && ln -sf $HOME/.opencode/bin/opencode $HOME/.local/bin/opencode",
		HomeEnvVar:        "OPENCODE_CONFIG_DIR",
		DefaultHomeSubdir: filepath.Join(".config", "opencode"),
		InstallHint:       "Install via: curl -fsSL https://opencode.ai/install | bash",
	},
	"cursor": {
		Binary:            "agent",
		DisplayName:       "Cursor",
		InstallScript:     "curl -fsSL https://cursor.com/install -o /tmp/aw-install-cursor.sh && bash /tmp/aw-install-cursor.sh && rm -f /tmp/aw-install-cursor.sh && { [ -f $HOME/.local/bin/agent ] || { mkdir -p $HOME/.local/bin && ln -sf $(command -v agent) $HOME/.local/bin/agent; }; }",
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

// InstallScript returns the command that installs the tool inside a
// container, with a check that the binary actually exists appended.
//
// The check is not redundant with the installers' own exit codes. An
// installer can report success and still leave nothing usable behind: a
// release archive missing the architecture, a partial download, or a symlink
// pointing at a path the installer never wrote. Without the check the image
// builds green and the tool is only missed at launch, which is how a 502 from
// claude.ai produced an image with no claude in it.
func InstallScript(tool string) string {
	spec, ok := Lookup(tool)
	if !ok || spec.InstallScript == "" {
		return ""
	}
	return spec.InstallScript + installedCheck(spec.Binary)
}

// installedCheck fails the build when binary is neither on PATH nor in the
// per-user bin directory. -x rather than -f so a dangling symlink, which
// ln -sf creates happily when its target is missing, counts as absent.
//
// It avoids double quotes: the script is passed to the image build as an ARG
// and run as su -c "$AW_TOOL_INSTALL_SCRIPT".
func installedCheck(binary string) string {
	if binary == "" {
		return ""
	}
	return "; if [ ! -x $HOME/.local/bin/" + binary + " ] && ! command -v " + binary +
		" >/dev/null 2>&1; then echo 'aw: " + binary +
		" was not installed (the installer did not leave a usable binary)' >&2; exit 1; fi"
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
const GhCLIVersion = "2.102.0"

// MiseVersion is the pinned version of mise installed in all images.
const MiseVersion = "2026.10.8"

// ZellijVersion is the pinned version of zellij installed when mount_zellij is enabled.
const ZellijVersion = "0.44.3"

// PanecomVersion is the pinned version of panecom installed when mount_zellij is enabled.
const PanecomVersion = "0.3.0"

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
