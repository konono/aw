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
	"strconv"
	"strings"
	"time"
)

// Run is the entry point for the reaper subprocess.
// It reads a ReaperSpec from fd 3, waits for EOF (= container exit),
// then executes tasks and cleans up.
func Run() int {
	pipe := os.NewFile(3, "pipe")
	if pipe == nil {
		log.Print("reaper: fd 3 not available")
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

	// Wait for EOF = container process exited (pipe write side closed)
	_, _ = io.Copy(io.Discard, reader)
	_ = pipe.Close()

	timeout := 60 * time.Second
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
	writeReport(report)

	// Remove spec file on successful completion
	if specPath != "" {
		if err := os.Remove(specPath); err != nil && !os.IsNotExist(err) {
			log.Printf("reaper: removing spec: %v", err)
		}
	}

	return 0
}

func inspectWithRetry(runtime, name string, maxRetries int) int {
	for i := 0; i < maxRetries; i++ {
		out, err := exec.Command(runtime, "inspect", name,
			"--format", "{{.State.ExitCode}}").Output()
		if err == nil {
			code, _ := strconv.Atoi(strings.TrimSpace(string(out)))
			return code
		}
		if i < maxRetries-1 {
			delay := 500 * time.Millisecond
			if i > 0 {
				delay = 2 * time.Second
			}
			time.Sleep(delay)
		}
	}
	return -1
}

func reaperDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "aw", "reaper")
}

func saveSpec(spec ReaperSpec) string {
	dir := reaperDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("reaper: creating spec dir: %v", err)
		return ""
	}
	path := filepath.Join(dir, spec.ContainerName+".spec.json")
	data, err := json.Marshal(spec)
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

func writeReport(report RunReport) {
	dir := reaperDir()
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
}
