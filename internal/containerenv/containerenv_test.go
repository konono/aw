package containerenv

import (
	"testing"
)

func TestDefault(t *testing.T) {
	c := Default()
	if c.User != "agent" {
		t.Errorf("User = %q, want %q", c.User, "agent")
	}
	if c.Home != "/home/agent" {
		t.Errorf("Home = %q, want %q", c.Home, "/home/agent")
	}
	if c.Workspace != "/workspace" {
		t.Errorf("Workspace = %q, want %q", c.Workspace, "/workspace")
	}
}

func TestFromUser(t *testing.T) {
	tests := []struct {
		user     string
		wantUser string
		wantHome string
	}{
		{"", "agent", "/home/agent"},
		{"agent", "agent", "/home/agent"},
		{"dev", "dev", "/home/dev"},
		{"myuser", "myuser", "/home/myuser"},
	}
	for _, tt := range tests {
		c := FromUser(tt.user)
		if c.User != tt.wantUser {
			t.Errorf("FromUser(%q).User = %q, want %q", tt.user, c.User, tt.wantUser)
		}
		if c.Home != tt.wantHome {
			t.Errorf("FromUser(%q).Home = %q, want %q", tt.user, c.Home, tt.wantHome)
		}
		if c.Workspace != "/workspace" {
			t.Errorf("FromUser(%q).Workspace = %q, want %q", tt.user, c.Workspace, "/workspace")
		}
	}
}

func TestPathDerivation(t *testing.T) {
	c := FromUser("dev")

	tests := []struct {
		name string
		got  string
		want string
	}{
		{"AwEnvFile", c.AwEnvFile(), "/home/dev/.aw_env.sh"},
		{"BashRC", c.BashRC(), "/home/dev/.bashrc"},
		{"BashProfile", c.BashProfile(), "/home/dev/.bash_profile"},
		{"LocalBin", c.LocalBin(), "/home/dev/.local/bin"},
		{"GitConfig", c.GitConfig(), "/home/dev/.gitconfig"},
		{"GHConfig", c.GHConfig(), "/home/dev/.config/gh"},
		{"SSHHostDir", c.SSHHostDir(), "/home/dev/.ssh-host"},
		{"SSHDir", c.SSHDir(), "/home/dev/.ssh"},
		{"MiseDataDir", c.MiseDataDir(), "/home/dev/.local/share/mise"},
		{"MiseConfigDir", c.MiseConfigDir(), "/home/dev/.config/mise"},
		{"MiseShims", c.MiseShims(), "/home/dev/.local/share/mise/shims"},
		{"ClaudeJSON", c.ClaudeJSON(), "/home/dev/.claude.json"},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("%s = %q, want %q", tt.name, tt.got, tt.want)
		}
	}
}

func TestToolDir(t *testing.T) {
	c := FromUser("dev")

	tests := []struct {
		tool string
		want string
	}{
		{"claude", "/home/dev/.claude"},
		{"codex", "/home/dev/.codex"},
		{"opencode", "/home/dev/.config/opencode"},
		{"cursor", "/home/dev/.cursor"},
		{"unknown", ""},
	}
	for _, tt := range tests {
		got := c.ToolDir(tt.tool)
		if got != tt.want {
			t.Errorf("ToolDir(%q) = %q, want %q", tt.tool, got, tt.want)
		}
	}
}

func TestToolDataSymlinks(t *testing.T) {
	c := FromUser("dev")

	got := c.ToolDataSymlinks("opencode")
	want := "/home/dev/.local/share/opencode:/home/dev/.config/opencode/data"
	if got != want {
		t.Errorf("ToolDataSymlinks(opencode) = %q, want %q", got, want)
	}

	got = c.ToolDataSymlinks("cursor")
	wantCursor := "/home/dev/.config/cursor:/home/dev/.cursor"
	if got != wantCursor {
		t.Errorf("ToolDataSymlinks(cursor) = %q, want %q", got, wantCursor)
	}

	got = c.ToolDataSymlinks("claude")
	if got != "" {
		t.Errorf("ToolDataSymlinks(claude) = %q, want empty", got)
	}
}
