//go:build windows

package platform

import (
	"os"
	"os/signal"
	"syscall"
)

// ReaperSysProcAttr returns the SysProcAttr for the reaper subprocess.
// On Windows, CREATE_NEW_PROCESS_GROUP separates the reaper from the console.
func ReaperSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}

// ContainerSurvivalSignals returns the signals to absorb so the container
// survives when the terminal is closed. On Windows, only os.Interrupt is available.
func ContainerSurvivalSignals() []os.Signal {
	return []os.Signal{os.Interrupt}
}

// NotifyContainerSurvivalSignals sets up signal handling for container survival.
func NotifyContainerSurvivalSignals(ch chan<- os.Signal) {
	signal.Notify(ch, os.Interrupt)
}

// KillProcessIfSSH kills a process by PID if it appears to be an SSH agent tunnel.
// On Windows, we verify the process is still alive before killing it.
// Unlike Unix, we cannot inspect the command line cheaply, so we rely on
// the caller (reaper) providing the correct PID from a trusted spec.
func KillProcessIfSSH(pid, _ int) error {
	p, err := os.FindProcess(pid)
	if err != nil {
		return nil
	}
	if err := p.Kill(); err != nil {
		return nil
	}
	return nil
}

// HostUserID returns the --user value for docker run.
// On Windows, Docker Desktop handles UID mapping automatically.
func HostUserID() string {
	return ""
}

// IsRunningAsRoot returns true if the current process is running with
// elevated privileges. On Windows, this always returns false since
// the Unix root concept does not apply.
func IsRunningAsRoot() bool {
	return false
}
