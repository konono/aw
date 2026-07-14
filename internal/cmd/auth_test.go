package cmd

import (
	"testing"

	awauth "github.com/konono/aw/v4/internal/auth"
	"github.com/konono/aw/v4/internal/profile"
)

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

func TestBuildToolAuthProfile_Cursor(t *testing.T) {
	p, name, err := buildToolAuthProfile(nil, "cursor")
	if err != nil {
		t.Fatalf("buildToolAuthProfile() error: %v", err)
	}
	if name != "cursor" {
		t.Errorf("name = %q, want %q", name, "cursor")
	}
	if p.Launch != profile.LaunchCursor {
		t.Errorf("Launch = %q, want %q", p.Launch, profile.LaunchCursor)
	}
}

func TestResolveAuthTarget_CursorUsesToolProfile(t *testing.T) {
	p, name, err := resolveAuthTarget(nil, authTarget{Name: "cursor"})
	if err != nil {
		t.Fatalf("resolveAuthTarget() error: %v", err)
	}
	if name != "cursor" {
		t.Errorf("name = %q, want %q", name, "cursor")
	}
	if p.Launch != profile.LaunchCursor {
		t.Errorf("Launch = %q, want %q", p.Launch, profile.LaunchCursor)
	}
}

func TestResolveAuthTarget_ExplicitProfile(t *testing.T) {
	target := authTarget{Name: "claude-vertex", ExplicitProfile: true}
	// This will fail since there's no config loaded, but verifies the code path.
	_, _, err := resolveAuthTarget(&profile.Config{
		Profiles: map[string]profile.Profile{
			"claude-vertex": {
				Environment: profile.EnvironmentContainer,
				Launch:      profile.LaunchClaude,
				OS:          profile.OSDebian12,
			},
		},
	}, target)
	if err != nil {
		t.Fatalf("resolveAuthTarget() error: %v", err)
	}
}
