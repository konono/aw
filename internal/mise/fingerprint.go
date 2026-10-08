// Package mise identifies the workspace inputs that drive `mise install`.
//
// aw build bakes a fingerprint of those inputs into the snapshot image. At
// launch the aw CLI fingerprints the workspace again and passes the result to
// the container, where the entrypoint compares the two strings: equal means the
// image already has exactly these tools, so the install can be skipped.
//
// Both sides call Fingerprint, so there is no second implementation in shell
// that could drift away from this one.
package mise

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// snapshotFiles are the config files the snapshot script copies into the image.
// The fingerprint covers exactly these, so it must stay in sync with
// internal/cmd/embed/snapshot.sh.tmpl.
var snapshotFiles = []string{"mise.toml", ".mise.toml"}

// extraInputGlobs match mise inputs that the snapshot does not copy and the
// fingerprint therefore cannot describe. Their presence disqualifies the
// workspace: aw falls back to running mise install rather than trusting a
// fingerprint that ignores half of what mise reads.
var extraInputGlobs = []string{
	"mise.lock",
	".tool-versions",
	"mise.*.toml",
	".mise.*.toml",
	"mise/config.toml",
	".mise/config.toml",
	"mise/config.*.toml",
	".mise/config.*.toml",
	".config/mise.toml",
	".config/mise/config.toml",
}

// Fingerprint hashes the mise config files a snapshot would bake in from dir.
//
// ok is false when the fingerprint cannot stand for the whole input set, either
// because dir has no mise config at all or because it has inputs outside the
// snapshot's reach. Callers must run mise install in that case.
func Fingerprint(dir string) (fp string, ok bool) {
	var covered []string
	h := sha256.New()

	for _, name := range snapshotFiles {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		// References to other files resolve relative to mise's own lookup
		// rules, so a config that includes anything is not self-describing.
		if hasIncludeDirective(data) {
			return "", false
		}
		covered = append(covered, name)
		// hash.Hash writes never fail.
		h.Write(fmt.Appendf(nil, "%s\x00%d\x00", name, len(data)))
		h.Write(data)
	}

	if len(covered) == 0 {
		return "", false
	}
	if hasExtraInputs(dir) {
		return "", false
	}

	return fmt.Sprintf("sha256:%x", h.Sum(nil)), true
}

// hasExtraInputs reports whether dir holds a mise input the snapshot ignores.
func hasExtraInputs(dir string) bool {
	for _, pattern := range extraInputGlobs {
		matches, err := filepath.Glob(filepath.Join(dir, pattern))
		if err != nil {
			// A malformed pattern is a programming error; treat the workspace
			// as unknown rather than silently trusting a partial fingerprint.
			return true
		}
		if len(matches) > 0 {
			return true
		}
	}
	return false
}

// hasIncludeDirective looks for a mise config that pulls in further files.
// It is deliberately loose: a false positive only costs one mise install run,
// while a false negative would skip installing tools the user asked for.
func hasIncludeDirective(data []byte) bool {
	for line := range strings.Lines(string(data)) {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[[include]]") || strings.HasPrefix(line, "[include]") {
			return true
		}
		if key, _, found := strings.Cut(line, "="); found && strings.TrimSpace(key) == "include" {
			return true
		}
	}
	return false
}
