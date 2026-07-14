package cmd

import (
	"context"
	"fmt"

	awauth "github.com/konono/aw/v4/internal/auth"
	"github.com/konono/aw/v4/internal/pipeline"
	"github.com/konono/aw/v4/internal/profile"
	"github.com/konono/aw/v4/internal/stage"
)

type authTarget struct {
	Name            string
	ExplicitProfile bool
}

func runAuthAction(action awauth.Action, tool, profileFlag string) error {
	target := authTarget{Name: tool}
	if profileFlag != "" {
		target.Name = profileFlag
		target.ExplicitProfile = true
	}

	if target.Name == "" {
		return fmt.Errorf("tool name is required")
	}

	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	p, resolvedName, err := resolveAuthTarget(cfg, target)
	if err != nil {
		return err
	}

	ec, err := buildExecutionContext(resolvedName, p)
	if err != nil {
		return err
	}

	stages := buildAuthStages(p, action)
	pipe := pipeline.New(stages...)
	return pipe.Execute(context.Background(), ec)
}

// Run handles auth login.
func (a *AuthLoginCmd) Run() error {
	return runAuthAction(awauth.ActionLogin, a.Tool, a.Profile)
}

// Run handles auth logout.
func (a *AuthLogoutCmd) Run() error {
	return runAuthAction(awauth.ActionLogout, a.Tool, a.Profile)
}

// Run handles auth status.
func (a *AuthStatusCmd) Run() error {
	return runAuthAction(awauth.ActionStatus, a.Tool, a.Profile)
}

// Run handles the login alias (delegates to auth login).
func (l *LoginCmd) Run() error {
	return runAuthAction(awauth.ActionLogin, l.Tool, l.Profile)
}

func loadConfig() (*profile.Config, error) {
	cfg, err := profile.Load()
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}
	return cfg, nil
}

func loadExplicitProfile(cfg *profile.Config, profileName string) (profile.Profile, string, error) {
	p, ok := cfg.Profiles[profileName]
	if !ok {
		return profile.Profile{}, "", fmt.Errorf("profile %q not found", profileName)
	}

	if err := profile.Validate(p); err != nil {
		return profile.Profile{}, "", fmt.Errorf("invalid profile %q: %w", profileName, err)
	}

	return p, profileName, nil
}

func resolveAuthTarget(cfg *profile.Config, target authTarget) (profile.Profile, string, error) {
	if target.ExplicitProfile {
		return loadExplicitProfile(cfg, target.Name)
	}

	switch target.Name {
	case "claude", "codex", "opencode", "cursor":
		return buildToolAuthProfile(cfg, target.Name)
	default:
		return loadExplicitProfile(cfg, target.Name)
	}
}

func buildToolAuthProfile(cfg *profile.Config, tool string) (profile.Profile, string, error) {
	p := profile.Profile{
		Environment: profile.EnvironmentContainer,
		OS:          profile.OSDebian12,
	}
	if cfg != nil && cfg.Defaults.ContainerRuntime != "" {
		p.ContainerRuntime = cfg.Defaults.ContainerRuntime
	}

	switch tool {
	case "claude":
		p.Launch = profile.LaunchClaude
	case "codex":
		p.Launch = profile.LaunchCodex
	case "opencode":
		p.Launch = profile.LaunchOpenCode
	case "cursor":
		p.Launch = profile.LaunchCursor
	default:
		return profile.Profile{}, "", fmt.Errorf("unknown tool %q", tool)
	}

	if err := profile.Validate(p); err != nil {
		return profile.Profile{}, "", err
	}
	return p, tool, nil
}

func buildAuthStages(p profile.Profile, action awauth.Action) []pipeline.Stage {
	var stages []pipeline.Stage
	if p.Environment == profile.EnvironmentContainer {
		stages = append(stages, stage.NewDockerStage(), &stage.EnvStage{})
	}
	stages = append(stages, &stage.AuthStage{Action: action})
	return stages
}
