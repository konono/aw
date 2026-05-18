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

// ClaudeLauncher runs Claude Code.
type ClaudeLauncher struct{}

func (l *ClaudeLauncher) Launch(ctx context.Context, ec *pipeline.ExecutionContext) error {
	switch ec.Profile.Environment {
	case profile.EnvironmentHost:
		return l.launchHostClaude(ec)
	case profile.EnvironmentContainer:
		return l.launchDockerClaude(ctx, ec)
	default:
		return fmt.Errorf("unsupported environment: %q", ec.Profile.Environment)
	}
}

func (l *ClaudeLauncher) launchHostClaude(ec *pipeline.ExecutionContext) error {
	claudePath, err := exec.LookPath("claude")
	if err != nil {
		return fmt.Errorf("claude is not installed. Install Claude Code: https://claude.ai/install.sh")
	}

	fmt.Fprintf(os.Stderr, "Launching Claude in %s\n", ec.WorkDir)

	args := []string{"claude"}
	env := os.Environ()
	return syscall.Exec(claudePath, args, env)
}

func (l *ClaudeLauncher) launchDockerClaude(ctx context.Context, ec *pipeline.ExecutionContext) error {
	client := docker.NewShellClient(ec.Profile.EffectiveContainerRuntime())

	command := []string{"claude", "--permission-mode", "bypassPermissions"}

	envVars := buildContainerEnvVars(ec, "claude")
	envVars["HOST_WORKSPACE"] = ec.WorkDir

	runConfig := docker.RunConfig{
		ImageName: ec.DockerImage,
		Mounts:    ec.DockerMounts,
		EnvVars:   envVars,
		WorkDir:   ec.WorkDir,
		Command:   command,
	}

	return client.Run(ctx, runConfig)
}
