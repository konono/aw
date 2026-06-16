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
	specs, err := filepath.Glob(filepath.Join(ReaperDir(), "*.spec.json"))
	if err != nil || len(specs) == 0 {
		return
	}

	for _, specPath := range specs {
		name := strings.TrimSuffix(filepath.Base(specPath), ".spec.json")

		out, err := exec.Command(runtime, "inspect", name,
			"--format", "{{.State.Running}}").Output()

		switch {
		case err != nil:
			// Container gone — still run resource cleanup from spec
			fmt.Fprintf(os.Stderr, "[reaper] recovering cleanup for removed container: %s\n", name)
			if recErr := recoverReaper(specPath, name, true); recErr != nil {
				fmt.Fprintf(os.Stderr, "[reaper] recovery failed: %v\n", recErr)
				fmt.Fprintf(os.Stderr, "  manual recover: aw reaper recover %s\n", name)
				fmt.Fprintf(os.Stderr, "  discard: aw reaper discard %s\n", name)
			}

		case strings.TrimSpace(string(out)) == "true":
			// Container still running — another session's reaper is managing it.
			continue

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

// RecoverFromSpec executes pending tasks from a spec file (including shell).
// Tasks that already succeeded in the latest report are skipped.
// Used by `aw reaper recover` for manual full recovery.
func RecoverFromSpec(specPath, containerName string) error {
	return recoverReaper(specPath, containerName, false)
}

func recoverReaper(specPath, containerName string, resourceOnly bool) error {
	data, err := os.ReadFile(specPath)
	if err != nil {
		return fmt.Errorf("reading spec: %w", err)
	}
	var spec ReaperSpec
	if err := json.Unmarshal(data, &spec); err != nil {
		return fmt.Errorf("parsing spec: %w", err)
	}

	timeout := time.Duration(DefaultReaperTimeout) * time.Second
	if spec.Timeout > 0 {
		timeout = time.Duration(spec.Timeout) * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Wait for container to finish (no-op if already exited);
	// use timeout context to avoid blocking indefinitely.
	_ = exec.CommandContext(ctx, spec.Runtime, "wait", containerName).Run()

	report := RunReport{
		StartedAt:     time.Now(),
		ContainerName: containerName,
		Tasks:         make([]TaskResult, 0, len(spec.Tasks)),
	}

	diag := diagnoseContainer(spec.Runtime, containerName)
	report.ExitCode = diag.ExitCode
	report.ContainerDiag = diag

	// Collect diagnostic dump if container is still present
	report.DumpPath = collectDump(spec.Runtime, containerName, diag.ExitCode, spec.CollectLogs)

	succeeded := succeededTaskKeys(containerName)

	for _, task := range spec.Tasks {
		if resourceOnly && task.Type == "shell" {
			continue
		}
		if succeeded != nil && succeeded[taskKey(task)] {
			report.Tasks = append(report.Tasks, TaskResult{
				Type:   task.Type,
				Label:  task.Label,
				Status: "skipped",
			})
			fmt.Fprintf(os.Stderr, "  - %s (%s): skipped (already succeeded)\n", task.Label, task.Type)
			continue
		}
		start := time.Now()
		err := executeTask(ctx, task, &spec)
		result := TaskResult{
			Type:     task.Type,
			Label:    task.Label,
			Duration: time.Since(start),
		}
		if ctx.Err() == context.DeadlineExceeded {
			result.Status = "timeout"
			result.Error = "reaper timeout exceeded"
			fmt.Fprintf(os.Stderr, "  ✗ %s (%s): timeout\n", task.Label, task.Type)
			report.Tasks = append(report.Tasks, result)
			break
		} else if err != nil {
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
	writeReport(report, spec.ReportRetention)

	var failedTasks []string
	for _, t := range report.Tasks {
		if t.Status != "ok" && t.Status != "skipped" {
			failedTasks = append(failedTasks, t.Label)
		}
	}
	if len(failedTasks) > 0 {
		return fmt.Errorf("%d task(s) failed: %s", len(failedTasks), strings.Join(failedTasks, ", "))
	}
	if err := os.Remove(specPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing spec: %w", err)
	}
	return nil
}
