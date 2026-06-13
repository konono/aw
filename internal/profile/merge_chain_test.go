package profile

import (
	"testing"
)

// These tests verify the full merge chain (builtin → user → project)
// produces correct, valid profiles. Unlike merge_test.go which tests
// individual functions, these simulate the real config loading flow.

func TestMergeChain_ProjectProfileInheritsBuiltinDefaults(t *testing.T) {
	project := Config{
		Profiles: map[string]Profile{
			"my-tool": {Launch: LaunchClaude},
		},
	}

	merged := MergeConfig(builtinConfig, project)
	applied := ApplyDefaults(merged)

	p := applied.Profiles["my-tool"]
	if p.Environment != EnvironmentContainer {
		t.Errorf("Environment = %q, want %q (should inherit from builtin defaults)", p.Environment, EnvironmentContainer)
	}
	if p.Launch != LaunchClaude {
		t.Errorf("Launch = %q, want %q", p.Launch, LaunchClaude)
	}
	if err := Validate(p); err != nil {
		t.Errorf("profile should be valid after merge chain, got: %v", err)
	}
}

func TestMergeChain_ProjectProfileInheritsOS(t *testing.T) {
	project := Config{
		Profiles: map[string]Profile{
			"my-tool": {Launch: LaunchClaude},
		},
	}

	merged := MergeConfig(builtinConfig, project)
	applied := ApplyDefaults(merged)

	p := applied.Profiles["my-tool"]
	if p.OS == "" {
		t.Error("OS should be inherited from builtin defaults, got empty")
	}
}

func TestMergeChain_ProjectProfileOverridesDefaults(t *testing.T) {
	project := Config{
		Profiles: map[string]Profile{
			"host-shell": {
				Environment: EnvironmentHost,
				Launch:      LaunchShell,
			},
		},
	}

	merged := MergeConfig(builtinConfig, project)
	applied := ApplyDefaults(merged)

	p := applied.Profiles["host-shell"]
	if p.Environment != EnvironmentHost {
		t.Errorf("Environment = %q, want %q (explicit override should win)", p.Environment, EnvironmentHost)
	}
}

func TestMergeChain_ThreeLayerMerge(t *testing.T) {
	user := Config{
		Defaults: ProfileDefaultsFromProfile(Profile{
			MountSSH: boolPtr(true),
		}),
		Profiles: map[string]Profile{},
	}
	project := Config{
		Profiles: map[string]Profile{
			"dev": {Launch: LaunchClaude, GhToken: boolPtr(true)},
		},
	}

	merged := MergeConfig(builtinConfig, user)
	merged = MergeConfig(merged, project)
	applied := ApplyDefaults(merged)

	p := applied.Profiles["dev"]
	if p.Environment != EnvironmentContainer {
		t.Errorf("Environment = %q, want %q (from builtin)", p.Environment, EnvironmentContainer)
	}
	if !p.EffectiveMountSSH() {
		t.Error("mount_ssh should be true (from user defaults)")
	}
	if !p.EffectiveGhToken() {
		t.Error("gh_token should be true (from project profile)")
	}
	if err := Validate(p); err != nil {
		t.Errorf("profile should be valid after 3-layer merge, got: %v", err)
	}
}

func TestMergeChain_BuiltinProfilesSurviveProjectMerge(t *testing.T) {
	project := Config{
		Profiles: map[string]Profile{
			"extra": {Launch: LaunchShell},
		},
	}

	merged := MergeConfig(builtinConfig, project)
	applied := ApplyDefaults(merged)

	for _, name := range []string{"claude", "shell"} {
		p, ok := applied.Profiles[name]
		if !ok {
			t.Errorf("builtin profile %q should be preserved", name)
			continue
		}
		if p.Environment != EnvironmentContainer {
			t.Errorf("%s.Environment = %q, want %q", name, p.Environment, EnvironmentContainer)
		}
		if err := Validate(p); err != nil {
			t.Errorf("builtin profile %q should be valid after merge, got: %v", name, err)
		}
	}
}

func TestMergeChain_AllProfilesValidAfterApplyDefaults(t *testing.T) {
	project := Config{
		Profiles: map[string]Profile{
			"a": {Launch: LaunchClaude},
			"b": {Launch: LaunchShell, OS: OSUBI9},
			"c": {Launch: LaunchCodex, GhToken: boolPtr(true)},
		},
	}

	merged := MergeConfig(builtinConfig, project)
	applied := ApplyDefaults(merged)

	if err := ValidateConfig(&applied); err != nil {
		t.Errorf("all profiles should be valid after merge chain, got: %v", err)
	}
}

func TestMergeChain_UserDefaultsOverrideBuiltinDefaults(t *testing.T) {
	user := Config{
		Defaults: ProfileDefaultsFromProfile(Profile{
			ContainerRuntime: ContainerRuntimeDocker,
		}),
		Profiles: map[string]Profile{},
	}
	project := Config{
		Profiles: map[string]Profile{
			"dev": {Launch: LaunchClaude},
		},
	}

	merged := MergeConfig(builtinConfig, user)
	merged = MergeConfig(merged, project)
	applied := ApplyDefaults(merged)

	p := applied.Profiles["dev"]
	if p.ContainerRuntime != ContainerRuntimeDocker {
		t.Errorf("ContainerRuntime = %q, want %q (user default should override builtin)", p.ContainerRuntime, ContainerRuntimeDocker)
	}
}

func TestMergeChain_ProjectDefaultsOverrideUserDefaults(t *testing.T) {
	user := Config{
		Defaults: ProfileDefaultsFromProfile(Profile{
			OS: OSUBI9,
		}),
		Profiles: map[string]Profile{},
	}
	project := Config{
		Defaults: ProfileDefaultsFromProfile(Profile{
			OS: OSUBI10,
		}),
		Profiles: map[string]Profile{
			"dev": {Launch: LaunchClaude},
		},
	}

	merged := MergeConfig(builtinConfig, user)
	merged = MergeConfig(merged, project)
	applied := ApplyDefaults(merged)

	p := applied.Profiles["dev"]
	if p.OS != OSUBI10 {
		t.Errorf("OS = %q, want %q (project default should override user)", p.OS, OSUBI10)
	}
}
