package cmd

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/konono/aw/internal/profile"
)

func setTestHome(t *testing.T, home string) {
	t.Helper()
	t.Setenv("HOME", home)
	if runtime.GOOS == "windows" {
		t.Setenv("APPDATA", filepath.Join(home, ".config"))
		t.Setenv("LOCALAPPDATA", filepath.Join(home, ".local"))
	}
}

func captureOutput(t *testing.T, fn func()) (string, string) {
	t.Helper()

	oldStdout := os.Stdout
	oldStderr := os.Stderr

	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe(stdout): %v", err)
	}
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe(stderr): %v", err)
	}

	os.Stdout = stdoutW
	os.Stderr = stderrW
	defer func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
	}()

	fn()

	_ = stdoutW.Close()
	_ = stderrW.Close()

	stdout, err := io.ReadAll(stdoutR)
	if err != nil {
		t.Fatalf("reading stdout: %v", err)
	}
	stderr, err := io.ReadAll(stderrR)
	if err != nil {
		t.Fatalf("reading stderr: %v", err)
	}

	_ = stdoutR.Close()
	_ = stderrR.Close()

	return string(stdout), string(stderr)
}

func TestRunInit_CreatesConfig(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)

	cmd := &InitCmd{Force: false}
	stdout, stderr := captureOutput(t, func() {
		if err := cmd.Run(); err != nil {
			t.Fatalf("InitCmd.Run() error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}

	configPath := filepath.Join(home, ".config", "aw", "config.yml")
	got, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("reading created config: %v", err)
	}
	if !bytes.Equal(got, profile.DefaultConfigYAML()) {
		t.Fatalf("created config does not match embedded default")
	}
	if !strings.Contains(stdout, "Created "+configPath) {
		t.Errorf("stdout missing create message, got: %q", stdout)
	}
}

func TestRunInit_DoesNotOverwriteExistingConfig(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)

	configPath := filepath.Join(home, ".config", "aw", "config.yml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("custom: true\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := &InitCmd{Force: false}
	stdout, stderr := captureOutput(t, func() {
		err := cmd.Run()
		if err == nil {
			t.Fatalf("InitCmd.Run() should return error for existing config")
		}
	})

	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "already exists") {
		t.Fatalf("stderr = %q, want already exists message", stderr)
	}

	got, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("reading existing config: %v", err)
	}
	if string(got) != "custom: true\n" {
		t.Fatalf("existing config was overwritten: %q", string(got))
	}
}

func TestRunInit_ForceOverwritesExistingConfig(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)

	configPath := filepath.Join(home, ".config", "aw", "config.yml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("custom: true\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := &InitCmd{Force: true}
	stdout, stderr := captureOutput(t, func() {
		if err := cmd.Run(); err != nil {
			t.Fatalf("InitCmd.Run() error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}

	got, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("reading overwritten config: %v", err)
	}
	if !bytes.Equal(got, profile.DefaultConfigYAML()) {
		t.Fatalf("overwritten config does not match embedded default")
	}
	if !strings.Contains(stdout, "Overwrote "+configPath) {
		t.Errorf("stdout missing overwrite message, got: %q", stdout)
	}
}
