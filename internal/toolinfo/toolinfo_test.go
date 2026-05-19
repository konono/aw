package toolinfo

import (
	"testing"
)

func TestLookup_KnownTools(t *testing.T) {
	for _, tool := range []string{"claude", "codex", "opencode"} {
		spec, ok := Lookup(tool)
		if !ok {
			t.Errorf("Lookup(%q) returned false", tool)
			continue
		}
		if spec.Binary == "" {
			t.Errorf("%s: Binary is empty", tool)
		}
		if spec.DisplayName == "" {
			t.Errorf("%s: DisplayName is empty", tool)
		}
		if spec.DevboxPkg == "" {
			t.Errorf("%s: DevboxPkg is empty", tool)
		}
		if spec.ContainerDir == "" {
			t.Errorf("%s: ContainerDir is empty", tool)
		}
		if spec.InstallHint == "" {
			t.Errorf("%s: InstallHint is empty", tool)
		}
	}
}

func TestLookup_UnknownTool(t *testing.T) {
	_, ok := Lookup("nonexistent")
	if ok {
		t.Error("Lookup(nonexistent) should return false")
	}
}

func TestDevboxPkg(t *testing.T) {
	tests := []struct {
		tool string
		want string
	}{
		{"claude", "claude-code"},
		{"codex", "codex"},
		{"opencode", "opencode"},
		{"unknown", ""},
	}
	for _, tt := range tests {
		if got := DevboxPkg(tt.tool); got != tt.want {
			t.Errorf("DevboxPkg(%q) = %q, want %q", tt.tool, got, tt.want)
		}
	}
}

func TestHomePath(t *testing.T) {
	home := "/home/testuser"
	tests := []struct {
		tool string
		want string
	}{
		{"claude", "/home/testuser/.claude"},
		{"codex", "/home/testuser/.codex"},
		{"opencode", "/home/testuser/.config/opencode"},
		{"unknown", ""},
	}
	for _, tt := range tests {
		if got := HomePath(tt.tool, home); got != tt.want {
			t.Errorf("HomePath(%q, %q) = %q, want %q", tt.tool, home, got, tt.want)
		}
	}
}

func TestHomePath_EnvOverride(t *testing.T) {
	t.Setenv("CLAUDE_HOME", "/custom/claude")
	got := HomePath("claude", "/home/testuser")
	if got != "/custom/claude" {
		t.Errorf("HomePath with CLAUDE_HOME override = %q, want %q", got, "/custom/claude")
	}
}

func TestContainerDir(t *testing.T) {
	tests := []struct {
		tool string
		want string
	}{
		{"claude", "/home/agent/.claude"},
		{"codex", "/home/agent/.codex"},
		{"opencode", "/home/agent/.config/opencode"},
		{"unknown", ""},
	}
	for _, tt := range tests {
		if got := ContainerDir(tt.tool); got != tt.want {
			t.Errorf("ContainerDir(%q) = %q, want %q", tt.tool, got, tt.want)
		}
	}
}

func TestDataSymlinks(t *testing.T) {
	if got := DataSymlinks("opencode"); got == "" {
		t.Error("DataSymlinks(opencode) should not be empty")
	}
	if got := DataSymlinks("claude"); got != "" {
		t.Errorf("DataSymlinks(claude) = %q, want empty", got)
	}
	if got := DataSymlinks("unknown"); got != "" {
		t.Errorf("DataSymlinks(unknown) = %q, want empty", got)
	}
}
