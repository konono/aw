//go:build unix

package platform

import (
	"context"
	"os"
	"os/exec"
)

// DefaultShell returns the user's preferred shell on the host.
func DefaultShell() string {
	if shell := os.Getenv("SHELL"); shell != "" {
		return shell
	}
	return "/bin/sh"
}

// ShellCommand returns the shell binary and its prefix arguments for executing
// a command string. On Unix, this returns ("sh", ["-c"]).
func ShellCommand() (string, []string) {
	return "sh", []string{"-c"}
}

// RunShellCommand executes a command string via the system shell.
func RunShellCommand(ctx context.Context, command, dir string) error {
	shell, args := ShellCommand()
	cmd := exec.CommandContext(ctx, shell, append(args, command)...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
