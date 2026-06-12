package image

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/konono/aw/internal/containerenv"
	"github.com/konono/aw/internal/profile"
)

// PrepareBuildContext prepares a build context directory for docker build.
//
// If customDockerfilePath is non-empty, the directory containing that file is
// used as the build context — just like running "docker build" from that
// directory. Any files alongside the Dockerfile (entrypoint.sh, scripts, etc.)
// are available via COPY.
//
// If customDockerfilePath is empty, a temporary directory is created with the
// embedded Dockerfile template rendered with cenv and entrypoint.sh.
//
// The caller must call the returned cleanup function when done.
func PrepareBuildContext(customDockerfilePath string, osTemplate profile.OSTemplate, pkgMgr profile.PackageManager, cenv containerenv.Config) (dir string, cleanup func(), err error) {
	if customDockerfilePath != "" {
		absPath, err := filepath.Abs(customDockerfilePath)
		if err != nil {
			return "", nil, fmt.Errorf("resolving custom Dockerfile path: %w", err)
		}
		if _, err := os.Stat(absPath); err != nil {
			return "", nil, fmt.Errorf("reading custom Dockerfile %q: %w", customDockerfilePath, err)
		}
		return filepath.Dir(absPath), func() {}, nil
	}

	df, err := RenderDockerfile(osTemplate, pkgMgr, cenv)
	if err != nil {
		return "", nil, err
	}

	ep, err := RenderEntrypoint(pkgMgr, cenv)
	if err != nil {
		return "", nil, err
	}

	tmpDir, err := os.MkdirTemp("", "aw-build-*")
	if err != nil {
		return "", nil, fmt.Errorf("creating temp dir: %w", err)
	}

	cleanupFn := func() { _ = os.RemoveAll(tmpDir) }

	if err := os.WriteFile(filepath.Join(tmpDir, "Dockerfile"), df, 0644); err != nil {
		cleanupFn()
		return "", nil, fmt.Errorf("writing Dockerfile: %w", err)
	}

	if err := os.WriteFile(filepath.Join(tmpDir, "entrypoint.sh"), ep, 0755); err != nil {
		cleanupFn()
		return "", nil, fmt.Errorf("writing entrypoint.sh: %w", err)
	}

	return tmpDir, cleanupFn, nil
}
