package profile

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestParse(t *testing.T) {
	yaml := `
default: container-claude

profiles:
  container-claude:
    environment: container
    launch: claude

  worktree-shell:
    worktree:
      base: origin/main
    environment: host
    launch: shell

  worktree-zellij:
    worktree: {}
    environment: container
    launch: zellij
    zellij:
      layout: default
`

	cfg, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	if cfg.Default != "container-claude" {
		t.Errorf("Default = %q, want %q", cfg.Default, "container-claude")
	}

	if len(cfg.Profiles) != 3 {
		t.Fatalf("got %d profiles, want 3", len(cfg.Profiles))
	}

	// Check container-claude profile
	dc := cfg.Profiles["container-claude"]
	if dc.Environment != EnvironmentContainer {
		t.Errorf("container-claude.Environment = %q, want %q", dc.Environment, EnvironmentContainer)
	}
	if dc.Launch != LaunchClaude {
		t.Errorf("container-claude.Launch = %q, want %q", dc.Launch, LaunchClaude)
	}
	if dc.Worktree != nil {
		t.Errorf("container-claude.Worktree should be nil")
	}

	// Check worktree-shell profile
	ws := cfg.Profiles["worktree-shell"]
	if ws.Worktree == nil {
		t.Fatal("worktree-shell.Worktree should not be nil")
	}
	if ws.Worktree.Base != "origin/main" {
		t.Errorf("worktree-shell.Worktree.Base = %q, want %q", ws.Worktree.Base, "origin/main")
	}
	if ws.Environment != EnvironmentHost {
		t.Errorf("worktree-shell.Environment = %q, want %q", ws.Environment, EnvironmentHost)
	}
	if ws.Launch != LaunchShell {
		t.Errorf("worktree-shell.Launch = %q, want %q", ws.Launch, LaunchShell)
	}

	// Check worktree-zellij profile
	wz := cfg.Profiles["worktree-zellij"]
	if wz.Worktree == nil {
		t.Fatal("worktree-zellij.Worktree should not be nil")
	}
	if wz.Launch != LaunchZellij {
		t.Errorf("worktree-zellij.Launch = %q, want %q", wz.Launch, LaunchZellij)
	}
	if wz.Zellij == nil {
		t.Fatal("worktree-zellij.Zellij should not be nil")
	}
	if wz.Zellij.Layout != "default" {
		t.Errorf("worktree-zellij.Zellij.Layout = %q, want %q", wz.Zellij.Layout, "default")
	}
}

func TestParse_EmptyProfiles(t *testing.T) {
	yaml := `
default: ""
`
	cfg, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	if cfg.Profiles == nil {
		t.Fatal("Profiles should not be nil (should be empty map)")
	}
	if len(cfg.Profiles) != 0 {
		t.Errorf("got %d profiles, want 0", len(cfg.Profiles))
	}
}

func TestParse_InvalidYAML(t *testing.T) {
	_, err := Parse([]byte("}{invalid"))
	if err == nil {
		t.Fatal("Parse() should return error for invalid YAML")
	}
}

func TestParse_WorktreeEmptyObject(t *testing.T) {
	yaml := `
profiles:
  test:
    worktree: {}
    environment: host
    launch: claude
`
	cfg, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	p := cfg.Profiles["test"]
	if p.Worktree == nil {
		t.Fatal("Worktree should not be nil for empty object")
	}
	if p.Worktree.EffectiveBase() != "origin/main" {
		t.Errorf("EffectiveBase() = %q, want %q", p.Worktree.EffectiveBase(), "origin/main")
	}
}

