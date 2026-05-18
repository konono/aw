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

// OpenCodeLauncher runs OpenCode.
type OpenCodeLauncher struct{}

func (l *OpenCodeLauncher) Launch(ctx context.Context, ec *pipeline.ExecutionContext) error {
	switch ec.Profile.Environment {
	case profile.EnvironmentHost:
		return l.launchHostOpenCode(ec)
	case profile.EnvironmentContainer:
		return l.launchDockerOpenCode(ctx, ec)
	default:
		return fmt.Errorf("unsupported environment: %q", ec.Profile.Environment)
	}
}

func (l *OpenCodeLauncher) launchHostOpenCode(ec *pipeline.ExecutionContext) error {
	opencodePath, err := exec.LookPath("opencode")
	if err != nil {
		return fmt.Errorf("opencode is not installed. Install via: devbox add opencode")
	}

	fmt.Fprintf(os.Stderr, "Launching OpenCode in %s\n", ec.WorkDir)

	args := []string{"opencode"}
	env := os.Environ()
	return syscall.Exec(opencodePath, args, env)
}

func (l *OpenCodeLauncher) launchDockerOpenCode(ctx context.Context, ec *pipeline.ExecutionContext) error {
	client := docker.NewShellClient(ec.Profile.EffectiveContainerRuntime())

	command := []string{"opencode"}

	envVars := buildContainerEnvVars(ec, "opencode")
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
