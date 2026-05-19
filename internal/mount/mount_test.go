package mount

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/konono/aw/internal/docker"
)

func newTestOpts(homeDir, workDir string) MountOptions {
	return MountOptions{
		HomeDir:          homeDir,
		WorkDir:          workDir,
		ToolStageDir:     filepath.Join(homeDir, ".agent-workspace", "claude"),
		ToolContainerDir: "/home/agent/.claude",
		MountGH:          true,
	}
}

func findMount(mounts []docker.Mount, target string) *docker.Mount {
	for _, m := range mounts {
		if m.Target == target {
			return &m
		}
	}
	return nil
}

func TestBuildMounts_FixedMountsAlwaysPresent(t *testing.T) {
	homeDir := t.TempDir()
	workDir := t.TempDir()
	opts := newTestOpts(homeDir, workDir)

	builder := NewBuilder()
	mounts, err := builder.BuildMounts(opts)
	if err != nil {
		t.Fatalf("BuildMounts() error: %v", err)
	}

	// Tool config mount
	cfg := findMount(mounts, "/home/agent/.claude")
	if cfg == nil {
		t.Fatal("missing mount for /home/agent/.claude")
	}
	if cfg.Source != opts.ToolStageDir {
		t.Errorf("tool config source = %q, want %q", cfg.Source, opts.ToolStageDir)
	}

	// Workspace mount
	ws := findMount(mounts, workDir)
	if ws == nil {
		t.Fatalf("missing workspace mount for %s", workDir)
	}
	if ws.Source != workDir {
		t.Errorf("workspace source = %q, want %q", ws.Source, workDir)
	}
}

func TestBuildMounts_GitconfigWhenPresent(t *testing.T) {
	homeDir := t.TempDir()
	workDir := t.TempDir()

	// Create .gitconfig
	if err := os.WriteFile(filepath.Join(homeDir, ".gitconfig"), []byte("[user]\nname=test"), 0644); err != nil {
		t.Fatalf("writing .gitconfig: %v", err)
	}

	opts := newTestOpts(homeDir, workDir)
	builder := NewBuilder()
	mounts, err := builder.BuildMounts(opts)
	if err != nil {
		t.Fatalf("BuildMounts() error: %v", err)
	}

	m := findMount(mounts, "/home/agent/.gitconfig")
	if m == nil {
		t.Fatal("missing .gitconfig mount")
	}
	if m.Source != filepath.Join(homeDir, ".gitconfig") {
		t.Errorf("source = %q, want %q", m.Source, filepath.Join(homeDir, ".gitconfig"))
	}
	if !m.ReadOnly {
		t.Error(".gitconfig mount should be read-only")
	}
}

func TestBuildMounts_NoGitconfigWhenMissing(t *testing.T) {
	homeDir := t.TempDir()
	workDir := t.TempDir()

	opts := newTestOpts(homeDir, workDir)
	builder := NewBuilder()
	mounts, err := builder.BuildMounts(opts)
	if err != nil {
		t.Fatalf("BuildMounts() error: %v", err)
	}

	if findMount(mounts, "/home/agent/.gitconfig") != nil {
		t.Error(".gitconfig mount should not exist when file is missing")
	}
}

func TestBuildMounts_GhConfigWhenPresent(t *testing.T) {
	homeDir := t.TempDir()
	workDir := t.TempDir()

	if err := os.MkdirAll(filepath.Join(homeDir, ".config", "gh"), 0755); err != nil {
		t.Fatalf("creating .config/gh: %v", err)
	}

	opts := newTestOpts(homeDir, workDir)
	builder := NewBuilder()
	mounts, err := builder.BuildMounts(opts)
	if err != nil {
		t.Fatalf("BuildMounts() error: %v", err)
	}

	m := findMount(mounts, "/home/agent/.config/gh")
	if m == nil {
		t.Fatal("missing .config/gh mount")
	}
	if !m.ReadOnly {
		t.Error(".config/gh mount should be read-only")
	}
}

