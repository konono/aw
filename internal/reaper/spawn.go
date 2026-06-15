package reaper

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

// Handle holds the pipe write side passed to the container runtime after exec.
// If exec fails, call Abort to stop the reaper without running post-container tasks.
type Handle struct {
	Write         *os.File
	pid           int
	containerName string
}

// Abort kills the reaper subprocess and closes the pipe without waiting for
// container cleanup. Used when syscall.Exec fails after Spawn.
// Spawn transfers ownership of cmd.Process to Handle; Start() is called but
// cmd.Wait() is only invoked here via os.Process.Wait.
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
// The caller must clear O_CLOEXEC on Handle.Write before syscall.Exec.
func Spawn(spec ReaperSpec) (*Handle, error) {
	r, w, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("creating pipe: %w", err)
	}

	cmd := exec.Command(os.Args[0], "--internal-reaper")
	cmd.ExtraFiles = []*os.File{r} // fd 3
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true, // separate from foreground process group
	}
	if logFile, err := os.OpenFile(filepath.Join(ReaperDir(), "reaper.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
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
