package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/konono/aw/internal/doctor"
	"github.com/konono/aw/internal/image"
	"github.com/konono/aw/internal/pipeline"
	"github.com/konono/aw/internal/platform"
	"github.com/konono/aw/internal/profile"
	"github.com/konono/aw/internal/reaper"
	"github.com/konono/aw/internal/stage"
	"github.com/konono/aw/internal/update"
	"github.com/konono/aw/internal/version"
)

// Run method for RunCmd — the default command that launches a profile.
func (r *RunCmd) Run(pt *PassthroughCmd) error {
	// Save original working directory before any chdir
	origCwd, err := os.Getwd()
	if err != nil {
		return err
	}

	// Handle -C / --cwd: chdir before profile.Load()
	if r.Cwd != "" {
		target := expandTilde(r.Cwd)
		if err := os.Chdir(target); err != nil {
			return fmt.Errorf("cannot change to directory %q: %w", target, err)
		}
	}

	// Handle --recent: pick a directory from history, then chdir
	if r.Recent {
		dir, cancelled, err := selectRecentDir(r.Query)
		if err != nil {
			return err
		}
		if cancelled {
			return nil
		}
		if err := os.Chdir(dir); err != nil {
			return fmt.Errorf("cannot change to directory %q: %w", dir, err)
		}
	}

	// Load config (after chdir so .aw.yml is found in the selected directory)
	cfg, err := profile.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Validate the whole config
	if err := profile.ValidateConfig(cfg); err != nil {
		return err
	}

	profileName := r.Profile

	// If no profile name given, use default or list profiles
	if profileName == "" {
		if cfg.Default != "" {
			profileName = cfg.Default
		} else {
			printAvailableProfiles(cfg)
			return nil
		}
	}

	p, ok := cfg.Profiles[profileName]
	if !ok {
		fmt.Fprintf(os.Stderr, "Error: profile %q not found\n", profileName)
		printAvailableProfiles(cfg)
		return ExitError{Code: 1}
	}

	// Validate the selected profile
	if err := profile.Validate(p); err != nil {
		return fmt.Errorf("invalid profile %q: %w", profileName, err)
	}

	// Build execution context
	ec, err := buildExecutionContext(profileName, p)
	if err != nil {
		return err
	}
	ec.CommandOverride = pt.Args
	ec.NoCache = r.NoCache

	if len(ec.CommandOverride) > 0 && p.Environment == profile.EnvironmentHost {
		return fmt.Errorf("command passthrough is only supported with environment: container")
	}

	if pt.UsedLegacy && len(ec.CommandOverride) > 0 && os.Getenv("AW_NO_DEPRECATION") == "" {
		fmt.Fprintln(os.Stderr, "Warning: '-c' is deprecated for command passthrough; use '--' instead (e.g. aw claude -- echo hello). Suppress with AW_NO_DEPRECATION=1.")
	}

	// Record directory history (use OrigWorkDir, not worktree path)
	if !r.NoRecord {
		recordDir := ec.OrigWorkDir
		if r.Cwd != "" || r.Recent {
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
	if p.Environment == profile.EnvironmentContainer && platform.IsRunningAsRoot() {
		fmt.Fprintf(os.Stderr, "Error: aw must not be run as root — the container user cannot match uid 0.\n")
		fmt.Fprintf(os.Stderr, "Run as a regular user, or create one: useradd -m dev && su - dev\n")
		return ExitError{Code: 1}
	}

	// Container mode setup: recovery, stale check, name generation
	if p.Environment == profile.EnvironmentContainer {
		runtime := p.EffectiveContainerRuntime()

		reaper.CheckOrphanedReapers(runtime)
		reaper.CheckStaleContainers(runtime)

		// Nanosecond suffix avoids collisions when the same profile starts twice in one second.
		ec.ContainerName = fmt.Sprintf("aw-%s-%d", profileName, time.Now().UnixNano())
		if err := reaper.CheckStaleContainer(runtime, ec.ContainerName); err != nil {
			return err
		}
	}

	// Build pipeline stages
	stages := buildStages(p)
	pipe := pipeline.New(stages...)

	if err := pipe.Execute(context.Background(), ec); err != nil {
		runCleanups(ec)
		return err
	}

	// Container launch calls os.Exit on success; runCleanups is unreachable
	// in that case. Post-container cleanup runs in the reaper subprocess.
	runCleanups(ec)
	return nil
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

// ProfilesCmd.Run lists profiles.
func (p *ProfilesCmd) Run() error {
	cfg, err := profile.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Show config source
	if cfg.Source.IsBuiltin {
		fmt.Println("Source: built-in embedded default (no config files found)")
	} else {
		fmt.Printf("Source: %s\n", cfg.Source.FilePath)
	}
	fmt.Println()

	printAvailableProfiles(cfg)
	return nil
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

// UpdateCmd.Run updates aw.
func (u *UpdateCmd) Run() error {
	return update.Run(version.Version)
}

// DefaultDockerfileCmd.Run prints the default Dockerfile.
func (d *DefaultDockerfileCmd) Run() error {
	_, err := os.Stdout.Write(image.DefaultDockerfile())
	return err
}

// DefaultInitScriptCmd.Run prints the init script.
func (d *DefaultInitScriptCmd) Run() error {
	_, err := os.Stdout.Write(image.InitScript())
	return err
}

// DoctorCmd.Run runs doctor checks.
func (d *DoctorCmd) Run() error {
	return exitCode(doctor.RunDiagnostics(d.Verbose))
}
