package image

import (
	"fmt"
	"os"
	"path/filepath"
)

// PrepareBuildContext prepares a build context directory for docker build.
//
// If customDockerfilePath is non-empty, the directory containing that file is
// used as the build context — just like running "docker build" from that
// directory. Any files alongside the Dockerfile (entrypoint.sh, scripts, etc.)
// are available via COPY.
//
// If customDockerfilePath is empty, a temporary directory is created with the
// embedded default Dockerfile and entrypoint.sh.
//
// The caller must call the returned cleanup function when done.
func PrepareBuildContext(customDockerfilePath string) (dir string, cleanup func(), err error) {
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

	tmpDir, err := os.MkdirTemp("", "aw-build-*")
	if err != nil {
		return "", nil, fmt.Errorf("creating temp dir: %w", err)
	}

	cleanupFn := func() { _ = os.RemoveAll(tmpDir) }

	if err := os.WriteFile(filepath.Join(tmpDir, "Dockerfile"), dockerfile, 0644); err != nil {
		cleanupFn()
		return "", nil, fmt.Errorf("writing Dockerfile: %w", err)
	}

	if err := os.WriteFile(filepath.Join(tmpDir, "entrypoint.sh"), entrypointSh, 0755); err != nil {
		cleanupFn()
		return "", nil, fmt.Errorf("writing entrypoint.sh: %w", err)
	}

	return tmpDir, cleanupFn, nil
}
