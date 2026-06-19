//go:build windows

package platform

import (
	"context"
	"os"
	"os/exec"
)

// DefaultShell returns the user's preferred shell on the host.
// On Windows, defaults to cmd.exe unless SHELL is set (e.g., in Git Bash).
func DefaultShell() string {
	if shell := os.Getenv("SHELL"); shell != "" {
		return shell
	}
	if comspec := os.Getenv("COMSPEC"); comspec != "" {
		return comspec
	}
	return "cmd.exe"
}

// ShellCommand returns the shell binary and its prefix arguments for executing
// a command string. On Windows with Git Bash (SHELL is set), uses sh -c for
// compatibility with Unix-style hook scripts. Otherwise uses cmd /c.
func ShellCommand() (string, []string) {
	if shell := os.Getenv("SHELL"); shell != "" {
		return "sh", []string{"-c"}
	}
	return "cmd", []string{"/c"}
}

// RunShellCommand executes a command string via the system shell.
// On Windows, this uses cmd /c.
func RunShellCommand(ctx context.Context, command, dir string) error {
	cmd := exec.CommandContext(ctx, "cmd", "/c", command)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
