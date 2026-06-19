package platform

import (
	"runtime"
	"strings"
)

// ToContainerPath converts a host path to a path valid inside a Linux container.
// On Unix, the path is returned as-is since it is already a valid Linux path.
// On Windows, drive letters and backslashes are converted:
// C:\Users\foo → /c/Users/foo
func ToContainerPath(hostPath string) string {
	if runtime.GOOS != "windows" {
		return hostPath
	}
	return windowsToContainerPath(hostPath)
}

func windowsToContainerPath(p string) string {
	p = strings.ReplaceAll(p, `\`, "/")
	if len(p) >= 2 && p[1] == ':' {
		drive := strings.ToLower(string(p[0]))
		p = "/" + drive + p[2:]
	}
	return p
}
