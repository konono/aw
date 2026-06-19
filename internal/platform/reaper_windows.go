//go:build windows

package platform

import (
	"os"
	"os/exec"
)

// SetupReaperPipe configures the command to inherit the read end of the pipe.
// On Windows, ExtraFiles is not supported, so the pipe is passed via Stdin.
func SetupReaperPipe(cmd *exec.Cmd, r *os.File) {
	cmd.Stdin = r
}

// ReaperPipe returns the pipe for the reaper subprocess to read from.
// On Windows, the pipe is received via Stdin.
func ReaperPipe() *os.File {
	return os.Stdin
}
