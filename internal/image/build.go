package image

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/konono/aw/v4/internal/containerenv"
	"github.com/konono/aw/v4/internal/profile"
)

// PrepareBuildContext prepares a build context directory for docker build.
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

	ep := Entrypoint(pkgMgr)

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

	if err := os.WriteFile(filepath.Join(tmpDir, "aw-init.sh"), InitScript(), 0755); err != nil {
		cleanupFn()
		return "", nil, fmt.Errorf("writing aw-init.sh: %w", err)
	}

	if err := os.WriteFile(filepath.Join(tmpDir, "aw-deps.sh"), DepsScript(), 0755); err != nil {
		cleanupFn()
		return "", nil, fmt.Errorf("writing aw-deps.sh: %w", err)
	}

	if cenv.SessionLog {
		if err := writePtyLoggerBinaries(tmpDir); err != nil {
			cleanupFn()
			return "", nil, fmt.Errorf("building pty-logger: %w", err)
		}
	}

	if cenv.SockRelay {
		if err := writeSockRelayBinaries(tmpDir); err != nil {
			cleanupFn()
			return "", nil, fmt.Errorf("building sockrelay: %w", err)
		}
	}

	return tmpDir, cleanupFn, nil
}

// writePtyLoggerBinaries cross-compiles the pty-logger binary for both amd64
// and arm64 on the host and writes them to the Docker build context. The
// Dockerfile selects the correct binary at build time based on uname -m.
// This avoids running the Go toolchain under QEMU emulation in Docker.
func writePtyLoggerBinaries(buildDir string) error {
	if _, err := exec.LookPath("go"); err != nil {
		return fmt.Errorf("session_log requires the Go toolchain on the host: %w", err)
	}

	srcFS := PtyLoggerFS()

	tmpSrc, err := os.MkdirTemp("", "pty-logger-src-*")
	if err != nil {
		return fmt.Errorf("creating temp source dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpSrc) }()

	if err := fs.WalkDir(srcFS, ".", func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		outName := p
		if ext := filepath.Ext(p); ext == ".embed" {
			outName = p[:len(p)-len(ext)]
		}
		target := filepath.Join(tmpSrc, outName)
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		data, readErr := fs.ReadFile(srcFS, p)
		if readErr != nil {
			return fmt.Errorf("reading embedded %s: %w", p, readErr)
		}
		if filepath.Ext(outName) == ".go" {
			data = bytes.ReplaceAll(data, []byte("//go:build ignore\n\n"), nil)
		}
		return os.WriteFile(target, data, 0644)
	}); err != nil {
		return fmt.Errorf("extracting pty-logger source: %w", err)
	}

	for _, arch := range []string{"amd64", "arm64"} {
		outPath := filepath.Join(buildDir, "pty-logger-"+arch)
		cmd := exec.Command("go", "build", "-mod=vendor", "-ldflags=-s -w", "-o", outPath, ".")
		cmd.Dir = tmpSrc
		cmd.Env = append(os.Environ(), "GOOS=linux", "GOARCH="+arch, "CGO_ENABLED=0")
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("compiling pty-logger for linux/%s: %w", arch, err)
		}
	}
	return nil
}

// writeSockRelayBinaries cross-compiles the aw-sockrelay binary for both
// amd64 and arm64 and writes them to the Docker build context.
func writeSockRelayBinaries(buildDir string) error {
	if _, err := exec.LookPath("go"); err != nil {
		return fmt.Errorf("mount_zellij requires the Go toolchain on the host: %w", err)
	}

	// Find the module root by looking for go.mod relative to this package.
	modRoot, err := findModuleRoot()
	if err != nil {
		return err
	}

	for _, arch := range []string{"amd64", "arm64"} {
		outPath := filepath.Join(buildDir, "aw-sockrelay-"+arch)
		cmd := exec.Command("go", "build", "-ldflags=-s -w", "-o", outPath, "./cmd/sockrelay")
		cmd.Dir = modRoot
		cmd.Env = append(os.Environ(), "GOOS=linux", "GOARCH="+arch, "CGO_ENABLED=0")
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("compiling aw-sockrelay for linux/%s: %w", arch, err)
		}
	}
	return nil
}

func findModuleRoot() (string, error) {
	cmd := exec.Command("go", "env", "GOMOD")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("finding go module root: %w", err)
	}
	gomod := string(bytes.TrimSpace(out))
	if gomod == "" || gomod == os.DevNull {
		return "", fmt.Errorf("not inside a Go module")
	}
	return filepath.Dir(gomod), nil
}
