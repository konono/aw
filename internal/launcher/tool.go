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
	"github.com/konono/aw/internal/toolinfo"
)

// toolContainerCommands maps tool names to the command used in container mode.
// These differ from host mode because containers run with elevated permissions.
var toolContainerCommands = map[string][]string{
	"claude":   {"claude", "--permission-mode", "bypassPermissions"},
	"codex":    {"codex", "-a", "never"},
	"opencode": {"opencode"},
}

// ToolLauncher is a data-driven launcher for any registered tool.
type ToolLauncher struct {
	Tool string
}

func (l *ToolLauncher) Launch(ctx context.Context, ec *pipeline.ExecutionContext) error {
	switch ec.Profile.Environment {
	case profile.EnvironmentHost:
		return l.launchHost(ec)
	case profile.EnvironmentContainer:
		return l.launchContainer(ctx, ec)
	default:
		return fmt.Errorf("unsupported environment: %q", ec.Profile.Environment)
	}
}

func (l *ToolLauncher) launchHost(ec *pipeline.ExecutionContext) error {
	spec, ok := toolinfo.Lookup(l.Tool)
	if !ok {
		return fmt.Errorf("unknown tool: %q", l.Tool)
	}

	binPath, err := exec.LookPath(spec.Binary)
	if err != nil {
		return fmt.Errorf("%s is not installed. %s", spec.Binary, spec.InstallHint)
	}

	fmt.Fprintf(os.Stderr, "Launching %s in %s\n", spec.DisplayName, ec.WorkDir)

	return syscall.Exec(binPath, []string{spec.Binary}, os.Environ())
}

func (l *ToolLauncher) launchContainer(ctx context.Context, ec *pipeline.ExecutionContext) error {
	if _, ok := toolinfo.Lookup(l.Tool); !ok {
		return fmt.Errorf("unknown tool: %q", l.Tool)
	}

	client := docker.NewShellClient(ec.Profile.EffectiveContainerRuntime())

	command := toolContainerCommands[l.Tool]
	if command == nil {
		command = []string{l.Tool}
	}

	envVars := buildContainerEnvVars(ec, l.Tool)
	envVars["HOST_WORKSPACE"] = ec.WorkDir

	runConfig := docker.RunConfig{
		ImageName:    ec.DockerImage,
		Mounts:       ec.DockerMounts,
		EnvVars:      envVars,
		WorkDir:      ec.WorkDir,
		Command:      command,
		SecurityOpts: ec.DockerSecurityOpts,
		CapAdd:       ec.DockerCapAdd,
	}

	return client.Run(ctx, runConfig)
}
