//go:build unix

package platform

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
)

// ReaperSysProcAttr returns the SysProcAttr for the reaper subprocess.
// On Unix, Setpgid separates the reaper from the foreground process group.
func ReaperSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Setpgid: true,
	}
}

// ContainerSurvivalSignals returns the signals to absorb so the container
// survives when the terminal is closed or the process receives a termination signal.
func ContainerSurvivalSignals() []os.Signal {
	return []os.Signal{syscall.SIGTERM, syscall.SIGHUP}
}

// NotifyContainerSurvivalSignals sets up signal handling for container survival.
func NotifyContainerSurvivalSignals(ch chan<- os.Signal) {
	signal.Notify(ch, syscall.SIGTERM, syscall.SIGHUP)
}

// KillProcessIfSSH kills a process only if it is an SSH agent tunnel process.
// It checks the process command line via ps to avoid killing unrelated processes
// that reused the same PID.
func KillProcessIfSSH(pid, sig int) error {
	out, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "command=").Output()
	if err != nil {
		return nil
	}
	cmd := strings.TrimSpace(string(out))
	if !strings.Contains(cmd, "ssh") || !strings.Contains(cmd, "aw-ssh-agent") {
		return nil
	}

	p, err := os.FindProcess(pid)
	if err != nil {
		return nil
	}
	return p.Signal(syscall.Signal(sig))
}

// HostUserID returns the --user value for docker run.
// On Unix, this maps the current user's UID and GID.
func HostUserID() string {
	return fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid())
}

// IsRunningAsRoot returns true if the current process is running as root.
func IsRunningAsRoot() bool {
	return os.Getuid() == 0
}
