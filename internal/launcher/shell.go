package launcher

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/konono/aw/internal/docker"
	"github.com/konono/aw/internal/pipeline"
	"github.com/konono/aw/internal/profile"
	"github.com/konono/aw/internal/reaper"
)

// ShellLauncher opens a shell in the workspace.
type ShellLauncher struct{}

func (l *ShellLauncher) Launch(ctx context.Context, ec *pipeline.ExecutionContext) error {
	switch ec.Profile.Environment {
	case profile.EnvironmentHost:
		return l.launchHostShell(ec)
	case profile.EnvironmentContainer:
		return l.launchDockerShell(ctx, ec)
	default:
		return fmt.Errorf("unsupported environment: %q", ec.Profile.Environment)
	}
}

func (l *ShellLauncher) launchHostShell(ec *pipeline.ExecutionContext) error {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}

	shellPath, err := exec.LookPath(shell)
	if err != nil {
		return fmt.Errorf("shell not found: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Opening shell in %s\n", ec.WorkDir)

	env := os.Environ()
	return syscall.Exec(shellPath, []string{shell}, env)
}

func (l *ShellLauncher) launchDockerShell(_ context.Context, ec *pipeline.ExecutionContext) error {
	runtime := ec.Profile.EffectiveContainerRuntime()
	client := docker.NewShellClient(runtime)

	command := ec.CommandOverride
	if len(command) == 0 {
		command = []string{"/bin/bash"}
	}

	spec := reaper.BuildSpec(ec)
	runConfig := pipeline.ShellRunConfig(ec, runtime, command)
	return client.ExecRun(ec.ContainerName, runConfig, func() (*os.File, func(), error) {
		handle, err := reaper.Spawn(spec)
		if err != nil {
			return nil, nil, err
		}
		return handle.Write, handle.Abort, nil
	})
}
