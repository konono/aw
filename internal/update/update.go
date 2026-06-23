package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const binaryName = "aw"

// Updater holds dependencies for the update workflow.
type Updater struct {
	HTTPClient     HTTPClient
	CurrentVersion string
	GOOS           string
	GOARCH         string
	Stderr         io.Writer
	// ExecPath overrides the executable path detection (for testing).
	ExecPath string
}

// Run is the package-level entry point called from cmd.Run().
func Run(currentVersion string) error {
	u := &Updater{
		HTTPClient:     http.DefaultClient,
		CurrentVersion: currentVersion,
		GOOS:           runtime.GOOS,
		GOARCH:         runtime.GOARCH,
		Stderr:         os.Stderr,
	}
	return u.Execute()
}

// Execute performs the update workflow.
func (u *Updater) Execute() error {
	_, _ = fmt.Fprintln(u.Stderr, "Checking for updates...")

	// 1. Fetch latest release
	release, err := FetchLatestRelease(u.HTTPClient)
	if err != nil {
		return fmt.Errorf("checking latest release: %w", err)
	}

	latestVersion := strings.TrimPrefix(release.TagName, "v")

	// 2. Compare versions
	newer, err := isNewer(latestVersion, u.CurrentVersion)
	if err != nil {
		return fmt.Errorf("comparing versions: %w", err)
	}
	if !newer {
		_, _ = fmt.Fprintf(u.Stderr, "aw %s is already the latest version.\n", u.CurrentVersion)
		return nil
	}

	_, _ = fmt.Fprintf(u.Stderr, "Updating aw: %s → %s\n", u.CurrentVersion, latestVersion)

	// 3. Find asset URL
	assetURL, err := FindAssetURL(release, u.GOOS, u.GOARCH)
	if err != nil {
		return err
	}

	// 4. Download archive
	_, _ = fmt.Fprintf(u.Stderr, "Downloading for %s/%s...\n", u.GOOS, u.GOARCH)
	archiveData, err := u.download(assetURL)
	if err != nil {
		return fmt.Errorf("downloading release: %w", err)
	}

	// 5. Extract binary
	targetBinary := binaryName
	if u.GOOS == "windows" {
		targetBinary = binaryName + ".exe"
	}
	binaryData, err := extractBinary(archiveData, u.GOOS, targetBinary)
	if err != nil {
		return fmt.Errorf("extracting binary: %w", err)
	}

	// 6. Determine current binary path
	targetPath, err := u.executablePath()
	if err != nil {
		return fmt.Errorf("determining executable path: %w", err)
	}

	// 7. Replace binary
	if err := replaceBinary(targetPath, binaryData); err != nil {
		return fmt.Errorf("replacing binary: %w", err)
	}

	_, _ = fmt.Fprintln(u.Stderr, "Updated successfully! Run 'aw --version' to verify.")
	return nil
}

// download fetches data from the given URL.
func (u *Updater) download(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := u.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// extractBinary extracts the binary from an archive.
// For Windows (zip format), it looks for targetBinary in a zip archive.
// For other platforms (tar.gz), it looks in a tar.gz archive.
func extractBinary(archiveData []byte, goos, targetBinary string) ([]byte, error) {
	if goos == "windows" {
		return extractBinaryFromZip(archiveData, targetBinary)
	}
	return ExtractBinaryFromTarGz(archiveData, targetBinary)
}

func ExtractBinaryFromTarGz(archiveData []byte, targetBinary string) ([]byte, error) {
	gr, err := gzip.NewReader(bytes.NewReader(archiveData))
	if err != nil {
		return nil, fmt.Errorf("opening gzip: %w", err)
	}
	defer func() { _ = gr.Close() }()

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading tar: %w", err)
		}

		if filepath.Base(hdr.Name) == targetBinary {
			data, err := io.ReadAll(tr)
			if err != nil {
				return nil, fmt.Errorf("reading binary from archive: %w", err)
			}
			return data, nil
		}
	}

	return nil, fmt.Errorf("binary %q not found in archive", targetBinary)
}

func extractBinaryFromZip(archiveData []byte, targetBinary string) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(archiveData), int64(len(archiveData)))
	if err != nil {
		return nil, fmt.Errorf("opening zip: %w", err)
	}

	for _, f := range zr.File {
		if filepath.Base(f.Name) == targetBinary {
			rc, err := f.Open()
			if err != nil {
				return nil, fmt.Errorf("opening file in zip: %w", err)
			}
			defer func() { _ = rc.Close() }()
			data, err := io.ReadAll(rc)
			if err != nil {
				return nil, fmt.Errorf("reading binary from zip: %w", err)
			}
			return data, nil
		}
	}

	return nil, fmt.Errorf("binary %q not found in archive", targetBinary)
}

// executablePath returns the resolved path of the currently running binary.
func (u *Updater) executablePath() (string, error) {
	if u.ExecPath != "" {
		return u.ExecPath, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(exe)
}

// replaceBinary replaces the binary at targetPath with newBinary.
// On Unix this is an atomic rename; on Windows a two-step move is used
// because the OS locks the running executable.
func replaceBinary(targetPath string, newBinary []byte) error {
	dir := filepath.Dir(targetPath)
	tmpFile, err := os.CreateTemp(dir, binaryName+".update.*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	// Clean up on any failure
	defer func() {
		if tmpPath != "" {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmpFile.Write(newBinary); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("writing temp file: %w", err)
	}
	if runtime.GOOS != "windows" {
		if err := tmpFile.Chmod(0755); err != nil {
			_ = tmpFile.Close()
			return fmt.Errorf("setting permissions: %w", err)
		}
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("closing temp file: %w", err)
	}

	// On Windows the running binary holds a file lock, so os.Rename
	// over it fails. Move the old binary aside first, then put the
	// new one in place.
	if runtime.GOOS == "windows" {
		oldPath := targetPath + ".old"
		_ = os.Remove(oldPath)
		if err := os.Rename(targetPath, oldPath); err != nil {
			return fmt.Errorf("moving old binary aside: %w", err)
		}
		if err := os.Rename(tmpPath, targetPath); err != nil {
			if rbErr := os.Rename(oldPath, targetPath); rbErr != nil {
				return fmt.Errorf("placing new binary: %w (rollback also failed: %v)", err, rbErr)
			}
			return fmt.Errorf("placing new binary: %w", err)
		}
		// The old binary is still held by this process, so Remove will
		// likely fail on Windows. The leftover .old is cleaned up at the
		// start of the next update run.
		_ = os.Remove(oldPath)
		tmpPath = ""
		return nil
	}

	// Unix: atomic rename (overwrites in place).
	if err := os.Rename(tmpPath, targetPath); err != nil {
		return fmt.Errorf("renaming: %w", err)
	}

	tmpPath = ""
	return nil
}
