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

// EmbeddedBinary describes an embedded Go source tree that gets cross-compiled
// into the container image at build time.
type EmbeddedBinary struct {
	Name     string // human-readable name (e.g. "pty-logger", "aw-sockrelay")
	SrcFS    fs.FS  // embedded source tree
	BuildOpt string // extra go build flag (e.g. "-mod=vendor"), empty if none
}

// EmbeddedBinaries returns all registered embedded Go source trees.
// Used by tests to verify that all embedded sources compile correctly.
func EmbeddedBinaries() []EmbeddedBinary {
	return []EmbeddedBinary{
		{Name: "pty-logger", SrcFS: PtyLoggerFS(), BuildOpt: "-mod=vendor"},
		{Name: "aw-sockrelay", SrcFS: SockRelayFS()},
	}
}

// crossCompileEmbedded extracts an embedded Go source tree to a temp directory
// and cross-compiles it for the given OS/arch. Returns the output binary path.
func crossCompileEmbedded(srcFS fs.FS, name, buildOpt, outDir, goos, goarch string) (string, error) {
	tmpSrc, err := os.MkdirTemp("", name+"-src-*")
	if err != nil {
		return "", fmt.Errorf("creating temp source dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpSrc) }()

	if err := extractEmbeddedSource(srcFS, tmpSrc); err != nil {
		return "", fmt.Errorf("extracting %s source: %w", name, err)
	}

	outPath := filepath.Join(outDir, name+"-"+goarch)
	args := []string{"build", "-ldflags=-s -w", "-o", outPath}
	if buildOpt != "" {
		args = append(args[:1], append([]string{buildOpt}, args[1:]...)...)
	}
	args = append(args, ".")

	cmd := exec.Command("go", args...)
	cmd.Dir = tmpSrc
	cmd.Env = append(os.Environ(), "GOOS="+goos, "GOARCH="+goarch, "CGO_ENABLED=0")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("compiling %s for %s/%s: %w", name, goos, goarch, err)
	}
	return outPath, nil
}

func extractEmbeddedSource(srcFS fs.FS, destDir string) error {
	return fs.WalkDir(srcFS, ".", func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		outName := p
		if ext := filepath.Ext(p); ext == ".embed" {
			outName = p[:len(p)-len(ext)]
		}
		target := filepath.Join(destDir, outName)
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		data, err := fs.ReadFile(srcFS, p)
		if err != nil {
			return fmt.Errorf("reading embedded %s: %w", p, err)
		}
		if filepath.Ext(outName) == ".go" {
			data = bytes.ReplaceAll(data, []byte("//go:build ignore\n\n"), nil)
		}
		return os.WriteFile(target, data, 0644)
	})
}

func writePtyLoggerBinaries(buildDir string) error {
	if _, err := exec.LookPath("go"); err != nil {
		return fmt.Errorf("session_log requires the Go toolchain on the host: %w", err)
	}
	for _, arch := range []string{"amd64", "arm64"} {
		if _, err := crossCompileEmbedded(PtyLoggerFS(), "pty-logger", "-mod=vendor", buildDir, "linux", arch); err != nil {
			return err
		}
	}
	return nil
}

func writeSockRelayBinaries(buildDir string) error {
	if _, err := exec.LookPath("go"); err != nil {
		return fmt.Errorf("mount_zellij requires the Go toolchain on the host: %w", err)
	}
	for _, arch := range []string{"amd64", "arm64"} {
		if _, err := crossCompileEmbedded(SockRelayFS(), "aw-sockrelay", "", buildDir, "linux", arch); err != nil {
			return err
		}
	}
	return nil
}

