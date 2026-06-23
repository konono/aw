package linuxbin

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/konono/aw/internal/platform"
	"github.com/konono/aw/internal/update"
	"github.com/konono/aw/internal/version"
)

// Resolve returns the path to a Linux aw binary for the given architecture.
// On Linux hosts, it returns the current executable (goarch is ignored; the
// caller is expected to use the same arch as the container). On other hosts,
// it downloads from GitHub Releases (cached) or falls back to cross-compilation.
func Resolve(goarch string) (string, error) {
	if runtime.GOOS == "linux" {
		exe, err := os.Executable()
		if err != nil {
			return "", fmt.Errorf("resolving executable: %w", err)
		}
		return filepath.EvalSymlinks(exe)
	}

	ver := version.Version
	binDir := filepath.Join(platform.CacheDir(), "bin")

	// Prefer crossbuild when source is available (development builds may
	// contain unreleased features not in the GitHub release).
	hasSource := findModuleRootQuiet() != ""

	if hasSource {
		devCachePath := filepath.Join(binDir, fmt.Sprintf("aw-linux-%s-%s-dev", goarch, ver))
		if info, err := os.Stat(devCachePath); err == nil && !info.IsDir() {
			return devCachePath, nil
		}
		binData, buildErr := crossbuild(goarch)
		if buildErr == nil {
			if err := atomicWrite(devCachePath, binData); err != nil {
				return "", fmt.Errorf("caching crossbuilt binary: %w", err)
			}
			return devCachePath, nil
		}
		// Fall through to download if crossbuild fails.
	}

	releaseCachePath := filepath.Join(binDir, fmt.Sprintf("aw-linux-%s-%s", goarch, ver))
	if info, err := os.Stat(releaseCachePath); err == nil && !info.IsDir() {
		return releaseCachePath, nil
	}

	binData, dlErr := downloadLinuxBinary(goarch, ver)
	if dlErr == nil {
		if err := atomicWrite(releaseCachePath, binData); err != nil {
			return "", fmt.Errorf("caching downloaded binary: %w", err)
		}
		return releaseCachePath, nil
	}

	if !hasSource {
		binData, buildErr := crossbuild(goarch)
		if buildErr != nil {
			return "", fmt.Errorf("cannot obtain Linux binary (download: %v, crossbuild: %v)", dlErr, buildErr)
		}
		if err := atomicWrite(releaseCachePath, binData); err != nil {
			return "", fmt.Errorf("caching crossbuilt binary: %w", err)
		}
		return releaseCachePath, nil
	}

	return "", fmt.Errorf("cannot obtain Linux binary (download: %v)", dlErr)
}

func downloadLinuxBinary(goarch, ver string) ([]byte, error) {
	fmt.Fprintf(os.Stderr, "Downloading Linux binary for aw v%s...\n", ver)

	tag := "v" + ver
	release, err := update.FetchReleaseByTag(http.DefaultClient, tag)
	if err != nil {
		return nil, fmt.Errorf("fetching release %s: %w", tag, err)
	}

	assetURL, err := update.FindAssetURL(release, "linux", goarch)
	if err != nil {
		return nil, err
	}

	resp, err := http.Get(assetURL)
	if err != nil {
		return nil, fmt.Errorf("downloading: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	archiveData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading download: %w", err)
	}

	return update.ExtractBinaryFromTarGz(archiveData, "aw")
}

func crossbuild(goarch string) ([]byte, error) {
	fmt.Fprintln(os.Stderr, "Download failed, building Linux binary from source...")

	goPath, err := exec.LookPath("go")
	if err != nil {
		return nil, fmt.Errorf("go toolchain not found: %w", err)
	}

	moduleRoot, err := findModuleRoot()
	if err != nil {
		return nil, fmt.Errorf("finding module root: %w", err)
	}

	tmpFile, err := os.CreateTemp("", "aw-linux-*")
	if err != nil {
		return nil, err
	}
	tmpPath := tmpFile.Name()
	_ = tmpFile.Close()
	defer func() { _ = os.Remove(tmpPath) }()

	cmd := exec.Command(goPath, "build", "-o", tmpPath, ".")
	cmd.Dir = moduleRoot
	cmd.Env = append(os.Environ(),
		"GOOS=linux",
		"GOARCH="+goarch,
		"CGO_ENABLED=0",
	)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("go build: %w", err)
	}

	return os.ReadFile(tmpPath)
}

func findModuleRootQuiet() string {
	root, _ := findModuleRoot()
	return root
}

func findModuleRoot() (string, error) {
	exe, err := os.Executable()
	if err == nil {
		exe, _ = filepath.EvalSymlinks(exe)
		if root := walkUpForGoMod(filepath.Dir(exe)); root != "" {
			return root, nil
		}
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("cannot determine working directory: %w", err)
	}
	if root := walkUpForGoMod(cwd); root != "" {
		return root, nil
	}

	return "", fmt.Errorf("go.mod with module github.com/konono/aw not found")
}

func walkUpForGoMod(dir string) string {
	for {
		gomod := filepath.Join(dir, "go.mod")
		if data, err := os.ReadFile(gomod); err == nil {
			if strings.Contains(string(data), "module github.com/konono/aw") {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".aw-linux-bin-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() {
		if tmpPath != "" {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if runtime.GOOS != "windows" {
		if err := tmp.Chmod(0755); err != nil {
			_ = tmp.Close()
			return err
		}
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	tmpPath = ""
	return nil
}
