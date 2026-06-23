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

// CacheDir returns the aw cache directory.
// Unix: $XDG_CACHE_HOME/aw or ~/.cache/aw
// Windows: %LOCALAPPDATA%\aw\cache
func CacheDir() string {
	if runtime.GOOS == "windows" {
		if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
			return filepath.Join(localAppData, "aw", "cache")
		}
	}
	if dir := os.Getenv("XDG_CACHE_HOME"); dir != "" {
		return filepath.Join(dir, "aw")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".cache", "aw")
}

// TempSocketPath returns a temporary socket path for SSH agent forwarding.
// On Unix, /tmp is used to keep paths short (Unix sockets have a ~104 char limit;
// macOS os.TempDir() returns /var/folders/... which is too long).
// On Windows, os.TempDir() (%TEMP%) is used since /tmp does not exist.
func TempSocketPath(containerName string) string {
	dir := "/tmp"
	if runtime.GOOS == "windows" {
		dir = os.TempDir()
	}
	return filepath.Join(dir, fmt.Sprintf("aw-ssh-agent-%s.sock", containerName))
}
