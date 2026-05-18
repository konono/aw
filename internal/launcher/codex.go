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

// CodexLauncher runs OpenAI Codex CLI.
type CodexLauncher struct{}

func (l *CodexLauncher) Launch(ctx context.Context, ec *pipeline.ExecutionContext) error {
	switch ec.Profile.Environment {
	case profile.EnvironmentHost:
		return l.launchHostCodex(ec)
	case profile.EnvironmentContainer:
		return l.launchDockerCodex(ctx, ec)
	default:
		return fmt.Errorf("unsupported environment: %q", ec.Profile.Environment)
	}
}

func (l *CodexLauncher) launchHostCodex(ec *pipeline.ExecutionContext) error {
	codexPath, err := exec.LookPath("codex")
	if err != nil {
		return fmt.Errorf("codex is not installed. Install Codex CLI: npm i -g @openai/codex")
	}

	fmt.Fprintf(os.Stderr, "Launching Codex in %s\n", ec.WorkDir)

	args := []string{"codex"}
	env := os.Environ()
	return syscall.Exec(codexPath, args, env)
}

func (l *CodexLauncher) launchDockerCodex(ctx context.Context, ec *pipeline.ExecutionContext) error {
	client := docker.NewShellClient(ec.Profile.EffectiveContainerRuntime())

	command := []string{"codex", "-a", "never"}

	envVars := buildContainerEnvVars(ec, "codex")
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
