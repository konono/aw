package stage

import (
	"context"
	"crypto/sha256"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/konono/aw/v4/internal/containerenv"
	"github.com/konono/aw/v4/internal/gitroot"
	"github.com/konono/aw/v4/internal/image"
	"github.com/konono/aw/v4/internal/pathutil"
	"github.com/konono/aw/v4/internal/pipeline"
	"github.com/konono/aw/v4/internal/profile"
	"github.com/konono/aw/v4/internal/toolinfo"
)

type buildInputs struct {
	toolPkg           string
	toolInstallScript string
	extraPackages     string
}

func resolveBuildInputs(customDockerfile string, tool string, pkgMgr profile.PackageManager, ec *pipeline.ExecutionContext) buildInputs {
	var bi buildInputs
	if customDockerfile != "" {
		return bi
	}
	if pkgMgr == profile.PackageManagerDevbox {
		bi.toolPkg = toolinfo.DevboxPkg(tool)
	} else {
		bi.toolInstallScript = toolinfo.InstallScript(tool)
	}
	packages := pipeline.CollectPackages(ec.Profile.Packages, ec.OrigWorkDir)
	if len(packages) > 0 {
		bi.extraPackages = strings.Join(packages, " ")
	}
	return bi
}

func computeImageTag(buildDir, customDockerfile string, ec *pipeline.ExecutionContext, cenv containerenv.Config, pkgMgr profile.PackageManager, bi buildInputs) string {
	hashSource := filepath.Join(buildDir, "Dockerfile")
	if customDockerfile != "" {
		hashSource = customDockerfile
	}

	hashInput := ""
	if dfBytes, err := os.ReadFile(hashSource); err == nil {
		hashInput = string(dfBytes)
	}
	if epBytes, err := os.ReadFile(filepath.Join(buildDir, "entrypoint.sh")); err == nil {
		hashInput += "\n" + string(epBytes)
	}
	if initBytes, err := os.ReadFile(filepath.Join(buildDir, "aw-init.sh")); err == nil {
		hashInput += "\n" + string(initBytes)
	}
	hashInput += "\n" + string(ec.Profile.EffectiveOS())
	hashInput += "\n" + cenv.User

	if customDockerfile == "" {
		hashInput += "\n" + bi.toolPkg
		hashInput += "\n" + bi.toolInstallScript
		hashInput += "\n" + string(pkgMgr)
		hashInput += "\n" + toolinfo.GhCLIVersion
		hashInput += "\n" + toolinfo.MiseVersion
	}
	if bi.extraPackages != "" {
		hashInput += "\n" + bi.extraPackages
	}

	if ec.Profile.CACert != "" {
		certPath := pathutil.ExpandTilde(ec.Profile.CACert, ec.HomeDir)
		if certData, err := os.ReadFile(certPath); err == nil {
			hashInput += "\n" + string(certData)
		}
	}
	for _, k := range slices.Sorted(maps.Keys(ec.Profile.BuildEnv)) {
		hashInput += "\n" + k + "=" + ec.Profile.BuildEnv[k]
	}

	if hashInput != "" {
		hash := fmt.Sprintf("%x", sha256.Sum256([]byte(hashInput)))[:12]
		return fmt.Sprintf("%s:%s", defaultImageName, hash)
	}
	return defaultImageName
}

func collectBuildArgs(customDockerfile string, ec *pipeline.ExecutionContext, bi buildInputs) map[string]string {
	buildArgs := map[string]string{}
	if customDockerfile == "" {
		buildArgs["AW_GH_VERSION"] = toolinfo.GhCLIVersion
		buildArgs["AW_MISE_VERSION"] = toolinfo.MiseVersion
	}
	if bi.toolPkg != "" {
		buildArgs["AW_TOOL_PKG"] = bi.toolPkg
	}
	if bi.toolInstallScript != "" {
		buildArgs["AW_TOOL_INSTALL_SCRIPT"] = bi.toolInstallScript
	}
	if bi.extraPackages != "" && customDockerfile == "" {
		buildArgs["AW_EXTRA_PACKAGES"] = bi.extraPackages
	}
	for k, v := range ec.Profile.BuildEnv {
		buildArgs[k] = v
	}
	return buildArgs
}

func (s *DockerStage) buildImage(ctx context.Context, ec *pipeline.ExecutionContext, cenv containerenv.Config) (string, error) {
	customDockerfile := ""
	if ec.Profile.Dockerfile != "" {
		resolved, err := resolveDockerfilePath(ec.Profile.Dockerfile)
		if err != nil {
			return "", fmt.Errorf("resolving dockerfile path: %w", err)
		}
		customDockerfile = resolved
	}

	tool := ec.Profile.EffectiveTool()
	osTemplate := ec.Profile.EffectiveOS()
	pkgMgr := ec.Profile.EffectivePackageManager()

	buildDir, cleanup, err := image.PrepareBuildContext(customDockerfile, osTemplate, pkgMgr, cenv)
	if err != nil {
		return "", fmt.Errorf("preparing build context: %w", err)
	}
	defer cleanup()

	caCertInBuildDir, err := copyCACert(buildDir, customDockerfile, ec)
	if err != nil {
		return "", err
	}
	if caCertInBuildDir != "" {
		defer func() { _ = os.Remove(caCertInBuildDir) }()
	}

	bi := resolveBuildInputs(customDockerfile, tool, pkgMgr, ec)
	imageName := computeImageTag(buildDir, customDockerfile, ec, cenv, pkgMgr, bi)
	buildArgs := collectBuildArgs(customDockerfile, ec, bi)

	if customDockerfile != "" {
		fmt.Fprintf(os.Stderr, "Building Docker image '%s' (custom Dockerfile: %s)...\n", imageName, ec.Profile.Dockerfile)
	} else {
		fmt.Fprintf(os.Stderr, "Building Docker image '%s' (os: %s)...\n", imageName, osTemplate)
	}
	if err := s.DockerClient.Build(ctx, imageName, buildDir, customDockerfile, buildArgs, ec.NoCache); err != nil {
		return "", fmt.Errorf("building image: %w", err)
	}

	return imageName, nil
}

func copyCACert(buildDir, customDockerfile string, ec *pipeline.ExecutionContext) (caCertInBuildDir string, err error) {
	if ec.Profile.CACert == "" {
		return "", nil
	}
	certPath := pathutil.ExpandTilde(ec.Profile.CACert, ec.HomeDir)
	data, err := os.ReadFile(certPath)
	if err != nil {
		return "", fmt.Errorf("reading ca_cert %q: %w", ec.Profile.CACert, err)
	}
	dst := filepath.Join(buildDir, "ca-cert.pem")
	if err := os.WriteFile(dst, data, 0644); err != nil {
		return "", fmt.Errorf("copying ca_cert to build context: %w", err)
	}
	if customDockerfile != "" {
		return dst, nil
	}
	return "", nil
}

func resolveDockerfilePath(dockerfilePath string) (string, error) {
	if filepath.IsAbs(dockerfilePath) {
		return dockerfilePath, nil
	}

	repoRoot, err := gitroot.FindRoot()
	if err != nil {
		return "", fmt.Errorf("finding git root to resolve dockerfile path: %w", err)
	}
	return filepath.Join(repoRoot, dockerfilePath), nil
}
