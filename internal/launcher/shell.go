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

func (l *ShellLauncher) launchDockerShell(ctx context.Context, ec *pipeline.ExecutionContext) error {
	runtime := ec.Profile.EffectiveContainerRuntime()
	client := docker.NewShellClient(runtime)

	tool := ec.Profile.EffectiveTool()
	if tool == "" {
		tool = "claude"
	}
	envVars := buildContainerEnvVars(ec, tool)
	envVars["HOST_WORKSPACE"] = ec.WorkDir

	hostUser := fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid())

	runConfig := docker.RunConfig{
		ImageName:    ec.DockerImage,
		Mounts:       ec.DockerMounts,
		EnvVars:      envVars,
		WorkDir:      ec.WorkDir,
		Command:      []string{"/bin/bash"},
		SecurityOpts: ec.DockerSecurityOpts,
		CapAdd:       ec.DockerCapAdd,
		User:         hostUser,
		Userns:       podmanUserns(runtime),
	}

	return client.Run(ctx, runConfig)
}
