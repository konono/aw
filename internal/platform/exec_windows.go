//go:build windows

package platform

import (
	"os"
	"os/exec"
)

// ExecReplace emulates Unix exec by spawning the process and exiting.
// Windows does not support replacing the current process image.
func ExecReplace(binPath string, argv []string, env []string) error {
	cmd := exec.Command(binPath, argv[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = env

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		return err
	}
	os.Exit(0)
	return nil
}
