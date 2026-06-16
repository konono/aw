package pipeline

import (
	"os"
	"path/filepath"
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

	if pkgs := collectEnvPackages(ec.HomeDir, ec.Profile.Packages); pkgs != "" {
		envVars["AW_PACKAGES"] = pkgs
	}

	return envVars
}

func collectEnvPackages(homeDir string, profilePkgs []string) string {
	seen := make(map[string]bool)
	var result []string
	addPkg := func(pkg string) {
		pkg = strings.TrimSpace(pkg)
		if pkg != "" && !seen[pkg] {
			seen[pkg] = true
			result = append(result, pkg)
		}
	}

	packagesFile := filepath.Join(homeDir, ".config", "aw", "packages.txt")
	if data, err := os.ReadFile(packagesFile); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, "#") {
				addPkg(line)
			}
		}
	}

	for _, pkg := range profilePkgs {
		addPkg(pkg)
	}

	return strings.Join(result, ",")
}
