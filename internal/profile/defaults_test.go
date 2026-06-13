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
	// The embedded default config must parse, have defaults applied, and
	// pass ValidateConfig. mustParseBuiltinConfig() panics on failure, so
	// if we reach this point the embedded YAML is structurally valid.
	// Re-validate the runtime-ready form (with defaults applied) to confirm
	// ValidateConfig accepts the result.
	applied := ApplyDefaults(builtinConfig)
	if err := ValidateConfig(&applied); err != nil {
		t.Fatalf("builtin config with defaults applied fails validation: %v", err)
	}
}

func TestBuiltinConfig_ExpectedProfilesExist(t *testing.T) {
	expected := []string{"claude", "shell", "codex", "opencode"}
	for _, name := range expected {
		if _, ok := builtinConfig.Profiles[name]; !ok {
			t.Errorf("expected builtin profile %q to exist", name)
		}
	}
}

func TestBuiltinConfig_AllProfilesAreContainer(t *testing.T) {
	applied := ApplyDefaults(builtinConfig)
	for name, p := range applied.Profiles {
		if p.Environment != EnvironmentContainer {
			t.Errorf("profile %q: environment = %q, want %q", name, p.Environment, EnvironmentContainer)
		}
	}
}

func TestBuiltinConfig_AllProfilesPassValidate(t *testing.T) {
	applied := ApplyDefaults(builtinConfig)
	for name, p := range applied.Profiles {
		if err := Validate(p); err != nil {
			t.Errorf("profile %q fails Validate: %v", name, err)
		}
	}
}
