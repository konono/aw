package cmd

import (
	"testing"

	"github.com/konono/aw/internal/pipeline"
	"github.com/konono/aw/internal/profile"
)

func TestBuildStages_DockerClaude(t *testing.T) {
	p := profile.Profile{
		Environment: profile.EnvironmentContainer,
		Launch:      profile.LaunchClaude,
	}
	stages := buildStages(p)

	// Should have DockerStage + EnvStage + LaunchStage = 3 stages
	if len(stages) != 3 {
		t.Fatalf("got %d stages, want 3", len(stages))
	}
	if stages[0].Name() != "container" {
		t.Errorf("stage[0] = %q, want 'container'", stages[0].Name())
	}
	if stages[1].Name() != "env" {
		t.Errorf("stage[1] = %q, want 'env'", stages[1].Name())
	}
	if stages[2].Name() != "launch" {
		t.Errorf("stage[2] = %q, want 'launch'", stages[2].Name())
	}
}

func TestBuildStages_WorktreeHostShell(t *testing.T) {
	p := profile.Profile{
		Worktree:    &profile.WorktreeConfig{},
		Environment: profile.EnvironmentHost,
		Launch:      profile.LaunchShell,
	}
	stages := buildStages(p)

	// Should have WorktreeStage + LaunchStage = 2 stages
	if len(stages) != 2 {
		t.Fatalf("got %d stages, want 2", len(stages))
	}
	if stages[0].Name() != "worktree" {
		t.Errorf("stage[0] = %q, want 'worktree'", stages[0].Name())
	}
	if stages[1].Name() != "launch" {
		t.Errorf("stage[1] = %q, want 'launch'", stages[1].Name())
	}
}

func TestBuildStages_HostClaude(t *testing.T) {
	p := profile.Profile{
		Environment: profile.EnvironmentHost,
		Launch:      profile.LaunchClaude,
	}
	stages := buildStages(p)

	// Should have LaunchStage only = 1 stage
	if len(stages) != 1 {
		t.Fatalf("got %d stages, want 1", len(stages))
	}
	if stages[0].Name() != "launch" {
		t.Errorf("stage[0] = %q, want 'launch'", stages[0].Name())
	}
}

func TestBuildStages_AddsAuthCheckBeforeLaunch(t *testing.T) {
	p := profile.Profile{
		Environment: profile.EnvironmentContainer,
		Launch:      profile.LaunchCodex,
		Auth: &profile.AuthConfig{
			OnLaunch: &profile.OnLaunchAuthConfig{Check: profile.AuthOnLaunchCheckWarn},
		},
	}
	stages := buildStages(p)

	if len(stages) != 4 {
		t.Fatalf("got %d stages, want 4", len(stages))
	}
	if stages[2].Name() != "auth-check" {
		t.Errorf("stage[2] = %q, want 'auth-check'", stages[2].Name())
	}
	if stages[3].Name() != "launch" {
		t.Errorf("stage[3] = %q, want 'launch'", stages[3].Name())
	}
}

func TestRunOnEndIfConfigured_SkipsWhenNoWorktree(t *testing.T) {
	ec := &pipeline.ExecutionContext{
		Profile: profile.Profile{
			Environment: profile.EnvironmentHost,
			Launch:      profile.LaunchShell,
		},
	}
	// Should not panic or error
	runOnEndIfConfigured(ec)
}

func TestRunOnEndIfConfigured_SkipsWhenNoOnEnd(t *testing.T) {
	ec := &pipeline.ExecutionContext{
		Profile: profile.Profile{
			Worktree:    &profile.WorktreeConfig{},
			Environment: profile.EnvironmentHost,
			Launch:      profile.LaunchShell,
		},
		WorktreePath: "/some/path",
	}
	// Should not panic or error
	runOnEndIfConfigured(ec)
}

func TestRunOnEndIfConfigured_SkipsWhenWorktreePathEmpty(t *testing.T) {
	ec := &pipeline.ExecutionContext{
		Profile: profile.Profile{
			Worktree:    &profile.WorktreeConfig{OnEnd: "echo done"},
			Environment: profile.EnvironmentContainer,
			Launch:      profile.LaunchClaude,
		},
		WorktreePath: "",
	}
	// Should not panic or error (WorktreeStage didn't run)
	runOnEndIfConfigured(ec)
}

func TestDescribeProfile(t *testing.T) {
	tests := []struct {
		name    string
		profile profile.Profile
		want    string
	}{
		{
			"container claude",
			profile.Profile{Environment: profile.EnvironmentContainer, Launch: profile.LaunchClaude},
			"container + claude",
		},
		{
			"worktree host shell",
			profile.Profile{Worktree: &profile.WorktreeConfig{}, Environment: profile.EnvironmentHost, Launch: profile.LaunchShell},
			"worktree + host + shell",
		},
		{
			"container claude with os",
			profile.Profile{Environment: profile.EnvironmentContainer, Launch: profile.LaunchClaude, OS: profile.OSUBI9},
			"container + claude + os:ubi9",
		},
		{
			"container shell with os ubuntu",
			profile.Profile{Environment: profile.EnvironmentContainer, Launch: profile.LaunchShell, OS: profile.OSUbuntu2604},
			"container + shell + os:ubuntu2604",
		},
		{
			"container claude with image",
			profile.Profile{Environment: profile.EnvironmentContainer, Launch: profile.LaunchClaude, Image: "my-image:latest"},
			"container + claude + image:my-image:latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := profile.Describe(tt.profile)
			if got != tt.want {
				t.Errorf("profile.Describe() = %q, want %q", got, tt.want)
			}
		})
	}
}
