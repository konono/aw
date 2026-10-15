package cmd

import (
	"crypto/rand"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/konono/aw/v4/internal/gitroot"
)

// containerNameUnsafe matches everything podman/docker disallows in a
// container name. Underscores are legal but are folded into hyphens anyway so
// that a workspace like 20261009_aap_500 reads the same as a hyphenated one.
var containerNameUnsafe = regexp.MustCompile(`[^a-z0-9.-]`)

// generateContainerName builds the name aw gives the container it starts:
//
//	aw-<profile>-<workspace>-<random>
//
// workspace is the basename of the git repository holding workDir, falling
// back to workDir's own basename outside a repository. Without it every
// container from the same profile looked identical in podman ps, which is the
// common case when several are running at once.
//
// The random suffix, not a timestamp, keeps the name stable in length and
// unique when the same profile starts twice in the same instant; podman ps
// already reports the creation time.
func generateContainerName(profileName, workDir string) string {
	parts := []string{"aw", sanitizeNameSegment(profileName)}
	if ws := sanitizeNameSegment(workspaceLabel(workDir)); ws != "" {
		parts = append(parts, ws)
	}
	parts = append(parts, randomNameSuffix())
	return strings.Join(parts, "-")
}

// workspaceLabel returns the human-meaningful name of the workspace at dir.
func workspaceLabel(dir string) string {
	if dir == "" {
		return ""
	}
	if root, err := gitroot.FindRootFrom(dir); err == nil && root != "" {
		dir = root
	}
	base := filepath.Base(dir)
	if base == "." || base == string(filepath.Separator) {
		return ""
	}
	return base
}

// sanitizeNameSegment reduces s to characters a container name accepts.
func sanitizeNameSegment(s string) string {
	s = containerNameUnsafe.ReplaceAllString(strings.ToLower(s), "-")
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return strings.Trim(s, "-.")
}

// randomNameSuffix returns 6 hex characters, enough to separate concurrent
// containers without making the name unreadable.
func randomNameSuffix() string {
	b := make([]byte, 3)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand does not fail in practice; a fixed suffix still yields a
		// usable name, and the runtime rejects an outright collision.
		return "000000"
	}
	return fmt.Sprintf("%x", b)
}
