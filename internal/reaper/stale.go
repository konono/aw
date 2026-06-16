package reaper

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

// CheckStaleContainers warns about any stopped or orphaned running aw-* containers.
func CheckStaleContainers(runtime string) {
	checkStaleExited(runtime)
	checkStaleRunning(runtime)
}

func checkStaleExited(runtime string) {
	out, err := exec.Command(runtime, "ps", "-a",
		"--filter", "name=^aw-.*-[0-9]+$",
		"--filter", "status=exited",
		"--format", "{{.Names}} {{.Status}}").Output()
	if err != nil || len(strings.TrimSpace(string(out))) == 0 {
		return
	}
	fmt.Fprintf(os.Stderr, "[reaper] stale containers found:\n")
	fmt.Fprintf(os.Stderr, "%s", out)
	fmt.Fprintf(os.Stderr, "  remove: %s rm $(%s ps -a --filter name=^aw-.*-[0-9]+$ --filter status=exited -q)\n", runtime, runtime)
	fmt.Fprintf(os.Stderr, "  inspect: %s logs <container-name>\n", runtime)
}

func checkStaleRunning(runtime string) {
	out, err := exec.Command(runtime, "ps",
		"--filter", "name=^aw-.*-[0-9]+$",
		"--format", "{{.Names}}").Output()
	if err != nil || len(strings.TrimSpace(string(out))) == 0 {
		return
	}
	names := strings.Fields(strings.TrimSpace(string(out)))
	for _, name := range names {
		// Skip containers that have a spec file — a reaper is already tracking them.
		specPath := filepath.Join(ReaperDir(), name+".spec.json")
		if _, err := os.Stat(specPath); err == nil {
			continue
		}
		fmt.Fprintf(os.Stderr, "[reaper] container still running (another session?): %s\n", name)
		fmt.Fprintf(os.Stderr, "  recover after stop: aw reaper recover %s\n", name)
	}
}
