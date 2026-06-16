package pipeline

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/konono/aw/internal/profile"
)

// CollectPackages reads ~/.config/aw/packages.txt and merges with profile-level
// packages, deduplicating while preserving order. File entries come first.
// Invalid package names are silently skipped.
func CollectPackages(homeDir string, profilePkgs []string) []string {
	seen := make(map[string]bool)
	var result []string
	addPkg := func(pkg string) {
		pkg = strings.TrimSpace(pkg)
		if pkg != "" && !seen[pkg] && profile.ValidPackageName.MatchString(pkg) {
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

	return result
}
