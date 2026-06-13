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
		if spec.InstallScript == "" {
			t.Errorf("%s: InstallScript is empty", tool)
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

func TestLookup_AllRequiredFieldsNonEmpty(t *testing.T) {
	knownTools := []string{"claude", "codex", "opencode"}
	for _, tool := range knownTools {
		spec, ok := Lookup(tool)
		if !ok {
			t.Fatalf("Lookup(%q) returned false", tool)
		}
		fields := map[string]string{
			"Binary":            spec.Binary,
			"DisplayName":       spec.DisplayName,
			"InstallScript":     spec.InstallScript,
			"HomeEnvVar":        spec.HomeEnvVar,
			"DefaultHomeSubdir": spec.DefaultHomeSubdir,
			"ContainerDir":      spec.ContainerDir,
			"InstallHint":       spec.InstallHint,
		}
		for name, val := range fields {
			if val == "" {
				t.Errorf("%s: required field %s is empty", tool, name)
			}
		}
	}
}

func TestDevboxPkg_AllToolsNonEmpty(t *testing.T) {
	for _, tool := range []string{"claude", "codex", "opencode"} {
		if got := DevboxPkg(tool); got == "" {
			t.Errorf("DevboxPkg(%q) should not be empty (devbox mode compatibility)", tool)
		}
	}
}

func TestDevboxPkg_UnknownTool(t *testing.T) {
	if got := DevboxPkg("unknown"); got != "" {
		t.Errorf("DevboxPkg(unknown) = %q, want empty", got)
	}
}

func TestContainerEnvVars_SetsConfigDir(t *testing.T) {
	for _, tool := range []string{"claude", "codex", "opencode"} {
		envVars := ContainerEnvVars(nil, tool)
		if dir, ok := envVars["AW_CONTAINER_CONFIG_DIR"]; !ok || dir == "" {
			t.Errorf("ContainerEnvVars(nil, %q): AW_CONTAINER_CONFIG_DIR not set or empty", tool)
		}
	}
}

func TestContainerEnvVars_PreservesBaseEnvVars(t *testing.T) {
	base := map[string]string{
		"MY_KEY":    "my_value",
		"OTHER_KEY": "other_value",
	}
	envVars := ContainerEnvVars(base, "claude")

	// Base env vars must be preserved.
	for k, want := range base {
		got, ok := envVars[k]
		if !ok {
			t.Errorf("base env var %q missing from result", k)
		} else if got != want {
			t.Errorf("base env var %q = %q, want %q", k, got, want)
		}
	}

	// Tool-specific vars must also be present.
	if _, ok := envVars["AW_CONTAINER_CONFIG_DIR"]; !ok {
		t.Error("AW_CONTAINER_CONFIG_DIR missing after merge with base env vars")
	}
}

func TestContainerEnvVars_DoesNotOverwriteBase(t *testing.T) {
	// If a base env var uses the same key as a tool var, the tool var wins
	// (current behavior). This test documents that the base map is not mutated.
	base := map[string]string{
		"KEEP_ME": "original",
	}
	_ = ContainerEnvVars(base, "claude")

	if base["KEEP_ME"] != "original" {
		t.Errorf("base map was mutated: KEEP_ME = %q, want %q", base["KEEP_ME"], "original")
	}
}