func TestBuildMounts_GhConfigDisabled(t *testing.T) {
	homeDir := t.TempDir()
	workDir := t.TempDir()

	if err := os.MkdirAll(filepath.Join(homeDir, ".config", "gh"), 0755); err != nil {
		t.Fatalf("creating .config/gh: %v", err)
	}

	opts := newTestOpts(homeDir, workDir)
	opts.MountGH = false
	builder := NewBuilder()
	mounts, err := builder.BuildMounts(opts)
	if err != nil {
		t.Fatalf("BuildMounts() error: %v", err)
	}

	if findMount(mounts, "/home/agent/.config/gh") != nil {
		t.Error(".config/gh mount should not exist when MountGH is false")
	}
}

func TestBuildMounts_NoSSHByDefault(t *testing.T) {
	homeDir := t.TempDir()
	workDir := t.TempDir()

	if err := os.MkdirAll(filepath.Join(homeDir, ".ssh"), 0700); err != nil {
		t.Fatalf("creating .ssh: %v", err)
	}

	opts := newTestOpts(homeDir, workDir)
	builder := NewBuilder()
	mounts, err := builder.BuildMounts(opts)
	if err != nil {
		t.Fatalf("BuildMounts() error: %v", err)
	}

	if findMount(mounts, "/home/agent/.ssh-host") != nil {
		t.Fatal(".ssh-host mount should not exist unless mount_ssh is enabled")
	}
}

func TestBuildMounts_SSHReadOnlyWhenEnabled(t *testing.T) {
	homeDir := t.TempDir()
	workDir := t.TempDir()

	if err := os.MkdirAll(filepath.Join(homeDir, ".ssh"), 0700); err != nil {
		t.Fatalf("creating .ssh: %v", err)
	}

	opts := newTestOpts(homeDir, workDir)
	opts.MountSSH = true
	builder := NewBuilder()
	mounts, err := builder.BuildMounts(opts)
	if err != nil {
		t.Fatalf("BuildMounts() error: %v", err)
	}

	m := findMount(mounts, "/home/agent/.ssh-host")
	if m == nil {
		t.Fatal("missing .ssh-host mount")
	}
	if !m.ReadOnly {
		t.Error(".ssh mount should be read-only")
	}
	if m.Source != filepath.Join(homeDir, ".ssh") {
		t.Errorf("source = %q, want %q", m.Source, filepath.Join(homeDir, ".ssh"))
	}
}

func TestBuildMounts_NoSSHWhenMissing(t *testing.T) {
	homeDir := t.TempDir()
	workDir := t.TempDir()

	opts := newTestOpts(homeDir, workDir)
	opts.MountSSH = true
	builder := NewBuilder()
	mounts, err := builder.BuildMounts(opts)
	if err != nil {
		t.Fatalf("BuildMounts() error: %v", err)
	}

	if findMount(mounts, "/home/agent/.ssh-host") != nil {
		t.Error(".ssh-host mount should not exist when .ssh is missing")
	}
}

func TestBuildMounts_WorktreeAddsMount(t *testing.T) {
	// Set up a worktree scenario
	baseDir := t.TempDir()

	mainRepo := filepath.Join(baseDir, "main-repo")
	mainGitDir := filepath.Join(mainRepo, ".git")
	if err := os.MkdirAll(filepath.Join(mainGitDir, "worktrees", "wt"), 0755); err != nil {
		t.Fatalf("creating worktree dir: %v", err)
	}

	worktreeDir := filepath.Join(baseDir, "worktree")
	if err := os.MkdirAll(worktreeDir, 0755); err != nil {
		t.Fatalf("creating worktree dir: %v", err)
	}

	gitdirPath := filepath.Join(mainGitDir, "worktrees", "wt")
	if err := os.WriteFile(filepath.Join(worktreeDir, ".git"), []byte("gitdir: "+gitdirPath+"\n"), 0644); err != nil {
		t.Fatalf("writing .git file: %v", err)
	}

	homeDir := t.TempDir()
	opts := newTestOpts(homeDir, worktreeDir)
	builder := NewBuilder()
	mounts, err := builder.BuildMounts(opts)
	if err != nil {
		t.Fatalf("BuildMounts() error: %v", err)
	}

	absMainGitDir, _ := filepath.Abs(mainGitDir)
	m := findMount(mounts, absMainGitDir)
	if m == nil {
		t.Fatalf("missing worktree mount for %s", absMainGitDir)
	}
	if m.Source != absMainGitDir {
		t.Errorf("source = %q, want %q", m.Source, absMainGitDir)
	}
}

