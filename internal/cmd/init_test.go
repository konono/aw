package cmd

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/konono/aw/internal/profile"
)

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
	t.Setenv("HOME", home)

	stdout, stderr := captureOutput(t, func() {
		if code := Run([]string{"init"}); code != 0 {
			t.Fatalf("Run(init) = %d, want 0", code)
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
	t.Setenv("HOME", home)

	configPath := filepath.Join(home, ".config", "aw", "config.yml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("custom: true\n"), 0644); err != nil {
		t.Fatal(err)
	}

	stdout, stderr := captureOutput(t, func() {
		if code := Run([]string{"init"}); code != 1 {
			t.Fatalf("Run(init) = %d, want 1", code)
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
	t.Setenv("HOME", home)

	configPath := filepath.Join(home, ".config", "aw", "config.yml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("custom: true\n"), 0644); err != nil {
		t.Fatal(err)
	}

	stdout, stderr := captureOutput(t, func() {
		if code := Run([]string{"init", "--force"}); code != 0 {
			t.Fatalf("Run(init --force) = %d, want 0", code)
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

func TestRunInit_UpdateMigratesExistingConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	configPath := filepath.Join(home, ".config", "aw", "config.yml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		t.Fatal(err)
	}

	userConfig := []byte(`default: shell
environment: container
os: debian12
container_runtime: docker
mount_ssh: true
profiles:
  shell:
    launch: shell
  claude:
    launch: claude
  my-project:
    environment: host
    launch: shell
    env:
      FOO: bar
`)
	if err := os.WriteFile(configPath, userConfig, 0644); err != nil {
		t.Fatal(err)
	}

	stdout, stderr := captureOutput(t, func() {
		if code := Run([]string{"init", "--update"}); code != 0 {
			t.Fatalf("Run(init --update) = %d, want 0\nstderr: %s", code, stderr)
		}
	})

	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	if !strings.Contains(stdout, "Updated") {
		t.Errorf("stdout missing Updated message, got: %q", stdout)
	}
	if !strings.Contains(stdout, "config.yml.bak") {
		t.Errorf("stdout missing backup message, got: %q", stdout)
	}

	backupPath := configPath + ".bak"
	backup, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("backup file should exist: %v", err)
	}
	if !bytes.Equal(backup, userConfig) {
		t.Error("backup should contain original config")
	}

	migrated, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("reading migrated config: %v", err)
	}

	cfg, err := profile.Parse(migrated)
	if err != nil {
		t.Fatalf("parsing migrated config: %v", err)
	}

	if cfg.Default != "shell" {
		t.Errorf("Default = %q, want %q", cfg.Default, "shell")
	}
	if cfg.Defaults.ContainerRuntime != "docker" {
		t.Errorf("ContainerRuntime = %q, want %q", cfg.Defaults.ContainerRuntime, "docker")
	}
	if _, ok := cfg.Profiles["my-project"]; !ok {
		t.Error("user profile 'my-project' should be preserved")
	}
}

func TestRunInit_UpdateWithNoConfigCreatesNew(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	stdout, stderr := captureOutput(t, func() {
		if code := Run([]string{"init", "--update"}); code != 0 {
			t.Fatalf("Run(init --update) = %d, want 0", code)
		}
	})

	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}

	configPath := filepath.Join(home, ".config", "aw", "config.yml")
	got, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("config should be created: %v", err)
	}
	if !bytes.Equal(got, profile.DefaultConfigYAML()) {
		t.Fatalf("created config does not match embedded default")
	}
	if !strings.Contains(stdout, "Created") {
		t.Errorf("stdout missing Created message, got: %q", stdout)
	}
}

func TestHelpIncludesInit(t *testing.T) {
	stdout, stderr := captureOutput(t, func() {
		if code := Run([]string{"--help"}); code != 0 {
			t.Fatalf("Run(--help) = %d, want 0", code)
		}
	})

	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	if !strings.Contains(stdout, "aw init") {
		t.Fatalf("stdout missing init help, got: %q", stdout)
	}
}
