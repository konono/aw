package toolinfo

import (
	"testing"

	"github.com/konono/aw/internal/containerenv"
)

func TestLookup_KnownTools(t *testing.T) {
	cenv := containerenv.Default()
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
		if spec.InstallScript == "" {
			t.Errorf("%s: InstallScript is empty", tool)
		}
		if ContainerDirFor(tool, cenv) == "" {
			t.Errorf("%s: ContainerDirFor is empty", tool)
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

func TestInstallScript(t *testing.T) {
	for _, tool := range []string{"claude", "codex", "opencode"} {
		if got := InstallScript(tool); got == "" {
			t.Errorf("InstallScript(%q) should not be empty", tool)
		}
	}
	if got := InstallScript("unknown"); got != "" {
		t.Errorf("InstallScript(unknown) = %q, want empty", got)
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

func TestContainerDirFor(t *testing.T) {
	cenv := containerenv.Default()
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
		if got := ContainerDirFor(tt.tool, cenv); got != tt.want {
			t.Errorf("ContainerDirFor(%q) = %q, want %q", tt.tool, got, tt.want)
		}
	}
}

func TestDataSymlinksFor(t *testing.T) {
	cenv := containerenv.Default()
	if got := DataSymlinksFor("opencode", cenv); got == "" {
		t.Error("DataSymlinksFor(opencode) should not be empty")
	}
	if got := DataSymlinksFor("claude", cenv); got != "" {
		t.Errorf("DataSymlinksFor(claude) = %q, want empty", got)
	}
	if got := DataSymlinksFor("unknown", cenv); got != "" {
		t.Errorf("DataSymlinksFor(unknown) = %q, want empty", got)
	}
}
