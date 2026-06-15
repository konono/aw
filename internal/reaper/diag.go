package reaper

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func diagnoseContainer(runtime, name string) *ContainerDiag {
	var diag ContainerDiag

	out, err := inspectContainerState(runtime, name, 3)
	if err != nil {
		diag.ExitCode = -1
		diag.Summary = "failed to inspect container state"
		return &diag
	}
	parts := strings.SplitN(strings.TrimSpace(string(out)), "|", 4)
	diag.ExitCode, _ = strconv.Atoi(parts[0])
	if len(parts) > 1 {
		diag.OOMKilled = parts[1] == "true"
	}
	if len(parts) > 2 {
		diag.ExitedAt = parts[2]
	}
	if len(parts) > 3 && parts[3] != "" {
		diag.Error = parts[3]
	}

	// exit 137 + no cgroup OOM → check for VM-level OOM
	// Only scan recent kernel messages (tail -200) to avoid false positives
	// from old OOM events in the VM's dmesg history.
	if diag.ExitCode == 137 && !diag.OOMKilled && runtime == "podman" {
		out, err := exec.Command("podman", "machine", "ssh",
			"dmesg | tail -200 | grep -ci 'oom\\|killed process'").Output()
		if err == nil && strings.TrimSpace(string(out)) != "0" {
			diag.VMOOMHint = true
		}
	}

	diag.Summary = summarizeExit(diag)
	return &diag
}

// inspectContainerState retrieves container state with retries.
// The container runtime may not reflect the final state immediately after
// the podman process exits, so retries are needed to avoid false negatives.
func inspectContainerState(runtime, name string, maxRetries int) ([]byte, error) {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		out, err := exec.Command(runtime, "inspect", name,
			"--format", "{{.State.ExitCode}}|{{.State.OOMKilled}}|{{.State.FinishedAt}}|{{.State.Error}}").Output()
		if err == nil {
			return out, nil
		}
		lastErr = err
		if i < maxRetries-1 {
			delay := 500 * time.Millisecond
			if i > 0 {
				delay = 2 * time.Second
			}
			time.Sleep(delay)
		}
	}
	return nil, lastErr
}

func summarizeExit(d ContainerDiag) string {
	switch {
	case d.ExitCode == 0:
		return "exited normally"
	case d.OOMKilled:
		return "container memory limit exceeded (OOM killed)"
	case d.ExitCode == 137 && d.VMOOMHint:
		return "possible Podman VM memory exhaustion (VM OOM)"
	case d.ExitCode == 137:
		return "killed by SIGKILL (exit 137)"
	case d.ExitCode == 143:
		return "terminated by SIGTERM (exit 143)"
	case d.ExitCode == 1:
		return "exited with error (exit 1)"
	case d.ExitCode == 127:
		return "command not found (exit 127)"
	case d.ExitCode == 126:
		return "permission denied (exit 126)"
	case d.ExitCode > 128:
		sig := d.ExitCode - 128
		return fmt.Sprintf("killed by signal %d (exit %d)", sig, d.ExitCode)
	default:
		return fmt.Sprintf("exited with code %d", d.ExitCode)
	}
}
