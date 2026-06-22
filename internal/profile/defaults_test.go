package profile

import (
	"bytes"
	"testing"
)

func TestDefaultConfigYAML_ExplicitlyDisablesMountSSH(t *testing.T) {
	if !bytes.Contains(DefaultConfigYAML(), []byte("mount_ssh: false")) {
		t.Fatal("DefaultConfigYAML() should explicitly include mount_ssh: false")
	}
}

func TestBuiltinConfig_ParsesAndValidates(t *testing.T) {
	applied := ApplyDefaults(builtinConfig)
	if err := ValidateConfig(&applied); err != nil {
		t.Fatalf("builtin config should be valid: %v", err)
	}
}

func TestBuiltinConfig_ExpectedProfilesExist(t *testing.T) {
	for _, name := range []string{"claude", "shell", "codex", "opencode", "cursor"} {
		if _, ok := builtinConfig.Profiles[name]; !ok {
			t.Errorf("builtin config missing expected profile %q", name)
		}
	}
}

func TestBuiltinConfig_AllProfilesAreContainer(t *testing.T) {
	applied := ApplyDefaults(builtinConfig)
	for name, p := range applied.Profiles {
		if p.Environment != EnvironmentContainer {
			t.Errorf("%s.Environment = %q, want %q", name, p.Environment, EnvironmentContainer)
		}
	}
}
