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

	// Validate ssh_agent_forwarding is only used with environment: container
	if p.EffectiveSSHAgentForwarding() && p.Environment != EnvironmentContainer {
		return fmt.Errorf("ssh_agent_forwarding is only valid with environment: container")
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
		switch m.Mode {
		case "", MountModeRO, MountModeRW:
			// ok
		default:
			return fmt.Errorf("mount[%d]: unknown mode %q (must be \"ro\" or \"rw\")", i, m.Mode)
		}
	}

	if err := validateAuth(p.Auth); err != nil {
		return err
	}

	return nil
}

func validateAuth(auth *AuthConfig) error {
	if auth == nil {
		return nil
	}

	if auth.OnLaunch != nil {
		switch auth.OnLaunch.Check {
		case "", AuthOnLaunchCheckNone, AuthOnLaunchCheckWarn, AuthOnLaunchCheckRequire:
			// ok
		default:
			return fmt.Errorf("unknown auth.on_launch.check: %q (must be \"none\", \"warn\", or \"require\")", auth.OnLaunch.Check)
		}
	}

	if auth.Codex != nil {
		switch auth.Codex.LoginMode {
		case "", CodexLoginModeBrowser, CodexLoginModeDevice, CodexLoginModeAPIKey, CodexLoginModeAccessToken:
			// ok
		default:
			return fmt.Errorf("unknown auth.codex.login_mode: %q (must be \"browser\", \"device\", \"api-key\", or \"access-token\")", auth.Codex.LoginMode)
		}
		switch auth.Codex.CredentialsStore {
		case "", CodexCredentialsStoreFile, CodexCredentialsStoreKeyring, CodexCredentialsStoreAuto:
			// ok
		default:
			return fmt.Errorf("unknown auth.codex.credentials_store: %q (must be \"file\", \"keyring\", or \"auto\")", auth.Codex.CredentialsStore)
		}
		switch auth.Codex.SeedFromHost {
		case "", AuthSeedFromHostIfMissing, AuthSeedFromHostAlways, AuthSeedFromHostNever:
			// ok
		default:
			return fmt.Errorf("unknown auth.codex.seed_from_host: %q (must be \"if_missing\", \"always\", or \"never\")", auth.Codex.SeedFromHost)
		}
		switch auth.Codex.PersistAuth {
		case "", AuthPersistModeStage:
			// ok
		default:
			return fmt.Errorf("unknown auth.codex.persist_auth: %q (must be \"stage\")", auth.Codex.PersistAuth)
		}
	}

	if auth.Claude != nil {
		switch auth.Claude.LoginMode {
		case "", ClaudeLoginModeBrowser, ClaudeLoginModeConsole, ClaudeLoginModeEmail, ClaudeLoginModeSSO:
			// ok
		default:
			return fmt.Errorf("unknown auth.claude.login_mode: %q (must be \"browser\", \"console\", \"email\", or \"sso\")", auth.Claude.LoginMode)
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
