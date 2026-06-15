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
