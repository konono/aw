package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestAppendTaskFlags_Claude(t *testing.T) {
	cmd := []string{"claude", "--permission-mode", "bypassPermissions"}
	got := appendTaskFlags("claude", cmd, "implement fizzbuzz")
	want := []string{"claude", "--permission-mode", "bypassPermissions", "--prompt", "implement fizzbuzz"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestAppendTaskFlags_NonClaude(t *testing.T) {
	cmd := []string{"codex", "-a", "never"}
	got := appendTaskFlags("codex", cmd, "implement fizzbuzz")
	if len(got) != len(cmd) {
		t.Errorf("non-claude tool should not add task flags, got %v", got)
	}
}

func TestAppendResumeFlags_Claude(t *testing.T) {
	cmd := []string{"claude"}
	got := appendResumeFlags("claude", cmd)
	if len(got) != 2 || got[1] != "--continue" {
		t.Errorf("got %v, want [claude --continue]", got)
	}
}

func TestAppendResumeFlags_Unknown(t *testing.T) {
	cmd := []string{"vim"}
	got := appendResumeFlags("vim", cmd)
	if len(got) != 1 {
		t.Errorf("unknown tool should not add resume flags, got %v", got)
	}
}

func TestEnsureWorktree(t *testing.T) {
	repoDir := t.TempDir()

	run := func(name string, args ...string) {
		cmd := exec.Command(name, args...)
		cmd.Dir = repoDir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%s %v failed: %v\n%s", name, args, err, out)
		}
	}

	run("git", "init")
	run("git", "checkout", "-b", "main")
	if err := os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	run("git", "add", "README.md")
	run("git", "commit", "-m", "initial")

	wtPath := filepath.Join(repoDir, "worktrees", "aw-test-dev-1")
	branch := "aw/test/developer-1"

	if err := ensureWorktree(repoDir, branch, wtPath, "HEAD", false); err != nil {
		t.Fatalf("ensureWorktree: %v", err)
	}
	if _, err := os.Stat(wtPath); err != nil {
		t.Fatalf("worktree path should exist: %v", err)
	}

	branchCmd := exec.Command("git", "-C", repoDir, "branch", "--list", branch)
	out, err := branchCmd.Output()
	if err != nil {
		t.Fatalf("git branch --list: %v", err)
	}
	if len(out) == 0 {
		t.Error("branch should exist")
	}

	if err := ensureWorktree(repoDir, branch, wtPath, "HEAD", true); err != nil {
		t.Fatalf("ensureWorktree resume: %v", err)
	}

	if err := ensureWorktree(repoDir, branch, wtPath, "HEAD", false); err == nil {
		t.Error("expected error when worktree exists and resume=false")
	}
}

func TestEnsureWorktree_ParentDirCreated(t *testing.T) {
	repoDir := t.TempDir()

	run := func(name string, args ...string) {
		cmd := exec.Command(name, args...)
		cmd.Dir = repoDir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%s %v failed: %v\n%s", name, args, err, out)
		}
	}

	run("git", "init")
	run("git", "checkout", "-b", "main")
	if err := os.WriteFile(filepath.Join(repoDir, "f.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	run("git", "add", "f.txt")
	run("git", "commit", "-m", "init")

	wtPath := filepath.Join(repoDir, "deep", "nested", "worktrees", "aw-test-dev")
	if err := ensureWorktree(repoDir, "aw/test/dev", wtPath, "HEAD", false); err != nil {
		t.Fatalf("ensureWorktree with nested path: %v", err)
	}
	if _, err := os.Stat(wtPath); err != nil {
		t.Fatalf("worktree should exist at nested path: %v", err)
	}
}