func TestParse_LaunchCodex(t *testing.T) {
	yaml := `
profiles:
  codex:
    environment: container
    launch: codex
  zellij-codex:
    environment: container
    launch: zellij
    zellij:
      layout: default
      tool: codex
`
	cfg, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	codex := cfg.Profiles["codex"]
	if codex.Launch != LaunchCodex {
		t.Errorf("codex.Launch = %q, want %q", codex.Launch, LaunchCodex)
	}

	zc := cfg.Profiles["zellij-codex"]
	if zc.Launch != LaunchZellij {
		t.Errorf("zellij-codex.Launch = %q, want %q", zc.Launch, LaunchZellij)
	}
	if zc.Zellij == nil {
		t.Fatal("zellij-codex.Zellij should not be nil")
	}
	if zc.Zellij.Tool != "codex" {
		t.Errorf("zellij-codex.Zellij.Tool = %q, want %q", zc.Zellij.Tool, "codex")
	}
}

func assertStarterProfiles(t *testing.T, cfg *Config) {
	t.Helper()

	tests := []struct {
		name   string
		launch LaunchMode
		os     OSTemplate
	}{
		{name: "shell", launch: LaunchShell, os: OSDebian12},
		{name: "claude", launch: LaunchClaude, os: OSDebian12},
		{name: "codex", launch: LaunchCodex, os: OSDebian12},
		{name: "opencode", launch: LaunchOpenCode, os: OSDebian12},
		{name: "ubi9-shell", launch: LaunchShell, os: OSUBI9},
		{name: "ubi10-shell", launch: LaunchShell, os: OSUBI10},
		{name: "ubuntu2604-shell", launch: LaunchShell, os: OSUbuntu2604},
	}

	for _, tt := range tests {
		p, ok := cfg.Profiles[tt.name]
		if !ok {
			t.Errorf("expected %s profile in builtin default", tt.name)
			continue
		}
		if p.Environment != EnvironmentContainer {
			t.Errorf("%s.Environment = %q, want %q", tt.name, p.Environment, EnvironmentContainer)
		}
		if p.Launch != tt.launch {
			t.Errorf("%s.Launch = %q, want %q", tt.name, p.Launch, tt.launch)
		}
		if p.OS != tt.os {
			t.Errorf("%s.OS = %q, want %q", tt.name, p.OS, tt.os)
		}
		if p.ContainerRuntime != ContainerRuntimePodman {
			t.Errorf("%s.ContainerRuntime = %q, want %q", tt.name, p.ContainerRuntime, ContainerRuntimePodman)
		}
		if p.EffectiveMountSSH() {
			t.Errorf("%s.EffectiveMountSSH() = true, want false", tt.name)
		}
		if p.EffectiveSSHAgentForwarding() {
			t.Errorf("%s.EffectiveSSHAgentForwarding() = true, want false", tt.name)
		}
	}
}

func TestLoadFile_NotFound(t *testing.T) {
	cfg, err := LoadFile("/nonexistent/path/.agent-workspace.yml")
	if err != nil {
		t.Fatalf("LoadFile() should not error for missing file, got: %v", err)
	}

	if cfg.Default != "claude" {
		t.Errorf("Default = %q, want %q", cfg.Default, "claude")
	}
	assertStarterProfiles(t, cfg)
}

func TestLoadFile_ValidFile(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".agent-workspace.yml")

	content := `
default: my-profile
profiles:
  my-profile:
    environment: host
    launch: shell
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFile(configPath)
	if err != nil {
		t.Fatalf("LoadFile() error: %v", err)
	}

	if cfg.Default != "my-profile" {
		t.Errorf("Default = %q, want %q", cfg.Default, "my-profile")
	}

	assertStarterProfiles(t, cfg)
	if _, ok := cfg.Profiles["my-profile"]; !ok {
		t.Error("user profile 'my-profile' should be present")
	}
}

func TestLoadFile_TopLevelMountSSHOverridesBuiltinProfiles(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".agent-workspace.yml")

	content := `
