package stage

import (
	"context"
	"fmt"

	"github.com/konono/aw/internal/launcher"
	"github.com/konono/aw/internal/pipeline"
	"github.com/konono/aw/internal/profile"
)

// LaunchStage selects and executes the appropriate launcher.
type LaunchStage struct {
	// LauncherFactory creates a Launcher for the given launch mode.
	// If nil, the default factory is used.
	LauncherFactory func(mode profile.LaunchMode) (launcher.Launcher, error)
}

func (s *LaunchStage) Name() string { return "launch" }

func (s *LaunchStage) Run(ctx context.Context, ec *pipeline.ExecutionContext) error {
	factory := s.LauncherFactory
	if factory == nil {
		factory = defaultLauncherFactory
	}

	l, err := factory(ec.Profile.Launch)
	if err != nil {
		return err
	}

	return l.Launch(ctx, ec)
}

func defaultLauncherFactory(mode profile.LaunchMode) (launcher.Launcher, error) {
	switch mode {
	case profile.LaunchShell:
		return &launcher.ShellLauncher{}, nil
	case profile.LaunchClaude:
		return &launcher.ToolLauncher{Tool: "claude"}, nil
	case profile.LaunchCodex:
		return &launcher.ToolLauncher{Tool: "codex"}, nil
	case profile.LaunchOpenCode:
		return &launcher.ToolLauncher{Tool: "opencode"}, nil
	default:
		return nil, fmt.Errorf("unknown launch mode: %q", mode)
	}
}
