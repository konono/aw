package mount

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/konono/aw/internal/docker"
)

func newTestOpts(homeDir, workDir string) MountOptions {
	return MountOptions{
		HomeDir:             homeDir,
		WorkDir:             workDir,
		ClaudeHome:          filepath.Join(homeDir, ".claude"),
		ContainerClaudeHome: filepath.Join(homeDir, ".agent-workspace"),
		ContainerClaudeJSON: filepath.Join(homeDir, ".agent-workspace.json"),
		VolumeName:          "claude-code-local",
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

	// Volume mount
	vol := findMount(mounts, "/home/claude/.local")
	if vol == nil {
		t.Fatal("missing volume mount for /home/claude/.local")
	}
	if vol.Source != "claude-code-local" || !vol.IsVolume {
		t.Errorf("volume mount = %+v, want source=claude-code-local, IsVolume=true", vol)
	}

	// Claude config mount
	cfg := findMount(mounts, "/home/claude/.claude")
	if cfg == nil {
		t.Fatal("missing mount for /home/claude/.claude")
	}
	if cfg.Source != opts.ContainerClaudeHome {
		t.Errorf("claude config source = %q, want %q", cfg.Source, opts.ContainerClaudeHome)
	}

	// Claude JSON mount
	json := findMount(mounts, "/home/claude/.claude.json")
	if json == nil {
		t.Fatal("missing mount for /home/claude/.claude.json")
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

	m := findMount(mounts, "/home/claude/.gitconfig")
	if m == nil {
		t.Fatal("missing .gitconfig mount")
	}
	if m.Source != filepath.Join(homeDir, ".gitconfig") {
		t.Errorf("source = %q, want %q", m.Source, filepath.Join(homeDir, ".gitconfig"))
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

	if findMount(mounts, "/home/claude/.gitconfig") != nil {
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

	m := findMount(mounts, "/home/claude/.config/gh")
	if m == nil {
		t.Fatal("missing .config/gh mount")
	}
}

func TestBuildMounts_SSHReadOnly(t *testing.T) {
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

	m := findMount(mounts, "/home/claude/.ssh-host")
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
	builder := NewBuilder()
	mounts, err := builder.BuildMounts(opts)
	if err != nil {
		t.Fatalf("BuildMounts() error: %v", err)
	}

	if findMount(mounts, "/home/claude/.ssh-host") != nil {
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

func TestBuildMounts_CodexSkipsClaudeMounts(t *testing.T) {
	homeDir := t.TempDir()
	workDir := t.TempDir()
	codexHome := filepath.Join(homeDir, ".agent-workspace-codex")
	if err := os.MkdirAll(codexHome, 0755); err != nil {
		t.Fatalf("creating codex home: %v", err)
	}

	opts := newTestOpts(homeDir, workDir)
	opts.Tool = "codex"
	opts.ContainerCodexHome = codexHome

	builder := NewBuilder()
	mounts, err := builder.BuildMounts(opts)
	if err != nil {
		t.Fatalf("BuildMounts() error: %v", err)
	}

	// Claude-specific mounts should NOT be present
	if findMount(mounts, "/home/claude/.claude") != nil {
		t.Error("claude config mount should not exist for codex tool")
	}
	if findMount(mounts, "/home/claude/.claude.json") != nil {
		t.Error("claude json mount should not exist for codex tool")
	}

	// Codex mount should be present
	codex := findMount(mounts, "/home/claude/.codex")
	if codex == nil {
		t.Fatal("missing codex config mount")
	}
	if codex.Source != codexHome {
		t.Errorf("codex source = %q, want %q", codex.Source, codexHome)
	}

	// Volume and workspace should still be present
	if findMount(mounts, "/home/claude/.local") == nil {
		t.Error("volume mount should still be present for codex")
	}
	if findMount(mounts, workDir) == nil {
		t.Error("workspace mount should still be present for codex")
	}
}

func TestBuildMounts_CodexNoCodexHomeSkipsMount(t *testing.T) {
	homeDir := t.TempDir()
	workDir := t.TempDir()

	opts := newTestOpts(homeDir, workDir)
	opts.Tool = "codex"
	opts.ContainerCodexHome = ""

	builder := NewBuilder()
	mounts, err := builder.BuildMounts(opts)
	if err != nil {
		t.Fatalf("BuildMounts() error: %v", err)
	}

	if findMount(mounts, "/home/claude/.codex") != nil {
		t.Error("codex mount should not exist when ContainerCodexHome is empty")
	}
	if findMount(mounts, "/home/claude/.claude") != nil {
		t.Error("claude mount should not exist for codex tool")
	}
}

func TestBuildMounts_EmptyToolDefaultsToClaude(t *testing.T) {
	homeDir := t.TempDir()
	workDir := t.TempDir()

	opts := newTestOpts(homeDir, workDir)
	// Tool is empty (default)

	builder := NewBuilder()
	mounts, err := builder.BuildMounts(opts)
	if err != nil {
		t.Fatalf("BuildMounts() error: %v", err)
	}

	// Claude mounts should be present
	if findMount(mounts, "/home/claude/.claude") == nil {
		t.Error("claude config mount should exist for default (empty) tool")
	}
	if findMount(mounts, "/home/claude/.claude.json") == nil {
		t.Error("claude json mount should exist for default (empty) tool")
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

	// Should only have the 4 fixed mounts (no optional ones since homeDir is empty)
	if len(mounts) != 4 {
		t.Errorf("expected 4 mounts (fixed only), got %d: %+v", len(mounts), mounts)
	}
}
