package pipeline

import (
	"strings"

	"github.com/konono/aw/internal/mount"
	"github.com/konono/aw/internal/profile"
)

// ContainerEnvVars builds the environment variables passed into a container run.
func ContainerEnvVars(ec *ExecutionContext, tool string) map[string]string {
	envVars := baseEnvVars(ec.EnvVars, tool, ec.ContainerEnv)

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

	if pkgs := CollectPackages(ec.Profile.Packages, ec.OrigWorkDir); len(pkgs) > 0 {
		envVars["AW_PACKAGES"] = strings.Join(pkgs, ",")
	}

	return envVars
}
