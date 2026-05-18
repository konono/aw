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
	envVars := make(map[string]string, len(ec.EnvVars)+5)
	for k, v := range ec.EnvVars {
		envVars[k] = v
	}

	hostHome := toolinfo.HomePath(tool, ec.HomeDir)
	containerDir := toolinfo.ContainerDir(tool)
	if hostHome != "" && containerDir != "" {
		envVars["AW_HOST_CONFIG_HOME"] = hostHome
		envVars["AW_CONTAINER_CONFIG_DIR"] = containerDir
	}
	if symlinks := toolinfo.DataSymlinks(tool); symlinks != "" {
		envVars["AW_DATA_SYMLINKS"] = symlinks
	}

	if ec.Profile.EffectiveSSHAgentForwarding() && !ec.Profile.EffectiveMountSSH() {
		envVars["SSH_AUTH_SOCK"] = mount.SSHAgentContainerPath
	}

	return envVars
}
