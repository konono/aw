package launcher

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sort"

	"github.com/konono/aw/v4/internal/docker"
	"github.com/konono/aw/v4/internal/pipeline"
	"github.com/konono/aw/v4/internal/platform"
	"github.com/konono/aw/v4/internal/profile"
	"github.com/konono/aw/v4/internal/reaper"
	"github.com/konono/aw/v4/internal/toolinfo"
)

// toolContainerCommands maps tool names to the command used in container mode.
// These differ from host mode because containers run with elevated permissions.
var toolContainerCommands = map[string][]string{
	"claude":   {"claude", "--permission-mode", "bypassPermissions"},
	"codex":    {"codex", "-a", "never"},
	"opencode": {"opencode"},
	"cursor":   {"agent", "--force"},
}

// ToolContainerCommand returns a copy of the container command for the given tool.
// Returns nil if the tool has no container command registered.
func ToolContainerCommand(tool string) []string {
	cmd, ok := toolContainerCommands[tool]
	if !ok {
		return nil
	}
	cp := make([]string, len(cmd))
	copy(cp, cmd)
	return cp
}

// ToolContainerCommandNames returns all tool names that have container commands registered.
func ToolContainerCommandNames() []string {
	names := make([]string, 0, len(toolContainerCommands))
	for name := range toolContainerCommands {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// toolPrintCommands maps tool names to the non-interactive (print mode) command
// used for background agent loops. Only tools with verified print mode + MCP
// support are listed here.
var toolPrintCommands = map[string][]string{
	"claude": {"claude", "-p", "--permission-mode", "bypassPermissions"},
	"cursor": {"agent", "-p", "--force", "--approve-mcps"},
}

// ToolPrintCommand returns a copy of the print mode command for the given tool.
// Returns nil if the tool does not support print mode.
func ToolPrintCommand(tool string) []string {
	cmd, ok := toolPrintCommands[tool]
	if !ok {
		return nil
	}
	cp := make([]string, len(cmd))
	copy(cp, cmd)
	return cp
}

// SupportsAgentLoop returns true if the tool supports non-interactive
// message-driven execution via print mode with MCP tools.
func SupportsAgentLoop(tool string) bool {
	_, ok := toolPrintCommands[tool]
	return ok
}

// AppendResumeFlags adds tool-specific flags to resume the most recent session.
func AppendResumeFlags(tool string, command []string) []string {
	switch tool {
	case "claude", "cursor", "opencode":
		return append(command, "--continue")
	default:
		return command
	}
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

	return platform.ExecReplace(binPath, []string{spec.Binary}, os.Environ())
}

func (l *ToolLauncher) launchContainer(_ context.Context, ec *pipeline.ExecutionContext) error {
	if _, ok := toolinfo.Lookup(l.Tool); !ok {
		return fmt.Errorf("unknown tool: %q", l.Tool)
	}

	runtime := ec.Profile.EffectiveContainerRuntime()
	client := docker.NewShellClient(runtime)

	command := ec.CommandOverride
	if len(command) == 0 {
		command = toolContainerCommands[l.Tool]
		if command == nil {
			command = []string{l.Tool}
		}
	}

	spec := reaper.BuildSpec(ec)
	runConfig := pipeline.ToolRunConfig(ec, runtime, l.Tool, command)
	spawnReaper := func() (*os.File, func(), error) {
		handle, err := reaper.Spawn(spec)
		if err != nil {
			return nil, nil, err
		}
		return handle.Write, handle.Abort, nil
	}

	if len(ec.CommandOverride) > 0 {
		return client.ExecRunForeground(ec.ContainerName, runConfig, spawnReaper)
	}
	return client.ExecRun(ec.ContainerName, runConfig, spawnReaper)
}
