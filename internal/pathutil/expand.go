package pathutil

import (
	"path/filepath"
	"strings"
)

// ExpandTilde expands a leading ~/ in path using homeDir.
// Other ~ forms (bare ~, ~username) are left unchanged.
func ExpandTilde(path, homeDir string) string {
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(homeDir, path[2:])
	}
	prefix := "~" + string(filepath.Separator)
	if strings.HasPrefix(path, prefix) {
		return filepath.Join(homeDir, strings.TrimPrefix(path, prefix))
	}
	return path
}
