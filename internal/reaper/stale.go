package reaper

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// CheckStaleContainer checks if a container with the same name already exists.
// If stopped, it removes it with a warning. If running, it returns an error.
func CheckStaleContainer(runtime, containerName string) error {
	out, err := exec.Command(runtime, "inspect", containerName,
		"--format", "{{.State.Running}}").Output()
	if err != nil {
		return nil // container does not exist
	}

	running := strings.TrimSpace(string(out))
	if running == "true" {
		return fmt.Errorf("container %s is already running; stop it first or choose a different profile", containerName)
	}

	fmt.Fprintf(os.Stderr, "[reaper] removing stale container: %s\n", containerName)
	_ = exec.Command(runtime, "rm", containerName).Run()
	return nil
}

// CheckStaleContainers warns about any stopped aw-* containers on disk.
func CheckStaleContainers(runtime string) {
	out, err := exec.Command(runtime, "ps", "-a",
		"--filter", "name=^aw-",
		"--filter", "status=exited",
		"--format", "{{.Names}} {{.Status}}").Output()
	if err != nil || len(strings.TrimSpace(string(out))) == 0 {
		return
	}
	fmt.Fprintf(os.Stderr, "[reaper] stale containers found:\n")
	fmt.Fprintf(os.Stderr, "%s", out)
	fmt.Fprintf(os.Stderr, "  remove: %s rm $(%s ps -a --filter name=^aw- --filter status=exited -q)\n", runtime, runtime)
	fmt.Fprintf(os.Stderr, "  inspect: %s logs <container-name>\n", runtime)
}
