package containerenv

import (
	"fmt"
	"path/filepath"
)

const (
	DefaultUser      = "agent"
	DefaultWorkspace = "/workspace"
)

type Config struct {
	User      string
	Home      string
	Workspace string
}

func Default() Config {
	return FromUser("")
}

func FromUser(user string) Config {
	if user == "" {
		user = DefaultUser
	}
	return Config{
		User:      user,
		Home:      filepath.Join("/home", user),
		Workspace: DefaultWorkspace,
	}
}

func (c Config) AwEnvFile() string    { return filepath.Join(c.Home, ".aw_env.sh") }
func (c Config) BashRC() string       { return filepath.Join(c.Home, ".bashrc") }
func (c Config) BashProfile() string  { return filepath.Join(c.Home, ".bash_profile") }
func (c Config) NixProfile() string   { return filepath.Join(c.Home, ".nix-profile") }
func (c Config) NixProfileBin() string { return filepath.Join(c.NixProfile(), "bin") }
func (c Config) NixProfileSh() string {
	return filepath.Join(c.NixProfile(), "etc", "profile.d", "nix.sh")
}
func (c Config) NixStateProfiles() string {
	return filepath.Join(c.Home, ".local", "state", "nix", "profiles", "profile")
}
func (c Config) LocalBin() string     { return filepath.Join(c.Home, ".local", "bin") }
func (c Config) GitConfig() string    { return filepath.Join(c.Home, ".gitconfig") }
func (c Config) GHConfig() string     { return filepath.Join(c.Home, ".config", "gh") }
func (c Config) SSHHostDir() string   { return filepath.Join(c.Home, ".ssh-host") }
func (c Config) SSHDir() string       { return filepath.Join(c.Home, ".ssh") }
func (c Config) MiseDataDir() string  { return filepath.Join(c.Home, ".local", "share", "mise") }
func (c Config) MiseConfigDir() string { return filepath.Join(c.Home, ".config", "mise") }
func (c Config) MiseShims() string    { return filepath.Join(c.MiseDataDir(), "shims") }
func (c Config) ClaudeJSON() string   { return filepath.Join(c.Home, ".claude.json") }

func (c Config) ToolDir(tool string) string {
	switch tool {
	case "claude":
		return filepath.Join(c.Home, ".claude")
	case "codex":
		return filepath.Join(c.Home, ".codex")
	case "opencode":
		return filepath.Join(c.Home, ".config", "opencode")
	default:
		return ""
	}
}

func (c Config) ToolDataSymlinks(tool string) string {
	switch tool {
	case "opencode":
		return fmt.Sprintf("%s:%s",
			filepath.Join(c.Home, ".local", "share", "opencode"),
			filepath.Join(c.Home, ".config", "opencode", "data"))
	default:
		return ""
	}
}
