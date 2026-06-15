package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/konono/aw/internal/doctor"
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

	if len(args) > 0 && args[0] == "doctor" {
		return doctor.Run(args[1:])
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

	// Parse profile name and run options
	opts, err := parseRunArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	// Save original working directory before any chdir
	origCwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	// Handle -C / --cwd: chdir before profile.Load()
	if opts.Cwd != "" {
		target := expandTilde(opts.Cwd)
		if err := os.Chdir(target); err != nil {
			fmt.Fprintf(os.Stderr, "Error: cannot change to directory %q: %v\n", target, err)
			return 1
		}
	}

	// Handle --recent: pick a directory from history, then chdir
	if opts.Recent {
		dir, cancelled, err := selectRecentDir(opts.Query)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}
		if cancelled {
			return 0
		}
		if err := os.Chdir(dir); err != nil {
			fmt.Fprintf(os.Stderr, "Error: cannot change to directory %q: %v\n", dir, err)
			return 1
		}
	}

	// Load config (after chdir so .aw.yml is found in the selected directory)
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

	profileName := opts.ProfileName

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
	ec.CommandOverride = opts.Command

	if len(ec.CommandOverride) > 0 && p.Environment == profile.EnvironmentHost {
		fmt.Fprintf(os.Stderr, "Error: -c flag is only supported with environment: container\n")
		return 1
	}

	// Record directory history (use OrigWorkDir, not worktree path)
	if !opts.NoRecord {
		recordDir := ec.OrigWorkDir
		if opts.Cwd != "" || opts.Recent {
			// When -C or --recent was used, the current dir is the target
			if cwd, err := os.Getwd(); err == nil {
				recordDir = cwd
			}
		}
		recordDirHistory(recordDir, profileName, origCwd)
	}

	// Warn about on-end limitations
	if p.Worktree != nil && p.Worktree.OnEnd != "" &&
		p.Environment == profile.EnvironmentHost {
		fmt.Fprintf(os.Stderr, "Warning: on-end hook will not run with environment: host + launch: %s (process is replaced via exec)\n", p.Launch)
	}

	// The launcher passes --user <host-uid>:<host-gid> so the container
	// process owns the same files as the host user. uid 0 is rejected
	// because Claude Code refuses to run as root.
	if p.Environment == profile.EnvironmentContainer && os.Getuid() == 0 {
		fmt.Fprintf(os.Stderr, "Error: aw must not be run as root — the container user cannot match uid 0.\n")
		fmt.Fprintf(os.Stderr, "Run as a regular user, or create one: useradd -m dev && su - dev\n")
		return 1
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
	authCheck := p.EffectiveAuthOnLaunchCheck()
	if authCheck != "" && authCheck != profile.AuthOnLaunchCheckNone {
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

		desc := profile.Describe(p)
		fmt.Printf("  %s%s  (%s)\n", marker, name, desc)
	}
	fmt.Println()
	fmt.Println("Usage: aw <profile-name>")
	if cfg.Default != "" {
		fmt.Printf("       aw              (runs default: %s)\n", cfg.Default)
	}
}

func runDefaultDockerfile() int {
	_, err := os.Stdout.Write(image.DefaultDockerfile())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error writing Dockerfile: %v\n", err)
		return 1
	}
	return 0
}

var subcommands = map[string]bool{
	"update": true, "profiles": true, "default-dockerfile": true,
	"export": true, "init": true, "auth": true, "login": true, "doctor": true,
}

// hasVersionFlag checks if the args contain --version or -v before any subcommand or -c flag.
func hasVersionFlag(args []string) bool {
	for _, a := range args {
		if a == "-c" || subcommands[a] {
			return false
		}
		if a == "--version" || a == "-v" {
			return true
		}
	}
	return false
}

// hasHelpFlag checks if the args contain --help or -h before any subcommand or -c flag.
func hasHelpFlag(args []string) bool {
	for _, a := range args {
		if a == "-c" || subcommands[a] {
			return false
		}
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
	fmt.Println("  aw auth <action> <tool> Run auth login/logout/status for a tool")
	fmt.Println("  aw login <tool>         Alias for `aw auth login <tool>`")
	fmt.Println("  aw export <profile>     Build and export a profile's image as a tar archive")
	fmt.Println("                          Use --snapshot to bake runtime setup into the image")
	fmt.Println("  aw doctor               Check system environment and configuration")
	fmt.Println("  aw default-dockerfile   Print the default Dockerfile")
	fmt.Println("  aw update               Update aw to the latest version")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  -c <cmd> [args...]      Override the launch command (container only; e.g. aw claude -c claude --version)")
	fmt.Println("  -r, --recent            Pick a directory from launch history")
	fmt.Println("  --query <text>          Initial query for --recent picker")
	fmt.Println("  -C, --cwd <path>        Change to <path> before loading config")
	fmt.Println("  --no-record             Don't record this launch in directory history")
	fmt.Println("  -h, --help              Show this help")
	fmt.Println("  -v, --version           Show version")
}
