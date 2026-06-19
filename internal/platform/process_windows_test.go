//go:build windows

package platform

import (
	"os"
	"os/exec"
	"testing"
)

func TestReaperSysProcAttr_CanSpawnSubprocess(t *testing.T) {
	cmd := exec.Command("cmd", "/c", "echo ok")
	cmd.SysProcAttr = ReaperSysProcAttr()
	if err := cmd.Run(); err != nil {
		t.Errorf("subprocess with ReaperSysProcAttr() failed: %v", err)
	}
}

func TestContainerSurvivalSignals_NotEmpty(t *testing.T) {
	signals := ContainerSurvivalSignals()
	if len(signals) == 0 {
		t.Error("ContainerSurvivalSignals() returned empty slice")
	}
}

func TestContainerSurvivalSignals_AreValidSignals(t *testing.T) {
	ch := make(chan os.Signal, 1)
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("signal.Notify panicked with ContainerSurvivalSignals: %v", r)
		}
	}()
	NotifyContainerSurvivalSignals(ch)
}

func TestHostUserID_EmptyOnWindows(t *testing.T) {
	uid := HostUserID()
	if uid != "" {
		t.Errorf("HostUserID() = %q, want empty string on Windows", uid)
	}
}

func TestIsRunningAsRoot_AlwaysFalseOnWindows(t *testing.T) {
	if IsRunningAsRoot() {
		t.Error("IsRunningAsRoot() = true, want false on Windows")
	}
}

func TestKillProcessIfSSH_SafeForNonExistentPID(t *testing.T) {
	err := KillProcessIfSSH(99999999, 15)
	if err != nil {
		t.Errorf("KillProcessIfSSH with nonexistent PID should not return error, got: %v", err)
	}
}
