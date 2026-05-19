package cmd

import (
	"testing"

	awauth "github.com/konono/aw/internal/auth"
	"github.com/konono/aw/internal/profile"
)

func TestParseAuthArgs(t *testing.T) {
	action, target, err := parseAuthArgs([]string{"login", "codex"})
	if err != nil {
		t.Fatalf("parseAuthArgs() error: %v", err)
	}
	if action != awauth.ActionLogin {
		t.Errorf("action = %q, want %q", action, awauth.ActionLogin)
	}
	if target.Name != "codex" {
		t.Errorf("target.Name = %q, want %q", target.Name, "codex")
	}
	if target.ExplicitProfile {
		t.Error("target.ExplicitProfile = true, want false")
	}
}

func TestParseAuthArgs_RequiresProfileName(t *testing.T) {
	_, _, err := parseAuthArgs([]string{"status"})
	if err == nil {
		t.Fatal("expected error when profile name is missing")
	}
}

func TestBuildAuthStages_ContainerProfile(t *testing.T) {
	stages := buildAuthStages(profile.Profile{
		Environment: profile.EnvironmentContainer,
		Launch:      profile.LaunchCodex,
	}, awauth.ActionStatus)

	if len(stages) != 3 {
		t.Fatalf("got %d stages, want 3", len(stages))
	}
	if stages[0].Name() != "container" || stages[1].Name() != "env" || stages[2].Name() != "auth" {
		t.Fatalf("unexpected stage order: %q, %q, %q", stages[0].Name(), stages[1].Name(), stages[2].Name())
	}
}

func TestParseAuthArgs_ProfileFlag(t *testing.T) {
	_, target, err := parseAuthArgs([]string{"status", "--profile", "claude-vertex"})
	if err != nil {
		t.Fatalf("parseAuthArgs() error: %v", err)
	}
	if target.Name != "claude-vertex" {
		t.Errorf("target.Name = %q, want %q", target.Name, "claude-vertex")
	}
	if !target.ExplicitProfile {
		t.Error("target.ExplicitProfile = false, want true")
	}
}

func TestBuildToolAuthProfile_UsesDefaultDebianContainer(t *testing.T) {
	cfg := &profile.Config{
		Defaults: profile.ProfileDefaults{
			ContainerRuntime: profile.ContainerRuntimeDocker,
		},
	}
	p, name, err := buildToolAuthProfile(cfg, "claude")
	if err != nil {
		t.Fatalf("buildToolAuthProfile() error: %v", err)
	}
	if name != "claude" {
		t.Errorf("name = %q, want %q", name, "claude")
	}
	if p.Environment != profile.EnvironmentContainer {
		t.Errorf("Environment = %q, want %q", p.Environment, profile.EnvironmentContainer)
	}
	if p.Launch != profile.LaunchClaude {
		t.Errorf("Launch = %q, want %q", p.Launch, profile.LaunchClaude)
	}
	if p.OS != profile.OSDebian12 {
		t.Errorf("OS = %q, want %q", p.OS, profile.OSDebian12)
	}
	if p.ContainerRuntime != profile.ContainerRuntimeDocker {
		t.Errorf("ContainerRuntime = %q, want %q", p.ContainerRuntime, profile.ContainerRuntimeDocker)
	}
}