mount_ssh: true
profiles:
  claude:
    mount_ssh: false
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFile(configPath)
	if err != nil {
		t.Fatalf("LoadFile() error: %v", err)
	}

	shellProfile, ok := cfg.Profiles["shell"]
	if !ok {
		t.Fatal("expected builtin shell profile to be preserved")
	}
	if !shellProfile.EffectiveMountSSH() {
		t.Error("shell should inherit mount_ssh: true from top-level user config")
	}

	claudeProfile, ok := cfg.Profiles["claude"]
	if !ok {
		t.Fatal("expected builtin claude profile to be preserved")
	}
	if claudeProfile.EffectiveMountSSH() {
		t.Error("claude should override inherited mount_ssh to false")
	}
}

func TestParse_WorktreeOnCreate(t *testing.T) {
	yaml := `
profiles:
  test:
    worktree:
      base: origin/main
      on-create: "./scripts/setup.sh"
    environment: host
    launch: claude
`
	cfg, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	p := cfg.Profiles["test"]
	if p.Worktree == nil {
		t.Fatal("Worktree should not be nil")
	}
	if p.Worktree.OnCreate != "./scripts/setup.sh" {
		t.Errorf("OnCreate = %q, want %q", p.Worktree.OnCreate, "./scripts/setup.sh")
	}
}

func TestParse_WorktreeWithoutOnCreate(t *testing.T) {
	yaml := `
profiles:
  test:
    worktree:
      base: origin/main
    environment: host
    launch: claude
`
	cfg, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	p := cfg.Profiles["test"]
	if p.Worktree == nil {
		t.Fatal("Worktree should not be nil")
	}
	if p.Worktree.OnCreate != "" {
		t.Errorf("OnCreate should be empty, got %q", p.Worktree.OnCreate)
	}
}

func TestParse_WorktreeOnEnd(t *testing.T) {
	yaml := `
profiles:
  test:
    worktree:
      base: origin/main
      on-create: "./scripts/setup.sh"
      on-end: "./scripts/cleanup.sh"
    environment: host
    launch: claude
`
	cfg, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	p := cfg.Profiles["test"]
	if p.Worktree == nil {
		t.Fatal("Worktree should not be nil")
	}
	if p.Worktree.OnEnd != "./scripts/cleanup.sh" {
		t.Errorf("OnEnd = %q, want %q", p.Worktree.OnEnd, "./scripts/cleanup.sh")
	}
}

func TestParse_WorktreeWithoutOnEnd(t *testing.T) {
	yaml := `
profiles:
  test:
    worktree:
      base: origin/main
      on-create: "./scripts/setup.sh"
    environment: host
    launch: claude
`
	cfg, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	p := cfg.Profiles["test"]
	if p.Worktree == nil {
		t.Fatal("Worktree should not be nil")
	}
	if p.Worktree.OnEnd != "" {
		t.Errorf("OnEnd should be empty, got %q", p.Worktree.OnEnd)
	}
}

func TestParse_OS(t *testing.T) {
	yaml := `
profiles:
  test:
    environment: container
    launch: claude
    os: ubi9
`
	cfg, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	p := cfg.Profiles["test"]
	if p.OS != OSUBI9 {
		t.Errorf("OS = %q, want %q", p.OS, OSUBI9)
	}
}

func TestParse_Dockerfile(t *testing.T) {
	yaml := `
profiles:
  test:
    environment: container
    launch: claude
    dockerfile: docker/Dockerfile.custom
`
	cfg, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	p := cfg.Profiles["test"]
	if p.Dockerfile != "docker/Dockerfile.custom" {
		t.Errorf("Dockerfile = %q, want %q", p.Dockerfile, "docker/Dockerfile.custom")
	}
}

