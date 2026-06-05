package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/konono/aw/internal/image"
	"github.com/konono/aw/internal/pipeline"
	"github.com/konono/aw/internal/profile"
	"github.com/konono/aw/internal/stage"
	"github.com/konono/aw/internal/update"
	"github.com/konono/aw/internal/version"
)

// Run is the top-level entry point. Returns an exit code.
func Run(args []string) int {
	if hasHelpFlag(args) {
		printHelp()
		return 0
	}

	if hasVersionFlag(args) {
		fmt.Printf("aw %s\n", version.Version)
		return 0
	}

	if len(args) > 0 && args[0] == "update" {
		if err := update.Run(version.Version); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}
		return 0
	}

	if len(args) > 0 && args[0] == "profiles" {
		return runProfiles()
	}

	if len(args) > 0 && args[0] == "default-dockerfile" {
		return runDefaultDockerfile()
	}

	if len(args) > 0 && args[0] == "export" {
		return runExport(args[1:])
	}

	if len(args) > 0 && args[0] == "init" {
		return runInit(args[1:])
	}

	if len(args) > 0 && args[0] == "auth" {
		return runAuth(args[1:])
	}

	if len(args) > 0 && args[0] == "login" {
		return runAuth(append([]string{"login"}, args[1:]...))
	}

	// Determine profile name
	profileName := ""
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		profileName = args[0]
	}

	// Load config
	cfg, err := profile.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		return 1
	}

	// Validate the whole config
	if err := profile.ValidateConfig(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	// If no profile name given, use default or list profiles
	if profileName == "" {
		if cfg.Default != "" {
			profileName = cfg.Default
		} else {
			printAvailableProfiles(cfg)
			return 0
		}
	}

	p, ok := cfg.Profiles[profileName]
	if !ok {
		fmt.Fprintf(os.Stderr, "Error: profile %q not found\n", profileName)
		printAvailableProfiles(cfg)
		return 1
	}

	// Validate the selected profile
	if err := profile.Validate(p); err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid profile %q: %v\n", profileName, err)
		return 1
	}

	// Build execution context
	ec, err := buildExecutionContext(profileName, p)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	// Warn about on-end limitations
	if p.Worktree != nil && p.Worktree.OnEnd != "" &&
		p.Environment == profile.EnvironmentHost &&
		p.Launch != profile.LaunchZellij {
		fmt.Fprintf(os.Stderr, "Warning: on-end hook will not run with environment: host + launch: %s (process is replaced via exec)\n", p.Launch)
	}

	// Build pipeline stages
	stages := buildStages(p)
	pipe := pipeline.New(stages...)

	if err := pipe.Execute(context.Background(), ec); err != nil {
		runCleanups(ec)
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	runCleanups(ec)
	return 0
}

func runCleanups(ec *pipeline.ExecutionContext) {
	if ec.SSHAgentCleanup != nil {
		ec.SSHAgentCleanup()
	}
	runOnEndIfConfigured(ec)
}

func runOnEndIfConfigured(ec *pipeline.ExecutionContext) {
	if ec.Profile.Worktree == nil || ec.Profile.Worktree.OnEnd == "" {
		return
	}
	if ec.WorktreePath == "" {
		return
	}
	fmt.Fprintf(os.Stderr, "Running on-end hook...\n")
	if err := stage.RunOnEndHook(ec); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: on-end hook failed: %v\n", err)
	}
}

// buildStages creates the pipeline stages based on the profile configuration.
func buildStages(p profile.Profile) []pipeline.Stage {
	var stages []pipeline.Stage

	// Stage 1: Worktree (conditional)
	if p.Worktree != nil {
		stages = append(stages, &stage.WorktreeStage{})
	}

	// Stage 2: Docker setup (conditional)
	if p.Environment == profile.EnvironmentContainer {
		stages = append(stages, stage.NewDockerStage())
	}

	// Stage 3: Env loading (conditional — only for Docker, where custom env vars are needed)
	if p.Environment == profile.EnvironmentContainer {
		stages = append(stages, &stage.EnvStage{})
	}

	// Stage 4: Optional auth preflight before normal launch.
	if p.EffectiveAuthOnLaunchCheck() != "" && p.EffectiveAuthOnLaunchCheck() != profile.AuthOnLaunchCheckNone {
		stages = append(stages, &stage.AuthCheckStage{})
	}

	// Stage 5: Launch (always)
	stages = append(stages, &stage.LaunchStage{})

	return stages
}

func runProfiles() int {
	cfg, err := profile.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		return 1
	}

	// Show config source
	if cfg.Source.IsBuiltin {
		fmt.Println("Source: built-in embedded default (no config files found)")
	} else {
		fmt.Printf("Source: %s\n", cfg.Source.FilePath)
	}
	fmt.Println()

	printAvailableProfiles(cfg)
	return 0
}

func printAvailableProfiles(cfg *profile.Config) {
	fmt.Println("Available profiles:")
	for name, p := range cfg.Profiles {
		marker := "  "
		if name == cfg.Default {
			marker = "* "
		}

		desc := describeProfile(p)
		fmt.Printf("  %s%s  (%s)\n", marker, name, desc)
	}
	fmt.Println()
	fmt.Println("Usage: aw <profile-name>")
	if cfg.Default != "" {
		fmt.Printf("       aw              (runs default: %s)\n", cfg.Default)
	}
}

func describeProfile(p profile.Profile) string {
	parts := []string{}
	if p.Worktree != nil {
		parts = append(parts, "worktree")
	}
	parts = append(parts, string(p.Environment))
	parts = append(parts, string(p.Launch))
	if p.OS != "" {
		parts = append(parts, "os:"+string(p.OS))
	}
	if p.Image != "" {
		parts = append(parts, "image:"+p.Image)
	}
	if p.Dockerfile != "" {
		parts = append(parts, "dockerfile:"+p.Dockerfile)
	}
	return strings.Join(parts, " + ")
}

func runDefaultDockerfile() int {
	_, err := os.Stdout.Write(image.DefaultDockerfile())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error writing Dockerfile: %v\n", err)
		return 1
	}
	return 0
}

// hasVersionFlag checks if the args contain --version or -v.
func hasVersionFlag(args []string) bool {
	for _, a := range args {
		if a == "--version" || a == "-v" {
			return true
		}
	}
	return false
}

// hasHelpFlag checks if the args contain --help or -h.
func hasHelpFlag(args []string) bool {
	for _, a := range args {
		if a == "--help" || a == "-h" {
			return true
		}
	}
	return false
}

func printHelp() {
	fmt.Printf("aw %s - agent-workspace\n\n", version.Version)
	fmt.Println("Usage:")
	fmt.Println("  aw                      Run default profile")
	fmt.Println("  aw <profile>            Run a specific profile")
	fmt.Println("  aw profiles             List available profiles")
	fmt.Println("  aw init                 Write the built-in config to ~/.config/aw/config.yml")
	fmt.Println("  aw init --update        Migrate existing config to the latest template")
	fmt.Println("  aw auth <action> <tool> Run auth login/logout/status for a tool")
	fmt.Println("  aw login <tool>         Alias for `aw auth login <tool>`")
	fmt.Println("  aw export <profile>     Build and export a profile's image as a tar archive")
	fmt.Println("  aw default-dockerfile   Print the default Dockerfile")
	fmt.Println("  aw update               Update aw to the latest version")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  -h, --help              Show this help")
	fmt.Println("  -v, --version           Show version")
}
