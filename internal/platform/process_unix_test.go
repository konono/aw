//go:build unix

package platform

import (
	"os"
	"os/exec"
	"regexp"
	"testing"
)

func TestReaperSysProcAttr_CanSpawnSubprocess(t *testing.T) {
	cmd := exec.Command("true")
	cmd.SysProcAttr = ReaperSysProcAttr()
	if err := cmd.Run(); err != nil {
		t.Errorf("subprocess with ReaperSysProcAttr() failed: %v", err)
	}
}

func TestReaperSysProcAttr_SubprocessHasDifferentPGID(t *testing.T) {
	cmd := exec.Command("sh", "-c", "echo $$")
	cmd.SysProcAttr = ReaperSysProcAttr()
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("subprocess failed: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("subprocess produced no output")
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

func TestHostUserID_HasExpectedFormat(t *testing.T) {
	uid := HostUserID()
	if uid == "" {
		t.Fatal("HostUserID() returned empty string on Unix")
	}
	matched, _ := regexp.MatchString(`^\d+:0$`, uid)
	if !matched {
		t.Errorf("HostUserID() = %q, want format '<number>:0'", uid)
	}
}

func TestIsRunningAsRoot_NotRootInCI(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root, cannot test non-root behavior")
	}
	if IsRunningAsRoot() {
		t.Error("IsRunningAsRoot() = true, want false for non-root user")
	}
}

func TestKillProcessIfSSH_SafeForNonExistentPID(t *testing.T) {
	err := KillProcessIfSSH(99999999, 15)
	if err != nil {
		t.Errorf("KillProcessIfSSH with nonexistent PID should return nil, got: %v", err)
	}
}