func TestParse_MountSSH(t *testing.T) {
	yaml := `
mount_ssh: true
profiles:
  enabled:
    environment: container
    launch: claude
  disabled:
    environment: container
    launch: shell
    mount_ssh: false
`
	cfg, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	if cfg.Defaults.MountSSH == nil || !*cfg.Defaults.MountSSH {
		t.Fatal("top-level mount_ssh should parse as true")
	}

	disabled := cfg.Profiles["disabled"]
	if disabled.MountSSH == nil {
		t.Fatal("profile mount_ssh should not be nil")
	}
	if *disabled.MountSSH {
		t.Error("disabled.mount_ssh = true, want false")
	}
}

func TestParse_SSHAgentForwarding(t *testing.T) {
	yaml := `
ssh_agent_forwarding: true
profiles:
  enabled:
    environment: container
    launch: claude
  disabled:
    environment: container
    launch: shell
    ssh_agent_forwarding: false
`
	cfg, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	if cfg.Defaults.SSHAgentForwarding == nil || !*cfg.Defaults.SSHAgentForwarding {
		t.Fatal("top-level ssh_agent_forwarding should parse as true")
	}

	disabled := cfg.Profiles["disabled"]
	if disabled.SSHAgentForwarding == nil {
		t.Fatal("profile ssh_agent_forwarding should not be nil")
	}
	if *disabled.SSHAgentForwarding {
		t.Error("disabled.ssh_agent_forwarding = true, want false")
	}
}

func TestParse_AuthConfig(t *testing.T) {
	yaml := `
profiles:
  codex:
    environment: container
    launch: codex
    auth:
      on_launch:
        check: warn
      codex:
        login_mode: device
        credentials_store: file
        seed_from_host: if_missing
        persist_auth: stage
        login_args:
          - --device-auth
`
	cfg, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}

	p := cfg.Profiles["codex"]
	if p.Auth == nil || p.Auth.Codex == nil {
		t.Fatal("auth.codex should not be nil")
	}
	if p.Auth.OnLaunch == nil || p.Auth.OnLaunch.Check != AuthOnLaunchCheckWarn {
		t.Fatalf("auth.on_launch = %+v, want warn", p.Auth.OnLaunch)
	}
	if p.Auth.Codex.LoginMode != CodexLoginModeDevice {
		t.Errorf("auth.codex.login_mode = %q, want %q", p.Auth.Codex.LoginMode, CodexLoginModeDevice)
	}
	if p.Auth.Codex.CredentialsStore != CodexCredentialsStoreFile {
		t.Errorf("auth.codex.credentials_store = %q, want %q", p.Auth.Codex.CredentialsStore, CodexCredentialsStoreFile)
	}
	if len(p.Auth.Codex.LoginArgs) != 1 || p.Auth.Codex.LoginArgs[0] != "--device-auth" {
		t.Errorf("auth.codex.login_args = %#v, want [--device-auth]", p.Auth.Codex.LoginArgs)
	}
}

