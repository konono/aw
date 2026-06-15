package reaper

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// CheckOrphanedReapers checks for spec files left by dead reapers and
// recovers resource-management tasks automatically.
func CheckOrphanedReapers(runtime string) {
	specs, err := filepath.Glob(filepath.Join(reaperDir(), "*.spec.json"))
	if err != nil || len(specs) == 0 {
		return
	}

	for _, specPath := range specs {
		name := strings.TrimSuffix(filepath.Base(specPath), ".spec.json")

		out, err := exec.Command(runtime, "inspect", name,
			"--format", "{{.State.Running}}").Output()

		switch {
		case err != nil:
			// Container does not exist → stale spec, clean up
			os.Remove(specPath)

		case strings.TrimSpace(string(out)) == "true":
			// Container still running → possibly another session
			fmt.Fprintf(os.Stderr, "[reaper] container still running (another session?): %s\n", name)
			fmt.Fprintf(os.Stderr, "  recover after stop: aw reaper recover %s\n", name)

		default:
			// Container exited → auto-recover resource tasks
			fmt.Fprintf(os.Stderr, "[reaper] recovering previous session cleanup...\n")
			if err := recoverReaper(specPath, name, true); err != nil {
				fmt.Fprintf(os.Stderr, "[reaper] recovery failed: %v\n", err)
				fmt.Fprintf(os.Stderr, "  manual recover: aw reaper recover %s\n", name)
				fmt.Fprintf(os.Stderr, "  discard: aw reaper discard %s\n", name)
			}
		}
	}
}

// recoverReaper executes tasks from a spec file. If resourceOnly is true,
// only kill_process and remove_file tasks are executed (not shell).
func recoverReaper(specPath, containerName string, resourceOnly bool) error {
	data, err := os.ReadFile(specPath)
	if err != nil {
		return fmt.Errorf("reading spec: %w", err)
	}
	var spec ReaperSpec
	if err := json.Unmarshal(data, &spec); err != nil {
		return fmt.Errorf("parsing spec: %w", err)
	}

	// Wait for container to finish (no-op if already exited)
	_ = exec.Command(spec.Runtime, "wait", containerName).Run()

	timeout := 60 * time.Second
	if spec.Timeout > 0 {
		timeout = time.Duration(spec.Timeout) * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	report := RunReport{
		StartedAt:     time.Now(),
		ContainerName: containerName,
	}

	diag := diagnoseContainer(spec.Runtime, containerName)
	report.ExitCode = diag.ExitCode
	report.ContainerDiag = diag

	for _, task := range spec.Tasks {
		if resourceOnly && task.Type == "shell" {
			continue
		}
		start := time.Now()
		err := executeTask(ctx, task, &spec)
		result := TaskResult{
			Type:     task.Type,
			Label:    task.Label,
			Duration: time.Since(start),
		}
		if err != nil {
			result.Status = "failed"
			result.Error = err.Error()
			fmt.Fprintf(os.Stderr, "  ✗ %s (%s): %v\n", task.Label, task.Type, err)
		} else {
			result.Status = "ok"
			fmt.Fprintf(os.Stderr, "  ✓ %s (%s)\n", task.Label, task.Type)
		}
		report.Tasks = append(report.Tasks, result)
	}

	if !spec.KeepContainer {
		_ = exec.Command(spec.Runtime, "rm", containerName).Run()
	} else {
		report.ContainerKept = true
	}

	report.FinishedAt = time.Now()
	writeReport(report)
	os.Remove(specPath)
	return nil
}
