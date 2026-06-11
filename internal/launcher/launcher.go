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

// podmanUserns returns "keep-id" for rootless Podman so that the host UID
// maps to the same UID inside the container, making bind-mounted files
// accessible with correct ownership.
func podmanUserns(runtime string) string {
	if runtime == "podman" {
		return "keep-id"
	}
	return ""
}

// buildContainerEnvVars creates the base set of environment variables for container execution.
func buildContainerEnvVars(ec *pipeline.ExecutionContext, tool string) map[string]string {
	envVars := toolinfo.ContainerEnvVarsFor(ec.EnvVars, tool, ec.ContainerEnv)

	if ec.SSHAgentReady {
		envVars["SSH_AUTH_SOCK"] = mount.SSHAgentContainerPath
	}

	// DOCKER_HOST is set by .aw_env.sh after devbox/nix initialization to
	// avoid early socket access that hangs on RHEL/SELinux hosts.

	if ec.Profile.EffectiveSkipDevboxInstall() {
		envVars["AW_SKIP_DEVBOX_INSTALL"] = "1"
	}
	if ec.Profile.EffectiveSkipMiseInstall() {
		envVars["AW_SKIP_MISE_INSTALL"] = "1"
	}

	return envVars
}
