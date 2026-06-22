package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	awauth "github.com/konono/aw/internal/auth"
	"github.com/konono/aw/internal/pipeline"
	"github.com/konono/aw/internal/profile"
	"github.com/konono/aw/internal/stage"
)

type authTarget struct {
	Name            string
	ExplicitProfile bool
}

func runAuth(args []string) int {
	action, target, err := parseAuthArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		printAuthHelp()
		return 1
	}

	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	p, resolvedName, err := resolveAuthTarget(cfg, target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	ec, err := buildExecutionContext(resolvedName, p)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	stages := buildAuthStages(p, action)
	pipe := pipeline.New(stages...)
	if err := pipe.Execute(context.Background(), ec); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	return 0
}

func parseAuthArgs(args []string) (awauth.Action, authTarget, error) {
	if len(args) == 0 {
		return "", authTarget{}, fmt.Errorf("auth action is required")
	}

	var action awauth.Action
	switch args[0] {
	case "login":
		action = awauth.ActionLogin
	case "logout":
		action = awauth.ActionLogout
	case "status":
		action = awauth.ActionStatus
	default:
		return "", authTarget{}, fmt.Errorf("unknown auth action %q", args[0])
	}

	target := authTarget{}
	rest := args[1:]
	for i := 0; i < len(rest); i++ {
		arg := rest[i]
		switch arg {
		case "--profile", "-p":
			if i+1 >= len(rest) {
				return "", authTarget{}, fmt.Errorf("%s requires a profile name", arg)
			}
			target.Name = rest[i+1]
			target.ExplicitProfile = true
			i++
		default:
			if strings.HasPrefix(arg, "-") {
				return "", authTarget{}, fmt.Errorf("unknown auth flag %q", arg)
			}
			if target.Name != "" {
				return "", authTarget{}, fmt.Errorf("too many auth targets")
			}
			target.Name = arg
		}
	}

	if target.Name == "" {
		return "", authTarget{}, fmt.Errorf("tool name is required")
	}

	return action, target, nil
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

func printAuthHelp() {
	fmt.Println("Usage:")
	fmt.Println("  aw auth login <tool>")
	fmt.Println("  aw auth logout <tool>")
	fmt.Println("  aw auth status <tool>")
	fmt.Println("  aw auth login --profile <name>")
	fmt.Println("  aw auth logout --profile <name>")
	fmt.Println("  aw auth status --profile <name>")
	fmt.Println("  aw login <tool>          (alias for `aw auth login <tool>`)")
}