func TestBuildMounts_CodexToolConfig(t *testing.T) {
	homeDir := t.TempDir()
	workDir := t.TempDir()
	codexStageDir := filepath.Join(homeDir, ".agent-workspace", "codex")
	if err := os.MkdirAll(codexStageDir, 0755); err != nil {
		t.Fatalf("creating codex stage dir: %v", err)
	}

	opts := MountOptions{
		HomeDir:          homeDir,
		WorkDir:          workDir,

		ToolStageDir:     codexStageDir,
		ToolContainerDir: "/home/agent/.codex",
	}

	builder := NewBuilder()
	mounts, err := builder.BuildMounts(opts)
	if err != nil {
		t.Fatalf("BuildMounts() error: %v", err)
	}

	// Codex mount should be present at the correct path
	codex := findMount(mounts, "/home/agent/.codex")
	if codex == nil {
		t.Fatal("missing codex config mount")
	}
	if codex.Source != codexStageDir {
		t.Errorf("codex source = %q, want %q", codex.Source, codexStageDir)
	}

	// Claude-specific mounts should NOT be present
	if findMount(mounts, "/home/agent/.claude") != nil {
		t.Error("claude config mount should not exist for codex tool")
	}

	// Workspace should still be present
	if findMount(mounts, workDir) == nil {
		t.Error("workspace mount should still be present for codex")
	}
}

func TestBuildMounts_NoToolConfigSkipsMount(t *testing.T) {
	homeDir := t.TempDir()
	workDir := t.TempDir()

	opts := MountOptions{
		HomeDir:          homeDir,
		WorkDir:          workDir,

		ToolStageDir:     "",
		ToolContainerDir: "",
	}

	builder := NewBuilder()
	mounts, err := builder.BuildMounts(opts)
	if err != nil {
		t.Fatalf("BuildMounts() error: %v", err)
	}

	if findMount(mounts, "/home/agent/.codex") != nil {
		t.Error("codex mount should not exist when ToolStageDir is empty")
	}
	if findMount(mounts, "/home/agent/.claude") != nil {
		t.Error("claude mount should not exist when ToolStageDir is empty")
	}
}

func TestBuildMounts_ToolConfigMountPresent(t *testing.T) {
	homeDir := t.TempDir()
	workDir := t.TempDir()

	opts := newTestOpts(homeDir, workDir)

	builder := NewBuilder()
	mounts, err := builder.BuildMounts(opts)
	if err != nil {
		t.Fatalf("BuildMounts() error: %v", err)
	}

	// Tool config mount should be present at the configured container dir
	if findMount(mounts, "/home/agent/.claude") == nil {
		t.Error("tool config mount should exist when ToolStageDir and ToolContainerDir are set")
	}
}

func TestBuildMounts_SSHAgentForwardingMountsAgentSocket(t *testing.T) {
	homeDir := t.TempDir()
	workDir := t.TempDir()

	// Create a fake socket file
	sockPath := filepath.Join(t.TempDir(), "agent.sock")
	if err := os.WriteFile(sockPath, []byte{}, 0600); err != nil {
		t.Fatalf("creating fake socket: %v", err)
	}

	opts := newTestOpts(homeDir, workDir)
	opts.SSHAgentForwarding = true
	opts.SSHAuthSock = sockPath
	builder := NewBuilder()
	mounts, err := builder.BuildMounts(opts)
	if err != nil {
		t.Fatalf("BuildMounts() error: %v", err)
	}

	m := findMount(mounts, SSHAgentContainerPath)
	if m == nil {
		t.Fatal("missing SSH agent socket mount")
	}
	if m.Source != sockPath {
		t.Errorf("source = %q, want %q", m.Source, sockPath)
	}
	if m.ReadOnly {
		t.Error("SSH agent socket mount should not be read-only")
	}
}

