package profile

import (
	"fmt"
	"strings"
)

// Validate checks that a profile configuration is semantically valid.
func Validate(p Profile) error {
	// Validate environment
	switch p.Environment {
	case EnvironmentHost, EnvironmentContainer:
		// ok
	case "":
		return fmt.Errorf("environment is required (\"host\" or \"container\")")
	default:
		return fmt.Errorf("unknown environment: %q (must be \"host\" or \"container\")", p.Environment)
	}

	// Validate launch mode
	switch p.Launch {
	case LaunchShell, LaunchClaude, LaunchCodex, LaunchOpenCode, LaunchZellij:
		// ok
	case "":
		return fmt.Errorf("launch is required (\"shell\", \"claude\", \"codex\", \"opencode\", or \"zellij\")")
	default:
		return fmt.Errorf("unknown launch mode: %q (must be \"shell\", \"claude\", \"codex\", \"opencode\", or \"zellij\")", p.Launch)
	}

	// Validate zellij config is only used with launch: zellij
	if p.Zellij != nil && p.Launch != LaunchZellij {
		return fmt.Errorf("zellij config is only valid with launch: zellij")
	}

	// Validate zellij tool field
	if p.Zellij != nil && p.Zellij.Tool != "" {
		switch p.Zellij.Tool {
		case "claude", "codex", "opencode":
			// ok
		default:
			return fmt.Errorf("unknown zellij tool: %q (must be \"claude\", \"codex\", or \"opencode\")", p.Zellij.Tool)
		}
	}

	// Validate os
	switch p.OS {
	case "", OSDebian12, OSUBI9, OSUBI10, OSUbuntu2604:
		// ok
	default:
		return fmt.Errorf("unknown os: %q (must be \"debian12\", \"ubi9\", \"ubi10\", or \"ubuntu2604\")", p.OS)
	}

	// Validate os is only used with environment: container
	if p.OS != "" && p.Environment != EnvironmentContainer {
		return fmt.Errorf("os is only valid with environment: container")
	}

	// Validate os and dockerfile are mutually exclusive
	if p.OS != "" && p.Dockerfile != "" {
		return fmt.Errorf("os and dockerfile are mutually exclusive; use os for built-in templates or dockerfile for a custom Dockerfile")
	}

	// Validate dockerfile is only used with environment: docker
	if p.Dockerfile != "" && p.Environment != EnvironmentContainer {
		return fmt.Errorf("dockerfile is only valid with environment: docker")
	}

	// Validate container_runtime
	switch p.ContainerRuntime {
	case "", ContainerRuntimeDocker, ContainerRuntimePodman:
		// ok
	default:
		return fmt.Errorf("unknown container_runtime: %q (must be \"docker\" or \"podman\")", p.ContainerRuntime)
	}

	// Validate mounts are only used with environment: docker
	if len(p.Mounts) > 0 && p.Environment != EnvironmentContainer {
		return fmt.Errorf("mounts are only valid with environment: docker")
	}
	for i, m := range p.Mounts {
		if m.Source == "" {
			return fmt.Errorf("mount[%d]: source is required", i)
		}
		if m.Target == "" {
			return fmt.Errorf("mount[%d]: target is required", i)
		}
	}

	return nil
}

// ValidateConfig checks the entire config for errors.
func ValidateConfig(cfg *Config) error {
	if len(cfg.Profiles) == 0 {
		return fmt.Errorf("no profiles defined")
	}

	// Check that default profile exists if specified
	if cfg.Default != "" {
		if _, ok := cfg.Profiles[cfg.Default]; !ok {
			return fmt.Errorf("default profile %q not found in profiles", cfg.Default)
		}
	}

	// Validate each profile
	var errs []string
	for name, p := range cfg.Profiles {
		if err := Validate(p); err != nil {
			errs = append(errs, fmt.Sprintf("profile %q: %v", name, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("config validation errors:\n  %s", strings.Join(errs, "\n  "))
	}

	return nil
}