func TestLoadFile_TopLevelSSHAgentForwardingOverridesBuiltinProfiles(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".agent-workspace.yml")

	content := `
ssh_agent_forwarding: true
profiles:
  claude:
    ssh_agent_forwarding: false
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFile(configPath)
	if err != nil {
		t.Fatalf("LoadFile() error: %v", err)
	}

	shellProfile, ok := cfg.Profiles["shell"]
	if !ok {
		t.Fatal("expected builtin shell profile to be preserved")
	}
	if !shellProfile.EffectiveSSHAgentForwarding() {
		t.Error("shell should inherit ssh_agent_forwarding: true from top-level user config")
	}

	claudeProfile, ok := cfg.Profiles["claude"]
	if !ok {
		t.Fatal("expected builtin claude profile to be preserved")
	}
	if claudeProfile.EffectiveSSHAgentForwarding() {
		t.Error("claude should override inherited ssh_agent_forwarding to false")
	}
}

// mockLoadEnv sets up findGitRoot and globalConfigDir mocks, restoring them on cleanup.
func mockLoadEnv(t *testing.T, gitRoot string, gitErr bool, globalDir string) {
	t.Helper()
	origGit := findGitRoot
	origGlobal := globalConfigDir
	t.Cleanup(func() {
		findGitRoot = origGit
		globalConfigDir = origGlobal
	})

	if gitErr {
		findGitRoot = func() (string, error) { return "", fmt.Errorf("not in a git repository") }
	} else {
		findGitRoot = func() (string, error) { return gitRoot, nil }
	}

	if globalDir == "" {
		globalConfigDir = func() (string, error) { return "", fmt.Errorf("no home") }
	} else {
		globalConfigDir = func() (string, error) { return globalDir, nil }
	}
}

func TestLoad_NoGitRepo_WithoutConfigInCwd(t *testing.T) {
	dir := t.TempDir()
	mockLoadEnv(t, "", true, dir)

	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() should not error when not in git repo, got: %v", err)
	}

	if cfg.Default != "claude" {
		t.Errorf("Default = %q, want %q", cfg.Default, "claude")
	}
	assertStarterProfiles(t, cfg)
	if !cfg.Source.IsBuiltin {
		t.Error("Source should be builtin when no config file found")
	}
}

func TestLoad_NoGitRepo_WithConfigInCwd(t *testing.T) {
	dir := t.TempDir()
	globalDir := t.TempDir()
	mockLoadEnv(t, "", true, globalDir)

	configPath := filepath.Join(dir, ".agent-workspace.yml")
	content := `
default: my-profile
container_runtime: podman
profiles:
  my-profile:
    environment: container
    launch: claude
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Default != "my-profile" {
		t.Errorf("Default = %q, want %q", cfg.Default, "my-profile")
	}
	if cfg.Source.IsBuiltin {
		t.Error("Source should not be builtin when config file exists in cwd")
	}

	p, ok := cfg.Profiles["my-profile"]
	if !ok {
		t.Fatal("expected my-profile in profiles")
	}
	if p.ContainerRuntime != ContainerRuntimePodman {
		t.Errorf("ContainerRuntime = %q, want %q", p.ContainerRuntime, ContainerRuntimePodman)
	}
}

