package cmd

import (
	"context"

	"github.com/konono/aw/v4/internal/pipeline"
	"github.com/konono/aw/v4/internal/reaper"
)

// containerNameStage picks the name the container will run under.
//
// It is a stage rather than a step before the pipeline because the worktree
// stage is what points WorkDir at the worktree. Naming earlier reads the
// parent repository, so every worktree started from one repo would share a
// workspace segment — the case the workspace segment exists to tell apart.
type containerNameStage struct{}

func (s *containerNameStage) Name() string { return "container-name" }

func (s *containerNameStage) Run(_ context.Context, ec *pipeline.ExecutionContext) error {
	ec.ContainerName = generateContainerName(ec.ProfileName, ec.WorkDir)
	return reaper.CheckStaleContainer(ec.Profile.EffectiveContainerRuntime(), ec.ContainerName)
}
