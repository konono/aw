package reaper

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// Spawn starts a reaper subprocess and writes the spec over a pipe.
// It returns the write side of the pipe, which must have O_CLOEXEC cleared
// so that the subsequent syscall.Exec inherits it to the container runtime.
func Spawn(spec ReaperSpec) (*os.File, error) {
	r, w, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("creating pipe: %w", err)
	}

	cmd := exec.Command(os.Args[0], "--internal-reaper")
	cmd.ExtraFiles = []*os.File{r} // fd 3
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true, // separate from foreground process group
	}
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		r.Close()
		w.Close()
		return nil, fmt.Errorf("starting reaper: %w", err)
	}
	r.Close()

	if err := json.NewEncoder(w).Encode(spec); err != nil {
		w.Close()
		return nil, fmt.Errorf("writing spec: %w", err)
	}

	return w, nil
}
