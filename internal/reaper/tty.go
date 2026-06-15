package reaper

import (
	"fmt"
	"os"
	"strings"
)

// DetectTTY returns the path of the controlling terminal, or empty string.
func DetectTTY() string {
	path, err := os.Readlink("/proc/self/fd/2") // Linux
	if err != nil {
		path, err = os.Readlink("/dev/fd/2") // macOS
	}
	if err != nil || !strings.HasPrefix(path, "/dev/") {
		return ""
	}
	return path
}

func notifyUser(spec ReaperSpec, message string) {
	if spec.TTY == "" {
		return
	}
	tty, err := os.OpenFile(spec.TTY, os.O_WRONLY, 0)
	if err != nil {
		return
	}
	defer tty.Close() //nolint:errcheck

	_, _ = fmt.Fprintf(tty, "\n[aw reaper] %s\n", message)
	_, _ = fmt.Fprintf(tty, "  recover: aw reaper recover %s\n", spec.ContainerName)
	_, _ = fmt.Fprintf(tty, "  details: aw reaper show\n")
}
