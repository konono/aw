package reaper

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/konono/aw/v4/internal/platform"
)

// Handle holds the pipe write side kept by the ExecRun wrapper.
// Call Abort to stop the reaper without running post-container tasks.
type Handle struct {
	Write         *os.File
	pid           int
	containerName string
}

// Abort kills the reaper subprocess, closes the pipe, and removes the spec
// file without running post-container tasks. Called when the container fails
// to start so that no orphaned reaper or spec is left behind.
func (h *Handle) Abort() {
	if h == nil {
		return
	}
	if h.pid > 0 {
		if p, err := os.FindProcess(h.pid); err == nil {
			_ = p.Kill()
			_, _ = p.Wait()
		}
	}
	if h.Write != nil {
		_ = h.Write.Close()
	}
	if h.containerName != "" {
		specPath := filepath.Join(ReaperDir(), h.containerName+".spec.json")
		_ = os.Remove(specPath)
	}
}

// Spawn starts a reaper subprocess and writes the spec over a pipe.
// The caller keeps Handle.Write open; when it closes, the reaper detects EOF.
func Spawn(spec ReaperSpec) (*Handle, error) {
	r, w, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("creating pipe: %w", err)
	}

	cmd := exec.Command(os.Args[0], "--internal-reaper")
	platform.SetupReaperPipe(cmd, r)
	cmd.SysProcAttr = platform.ReaperSysProcAttr()
	_ = os.MkdirAll(ReaperDir(), 0755)
	logPath := filepath.Join(ReaperDir(), "reaper.log")
	if info, err := os.Stat(logPath); err == nil && info.Size() > 1<<20 {
		_ = os.Truncate(logPath, 0)
	}
	if logFile, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
		cmd.Stderr = logFile
	}

	if err := cmd.Start(); err != nil {
		_ = r.Close()
		_ = w.Close()
		return nil, fmt.Errorf("starting reaper: %w", err)
	}
	_ = r.Close()

	if err := json.NewEncoder(w).Encode(spec); err != nil {
		_ = w.Close()
		return nil, fmt.Errorf("writing spec: %w", err)
	}

	return &Handle{
		Write:         w,
		pid:           cmd.Process.Pid,
		containerName: spec.ContainerName,
	}, nil
}
