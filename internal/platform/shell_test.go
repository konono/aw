package platform

import (
	"context"
	"os/exec"
	"testing"
)

func TestDefaultShell_RespectsEnv(t *testing.T) {
	t.Setenv("SHELL", "/usr/bin/zsh")
	got := DefaultShell()
	if got != "/usr/bin/zsh" {
		t.Errorf("DefaultShell() = %q, want /usr/bin/zsh when SHELL is set", got)
	}
}

func TestDefaultShell_FallbackIsNonEmpty(t *testing.T) {
	t.Setenv("SHELL", "")
	got := DefaultShell()
	if got == "" {
		t.Error("DefaultShell() returned empty string when SHELL is unset")
	}
}

func TestShellCommand_CanExecute(t *testing.T) {
	shell, flags := ShellCommand()
	args := append(flags, "echo ok")
	cmd := exec.Command(shell, args...)
	if err := cmd.Run(); err != nil {
		t.Errorf("ShellCommand() returned (%q, %v) which cannot execute 'echo ok': %v", shell, flags, err)
	}
}

func TestRunShellCommand_Success(t *testing.T) {
	err := RunShellCommand(context.Background(), "echo hello", "")
	if err != nil {
		t.Errorf("RunShellCommand('echo hello') returned error: %v", err)
	}
}

func TestRunShellCommand_Failure(t *testing.T) {
	err := RunShellCommand(context.Background(), "exit 1", "")
	if err == nil {
		t.Error("RunShellCommand('exit 1') should return error")
	}
}

func TestRunShellCommand_RespectsWorkDir(t *testing.T) {
	dir := t.TempDir()
	err := RunShellCommand(context.Background(), "echo ok", dir)
	if err != nil {
		t.Errorf("RunShellCommand with dir=%q returned error: %v", dir, err)
	}
}

func TestRunShellCommand_FailsWithBadDir(t *testing.T) {
	err := RunShellCommand(context.Background(), "echo ok", "/nonexistent-dir-12345")
	if err == nil {
		t.Error("RunShellCommand with nonexistent dir should return error")
	}
}

func TestRunShellCommand_RespectsContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := RunShellCommand(ctx, "echo ok", "")
	if err == nil {
		t.Error("RunShellCommand with cancelled context should return error")
	}
}
