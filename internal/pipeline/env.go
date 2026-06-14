package pipeline

import (
	"github.com/konono/aw/internal/mount"
	"github.com/konono/aw/internal/profile"
	"github.com/konono/aw/internal/toolinfo"
)

// ContainerEnvVars builds the environment variables passed into a container run.
func ContainerEnvVars(ec *ExecutionContext, tool string) map[string]string {
	envVars := toolinfo.ContainerEnvVarsFor(ec.EnvVars, tool, ec.ContainerEnv)

	if ec.SSHAgentReady {
		envVars["SSH_AUTH_SOCK"] = mount.SSHAgentContainerPath
	}

	if ec.GhTokenValue != "" {
		envVars["GITHUB_TOKEN"] = ec.GhTokenValue
	}

	if ec.Profile.EffectiveSkipMiseInstall() {
		envVars["AW_SKIP_MISE_INSTALL"] = "1"
	}

	if ec.Profile.EffectivePackageManager() == profile.PackageManagerDevbox && ec.Profile.EffectiveSkipDevboxInstall() {
		envVars["AW_SKIP_DEVBOX_INSTALL"] = "1"
	}

	return envVars
}
