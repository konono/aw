//go:build unix

package platform

import (
	"os"
	"os/exec"
)

// SetupReaperPipe configures the command to inherit the read end of the pipe.
// On Unix, the pipe is passed as fd 3 via ExtraFiles.
func SetupReaperPipe(cmd *exec.Cmd, r *os.File) {
	cmd.ExtraFiles = []*os.File{r}
}

// ReaperPipe returns the pipe for the reaper subprocess to read from.
// On Unix, this is fd 3 (inherited via ExtraFiles).
func ReaperPipe() *os.File {
	return os.NewFile(3, "pipe")
}
