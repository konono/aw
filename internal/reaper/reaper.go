package reaper

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/konono/aw/internal/platform"
)

// Run is the entry point for the reaper subprocess.
// It reads a ReaperSpec from a pipe (fd 3 on Unix, Stdin on Windows),
// waits for EOF (= container exit), then executes tasks and cleans up.
func Run() int {
	pipe := platform.ReaperPipe()
	if pipe == nil {
		log.Print("reaper: pipe not available")
		return 1
	}
	reader := bufio.NewReader(pipe)

	line, err := reader.ReadBytes('\n')
	if err != nil {
		log.Printf("reaper: reading spec: %v", err)
		return 1
	}

	var spec ReaperSpec
	if err := json.Unmarshal(line, &spec); err != nil {
		log.Printf("reaper: parsing spec: %v", err)
		return 1
	}

	specPath := saveSpec(spec)

	// Wait for EOF = wrapper process exited (pipe write side closed).
	// The container may still be running if the wrapper was killed externally.
	_, _ = io.Copy(io.Discard, reader)
	_ = pipe.Close()

	// If the container is still running (wrapper killed but container survived),
	// wait for the container to actually exit before proceeding with cleanup.
	if isContainerRunning(spec.Runtime, spec.ContainerName) {
		log.Printf("reaper: container %s still running, waiting for exit", spec.ContainerName)
		if err := exec.Command(spec.Runtime, "wait", spec.ContainerName).Run(); err != nil {
			log.Printf("reaper: wait error for %s: %v, re-checking state", spec.ContainerName, err)
			if isContainerRunning(spec.Runtime, spec.ContainerName) {
				log.Printf("reaper: container %s still running after wait failure, retrying wait", spec.ContainerName)
				_ = exec.Command(spec.Runtime, "wait", spec.ContainerName).Run()
			}
		}
	}

	timeout := time.Duration(DefaultReaperTimeout) * time.Second
	if spec.Timeout > 0 {
		timeout = time.Duration(spec.Timeout) * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	report := RunReport{
		StartedAt:     time.Now(),
		ContainerName: spec.ContainerName,
		Tasks:         make([]TaskResult, 0, len(spec.Tasks)),
	}

	// Diagnose container exit state
	diag := diagnoseContainer(spec.Runtime, spec.ContainerName)
	report.ExitCode = diag.ExitCode
	report.ContainerDiag = diag

	// Collect diagnostic dump before container removal
	report.DumpPath = collectDump(spec.Runtime, spec.ContainerName, diag.ExitCode, spec.CollectLogs)

	// Execute tasks
	for _, task := range spec.Tasks {
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
			report.Tasks = append(report.Tasks, result)
			break
		} else if err != nil {
			result.Status = "failed"
			result.Error = err.Error()
		} else {
			result.Status = "ok"
		}
		report.Tasks = append(report.Tasks, result)
	}

	// Remove container unless keep mode
	if !spec.KeepContainer {
		_ = exec.Command(spec.Runtime, "rm", spec.ContainerName).Run()
	} else {
		report.ContainerKept = true
	}

	report.FinishedAt = time.Now()
	writeReport(report, spec.ReportRetention)

	// Check for task failures
	var failedTasks []string
	for _, t := range report.Tasks {
		if t.Status != "ok" {
			failedTasks = append(failedTasks, t.Label)
		}
	}

	if len(failedTasks) > 0 {
		// Keep spec so `aw reaper recover` can retry
		notifyUser(spec, fmt.Sprintf("task failure (%s) in session %s",
			strings.Join(failedTasks, ", "), spec.ContainerName))
	} else if specPath != "" {
		// All tasks succeeded — remove spec
		if err := os.Remove(specPath); err != nil && !os.IsNotExist(err) {
			log.Printf("reaper: removing spec: %v", err)
		}
	}

	return 0
}

// reaperDirOverride allows tests to redirect all reaper I/O to a temp directory.
var reaperDirOverride string

func ReaperDir() string {
	if reaperDirOverride != "" {
		return reaperDirOverride
	}
	return filepath.Join(platform.ConfigDir(), "reaper")
}

// RuntimeFromSpec reads a spec file and returns the runtime field.
// Falls back to "podman" if the file cannot be read or parsed.
func RuntimeFromSpec(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return "podman"
	}
	var spec struct {
		Runtime string `json:"runtime"`
	}
	if err := json.Unmarshal(data, &spec); err != nil || spec.Runtime == "" {
		return "podman"
	}
	return spec.Runtime
}

// ListReports returns sorted report file paths, excluding spec files.
func ListReports() []string {
	all, _ := filepath.Glob(filepath.Join(ReaperDir(), "*.json"))
	var reports []string
	for _, f := range all {
		if !strings.HasSuffix(f, ".spec.json") {
			reports = append(reports, f)
		}
	}
	sort.Strings(reports)
	return reports
}

func saveSpec(spec ReaperSpec) string {
	dir := ReaperDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("reaper: creating spec dir: %v", err)
		return ""
	}
	path := filepath.Join(dir, spec.ContainerName+".spec.json")
	saved := spec
	saved.TTY = ""
	data, err := json.Marshal(saved)
	if err != nil {
		log.Printf("reaper: marshaling spec: %v", err)
		return ""
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		log.Printf("reaper: writing spec: %v", err)
		return ""
	}
	return path
}

func writeReport(report RunReport, retention int) {
	dir := ReaperDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("reaper: creating report dir: %v", err)
		return
	}
	ts := report.StartedAt.Format("2006-01-02T15-04-05")
	path := filepath.Join(dir, fmt.Sprintf("%s-%s.json", ts, report.ContainerName))
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		log.Printf("reaper: marshaling report: %v", err)
		return
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		log.Printf("reaper: writing report: %v", err)
	}
	rotateReports(effectiveReportRetention(retention))
}

func rotateReports(keep int) {
	reports := ListReports()
	if len(reports) <= keep {
		return
	}
	for _, r := range reports[:len(reports)-keep] {
		report, err := ReadReport(r)
		if err == nil && report.DumpPath != "" {
			_ = os.RemoveAll(report.DumpPath)
		}
		_ = os.Remove(r)
	}
}
