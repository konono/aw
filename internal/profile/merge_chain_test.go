package profile

import (
	"testing"
)

// These tests verify the full merge chain (builtin → user → project)
// produces correct, valid profiles — the behavior users depend on.

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
		t.Errorf("Environment = %q, want %q", p.Environment, EnvironmentContainer)
	}
	if p.OS == "" {
		t.Error("OS should be inherited from builtin defaults")
	}
	if err := Validate(p); err != nil {
		t.Errorf("profile should be valid after merge: %v", err)
	}
}

func TestMergeChain_ExplicitOverrideWins(t *testing.T) {
	project := Config{
		Profiles: map[string]Profile{
			"host-shell": {Environment: EnvironmentHost, Launch: LaunchShell},
		},
	}

	merged := MergeConfig(builtinConfig, project)
	applied := ApplyDefaults(merged)

	p := applied.Profiles["host-shell"]
	if p.Environment != EnvironmentHost {
		t.Errorf("Environment = %q, want %q", p.Environment, EnvironmentHost)
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
		t.Errorf("Environment = %q, want %q", p.Environment, EnvironmentContainer)
	}
	if !p.EffectiveMountSSH() {
		t.Error("mount_ssh should be true (from user defaults)")
	}
	if !p.EffectiveGhToken() {
		t.Error("gh_token should be true (from project profile)")
	}
	if err := Validate(p); err != nil {
		t.Errorf("profile should be valid: %v", err)
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
		if err := Validate(p); err != nil {
			t.Errorf("builtin profile %q should be valid: %v", name, err)
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
		t.Errorf("all profiles should be valid: %v", err)
	}
}

func TestMergeChain_ProjectDefaultsOverrideUserDefaults(t *testing.T) {
	user := Config{
		Defaults: ProfileDefaultsFromProfile(Profile{OS: OSUBI9}),
		Profiles: map[string]Profile{},
	}
	project := Config{
		Defaults: ProfileDefaultsFromProfile(Profile{OS: OSUBI10}),
		Profiles: map[string]Profile{
			"dev": {Launch: LaunchClaude},
		},
	}

	merged := MergeConfig(builtinConfig, user)
	merged = MergeConfig(merged, project)
	applied := ApplyDefaults(merged)

	if applied.Profiles["dev"].OS != OSUBI10 {
		t.Errorf("OS = %q, want %q (project default should win)", applied.Profiles["dev"].OS, OSUBI10)
	}
}