func TestLoad_GlobalConfigOnly(t *testing.T) {
	globalDir := t.TempDir()
	cwdDir := t.TempDir()
	mockLoadEnv(t, "", true, globalDir)

	content := `
default: global-profile
container_runtime: podman
env:
  MY_VAR: "from-global"
profiles:
  global-profile:
    environment: container
    launch: claude
`
	if err := os.WriteFile(filepath.Join(globalDir, "config.yml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	origDir, _ := os.Getwd()
	if err := os.Chdir(cwdDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Default != "global-profile" {
		t.Errorf("Default = %q, want %q", cfg.Default, "global-profile")
	}
	if cfg.Source.IsBuiltin {
		t.Error("Source should not be builtin when global config exists")
	}
	if cfg.Source.FilePath != filepath.Join(globalDir, "config.yml") {
		t.Errorf("Source.FilePath = %q, want %q", cfg.Source.FilePath, filepath.Join(globalDir, "config.yml"))
	}

	p, ok := cfg.Profiles["global-profile"]
	if !ok {
		t.Fatal("expected global-profile in profiles")
	}
	if p.ContainerRuntime != ContainerRuntimePodman {
		t.Errorf("ContainerRuntime = %q, want %q", p.ContainerRuntime, ContainerRuntimePodman)
	}
	if p.Env["MY_VAR"] != "from-global" {
		t.Errorf("Env[MY_VAR] = %q, want %q", p.Env["MY_VAR"], "from-global")
	}

	builtinClaude, ok := cfg.Profiles["claude"]
	if !ok {
		t.Fatal("expected builtin claude profile to be preserved")
	}
	if builtinClaude.ContainerRuntime != ContainerRuntimePodman {
		t.Errorf("builtin claude ContainerRuntime = %q, want %q", builtinClaude.ContainerRuntime, ContainerRuntimePodman)
	}
	if builtinClaude.Env["MY_VAR"] != "from-global" {
		t.Errorf("builtin claude Env[MY_VAR] = %q, want %q", builtinClaude.Env["MY_VAR"], "from-global")
	}
}

func TestLoad_GlobalAndProjectConfig(t *testing.T) {
	globalDir := t.TempDir()
	projectDir := t.TempDir()
	mockLoadEnv(t, "", true, globalDir)

	globalContent := `
default: global-profile
container_runtime: podman
env:
  SHARED_VAR: "global"
  OVERRIDE_VAR: "global"
profiles:
  global-profile:
    environment: container
    launch: claude
`
	if err := os.WriteFile(filepath.Join(globalDir, "config.yml"), []byte(globalContent), 0644); err != nil {
		t.Fatal(err)
	}

	projectContent := `
default: project-profile
env:
  OVERRIDE_VAR: "project"
profiles:
  project-profile:
    environment: host
    launch: shell
`
	if err := os.WriteFile(filepath.Join(projectDir, ".agent-workspace.yml"), []byte(projectContent), 0644); err != nil {
		t.Fatal(err)
	}

	origDir, _ := os.Getwd()
	if err := os.Chdir(projectDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// Project default takes precedence
	if cfg.Default != "project-profile" {
		t.Errorf("Default = %q, want %q", cfg.Default, "project-profile")
	}
	// Source points to project config (use filepath.EvalSymlinks for macOS /var → /private/var)
	wantPath, _ := filepath.EvalSymlinks(filepath.Join(projectDir, ".agent-workspace.yml"))
	gotPath, _ := filepath.EvalSymlinks(cfg.Source.FilePath)
	if gotPath != wantPath {
		t.Errorf("Source.FilePath = %q, want %q", cfg.Source.FilePath, wantPath)
	}
	// Global-only profile is preserved
	if _, ok := cfg.Profiles["global-profile"]; !ok {
		t.Error("expected global-profile to be preserved from global config")
	}
	// Project-only profile exists
	p, ok := cfg.Profiles["project-profile"]
	if !ok {
		t.Fatal("expected project-profile in profiles")
	}
	// Project profile inherits container_runtime from global top-level (via ApplyTopLevel)
	if p.ContainerRuntime != ContainerRuntimePodman {
		t.Errorf("ContainerRuntime = %q, want %q (inherited from global)", p.ContainerRuntime, ContainerRuntimePodman)
	}
	// Env: project overrides global
	if p.Env["OVERRIDE_VAR"] != "project" {
		t.Errorf("Env[OVERRIDE_VAR] = %q, want %q", p.Env["OVERRIDE_VAR"], "project")
	}
	// Env: global value preserved when not overridden
	if p.Env["SHARED_VAR"] != "global" {
		t.Errorf("Env[SHARED_VAR] = %q, want %q", p.Env["SHARED_VAR"], "global")
	}
}

func TestLoad_GlobalConfigParseError(t *testing.T) {
	globalDir := t.TempDir()
	mockLoadEnv(t, "", true, globalDir)

	if err := os.WriteFile(filepath.Join(globalDir, "config.yml"), []byte("}{invalid"), 0644); err != nil {
		t.Fatal(err)
	}

	cwdDir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(cwdDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	_, err := Load()
	if err == nil {
		t.Fatal("Load() should return error for invalid global config")
	}
}

func TestLoad_NoGlobalNoProject(t *testing.T) {
	globalDir := t.TempDir()
	cwdDir := t.TempDir()
	mockLoadEnv(t, "", true, globalDir)

	origDir, _ := os.Getwd()
	if err := os.Chdir(cwdDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Default != "claude" {
		t.Errorf("Default = %q, want %q", cfg.Default, "claude")
	}
	assertStarterProfiles(t, cfg)
	if !cfg.Source.IsBuiltin {
		t.Error("Source should be builtin when no config files found")
	}
}
