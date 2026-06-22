package containerenv

import (
	"fmt"
	"path"
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
		Home:      path.Join("/home", user),
		Workspace: DefaultWorkspace,
	}
}

func (c Config) AwEnvFile() string    { return path.Join(c.Home, ".aw_env.sh") }
func (c Config) BashRC() string       { return path.Join(c.Home, ".bashrc") }
func (c Config) BashProfile() string  { return path.Join(c.Home, ".bash_profile") }
func (c Config) LocalBin() string     { return path.Join(c.Home, ".local", "bin") }
func (c Config) NixProfile() string   { return path.Join(c.Home, ".nix-profile") }
func (c Config) NixProfileBin() string { return path.Join(c.NixProfile(), "bin") }
func (c Config) NixProfileSh() string {
	return path.Join(c.NixProfile(), "etc", "profile.d", "nix.sh")
}
func (c Config) NixStateProfiles() string {
	return path.Join(c.Home, ".local", "state", "nix", "profiles", "profile")
}
func (c Config) GitConfig() string    { return path.Join(c.Home, ".gitconfig") }
func (c Config) GHConfig() string     { return path.Join(c.Home, ".config", "gh") }
func (c Config) SSHHostDir() string   { return path.Join(c.Home, ".ssh-host") }
func (c Config) SSHDir() string       { return path.Join(c.Home, ".ssh") }
func (c Config) MiseDataDir() string  { return path.Join(c.Home, ".local", "share", "mise") }
func (c Config) MiseConfigDir() string { return path.Join(c.Home, ".config", "mise") }
func (c Config) MiseShims() string    { return path.Join(c.MiseDataDir(), "shims") }
func (c Config) ClaudeJSON() string   { return path.Join(c.Home, ".claude.json") }

func (c Config) ToolDir(tool string) string {
	switch tool {
	case "claude":
		return path.Join(c.Home, ".claude")
	case "codex":
		return path.Join(c.Home, ".codex")
	case "opencode":
		return path.Join(c.Home, ".config", "opencode")
	case "cursor":
		return path.Join(c.Home, ".cursor")
	default:
		return ""
	}
}

func (c Config) ToolDataSymlinks(tool string) string {
	switch tool {
	case "opencode":
		return fmt.Sprintf("%s:%s",
			path.Join(c.Home, ".local", "share", "opencode"),
			path.Join(c.Home, ".config", "opencode", "data"))
	case "cursor":
		return fmt.Sprintf("%s:%s",
			path.Join(c.Home, ".config", "cursor"),
			path.Join(c.Home, ".cursor"))
	default:
		return ""
	}
}
