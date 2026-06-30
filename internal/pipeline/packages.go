package pipeline

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/konono/aw/internal/profile"
)

// CollectPackages merges profile-level packages with workspace packages.txt
// files, deduplicating while preserving order.
// Optional workDirs specify workspace directories whose packages.txt files
// are also merged (after profile packages).
func CollectPackages(profilePkgs []string, workDirs ...string) []string {
	seen := make(map[string]bool)
	var result []string
	addPkg := func(pkg string) {
		pkg = strings.TrimSpace(pkg)
		if pkg != "" && !seen[pkg] && profile.ValidPackageName.MatchString(pkg) {
			seen[pkg] = true
			result = append(result, pkg)
		}
	}

	readPackagesFile := func(path string) {
		if data, err := os.ReadFile(path); err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				line = strings.TrimSpace(line)
				if line != "" && !strings.HasPrefix(line, "#") {
					addPkg(line)
				}
			}
		}
	}

	for _, pkg := range profilePkgs {
		addPkg(pkg)
	}

	for _, dir := range workDirs {
		readPackagesFile(filepath.Join(dir, "packages.txt"))
	}

	return result
}
