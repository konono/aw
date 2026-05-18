package toolinfo

import (
	"os"
	"path/filepath"
)

func DevboxPkg(tool string) string {
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

func HomePath(tool, homeDir string) string {
	switch tool {
	case "claude":
		if v := os.Getenv("CLAUDE_HOME"); v != "" {
			return v
		}
		return filepath.Join(homeDir, ".claude")
	case "codex":
		if v := os.Getenv("CODEX_HOME"); v != "" {
			return v
		}
		return filepath.Join(homeDir, ".codex")
	case "opencode":
		if v := os.Getenv("OPENCODE_CONFIG_DIR"); v != "" {
			return v
		}
		return filepath.Join(homeDir, ".config", "opencode")
	default:
		return ""
	}
}

func ContainerDir(tool string) string {
	switch tool {
	case "claude":
		return "/home/agent/.claude"
	case "codex":
		return "/home/agent/.codex"
	case "opencode":
		return "/home/agent/.config/opencode"
	default:
		return ""
	}
}

func DataSymlinks(tool string) string {
	switch tool {
	case "opencode":
		return "/home/agent/.local/share/opencode:/home/agent/.config/opencode/data"
	default:
		return ""
	}
}
