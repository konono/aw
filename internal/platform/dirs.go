package platform

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// ConfigDir returns the aw configuration directory.
// Unix: ~/.config/aw
// Windows: %APPDATA%\aw
func ConfigDir() string {
	if runtime.GOOS == "windows" {
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, "aw")
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".config", "aw")
}

// StateDir returns the aw state directory for data that persists across sessions.
// Unix: $XDG_STATE_HOME/aw or ~/.local/state/aw
// Windows: %LOCALAPPDATA%\aw
func StateDir() string {
	if runtime.GOOS == "windows" {
		if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
			return filepath.Join(localAppData, "aw")
		}
	}
	if dir := os.Getenv("XDG_STATE_HOME"); dir != "" {
		return filepath.Join(dir, "aw")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".local", "state", "aw")
}

// TempSocketPath returns a temporary socket path for SSH agent forwarding.
// On Unix: /tmp/aw-ssh-agent-{name}.sock
// On Windows: %TEMP%\aw-ssh-agent-{name}.sock
func TempSocketPath(containerName string) string {
	return filepath.Join(os.TempDir(), fmt.Sprintf("aw-ssh-agent-%s.sock", containerName))
}