func TestBuildMounts_SSHAgentForwardingSkippedWhenMountSSHEnabled(t *testing.T) {
	homeDir := t.TempDir()
	workDir := t.TempDir()

	sockPath := filepath.Join(t.TempDir(), "agent.sock")
	if err := os.WriteFile(sockPath, []byte{}, 0600); err != nil {
		t.Fatalf("creating fake socket: %v", err)
	}

	if err := os.MkdirAll(filepath.Join(homeDir, ".ssh"), 0700); err != nil {
		t.Fatalf("creating .ssh: %v", err)
	}

	opts := newTestOpts(homeDir, workDir)
	opts.MountSSH = true
	opts.SSHAgentForwarding = true
	opts.SSHAuthSock = sockPath
	builder := NewBuilder()
	mounts, err := builder.BuildMounts(opts)
	if err != nil {
		t.Fatalf("BuildMounts() error: %v", err)
	}

	if findMount(mounts, SSHAgentContainerPath) != nil {
		t.Error("SSH agent socket should not be mounted when mount_ssh is enabled")
	}
	if findMount(mounts, "/home/agent/.ssh-host") == nil {
		t.Error(".ssh-host should be mounted when mount_ssh is enabled")
	}
}

func TestBuildMounts_SSHAgentForwardingNoSocketPath(t *testing.T) {
	homeDir := t.TempDir()
	workDir := t.TempDir()

	opts := newTestOpts(homeDir, workDir)
	opts.SSHAgentForwarding = true
	opts.SSHAuthSock = ""
	builder := NewBuilder()
	mounts, err := builder.BuildMounts(opts)
	if err != nil {
		t.Fatalf("BuildMounts() error: %v", err)
	}

	if findMount(mounts, SSHAgentContainerPath) != nil {
		t.Error("SSH agent socket should not be mounted when SSHAuthSock is empty")
	}
}

func TestBuildMounts_SSHAgentForwardingVMPath(t *testing.T) {
	homeDir := t.TempDir()
	workDir := t.TempDir()

	opts := newTestOpts(homeDir, workDir)
	opts.SSHAgentForwarding = true
	opts.SSHAuthSock = "/tmp/aw-ssh-agent.sock"
	builder := NewBuilder()
	mounts, err := builder.BuildMounts(opts)
	if err != nil {
		t.Fatalf("BuildMounts() error: %v", err)
	}

	m := findMount(mounts, SSHAgentContainerPath)
	if m == nil {
		t.Fatal("SSH agent socket should be mounted even for VM-internal paths")
	}
	if m.Source != "/tmp/aw-ssh-agent.sock" {
		t.Errorf("source = %q, want %q", m.Source, "/tmp/aw-ssh-agent.sock")
	}
}

func TestBuildMounts_NoWorktreeMount_RegularRepo(t *testing.T) {
	homeDir := t.TempDir()
	workDir := t.TempDir()

	// Regular .git directory
	if err := os.MkdirAll(filepath.Join(workDir, ".git"), 0755); err != nil {
		t.Fatalf("creating .git dir: %v", err)
	}

	opts := newTestOpts(homeDir, workDir)
	builder := NewBuilder()
	mounts, err := builder.BuildMounts(opts)
	if err != nil {
		t.Fatalf("BuildMounts() error: %v", err)
	}

	// Should only have the 2 fixed mounts: tool config and workspace
	if len(mounts) != 2 {
		t.Errorf("expected 2 mounts (tool and workspace), got %d: %+v", len(mounts), mounts)
	}
}
