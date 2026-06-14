package pathutil

import (
	"path/filepath"
	"strings"
)

// ExpandTilde expands a leading ~ in path using homeDir.
func ExpandTilde(path, homeDir string) string {
	if strings.HasPrefix(path, "~") {
		return filepath.Join(homeDir, strings.TrimPrefix(path, "~"))
	}
	return path
}
