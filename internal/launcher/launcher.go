package launcher

import (
	"context"

	"github.com/konono/aw/internal/mount"
	"github.com/konono/aw/internal/pipeline"
	"github.com/konono/aw/internal/toolinfo"
)

// Launcher executes the final "run something" step of the pipeline.
type Launcher interface {
	Launch(ctx context.Context, ec *pipeline.ExecutionContext) error
}

// buildContainerEnvVars creates the base set of environment variables for container execution.
func buildContainerEnvVars(ec *pipeline.ExecutionContext, tool string) map[string]string {
	envVars := toolinfo.ContainerEnvVars(ec.EnvVars, tool)

	if ec.SSHAgentReady {
		envVars["SSH_AUTH_SOCK"] = mount.SSHAgentContainerPath
	}

	if ec.ContainerSockReady {
		envVars["DOCKER_HOST"] = "unix://" + mount.ContainerSockContainerPath
	}

	return envVars
}
