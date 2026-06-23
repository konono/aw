package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/konono/aw/internal/messaging"
)

func TestLatestCommitInfo(t *testing.T) {
	dir := t.TempDir()

	run := func(name string, args ...string) {
		cmd := exec.Command(name, args...)
		cmd.Dir = dir
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

	testFile := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	run("git", "add", "test.txt")
	run("git", "commit", "-m", "initial commit")

	// Change to the temp dir to test latestCommitInfo
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(dir)

	msg, hash := latestCommitInfo()
	if hash == "" {
		t.Fatal("expected non-empty hash")
	}
	if len(hash) != 40 {
		t.Errorf("expected 40-char hash, got %d chars: %s", len(hash), hash)
	}
	if msg != "initial commit" {
		t.Errorf("message = %q, want %q", msg, "initial commit")
	}
}

func TestOnCommitSendsToReviewers(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "messages.db")

	store, err := messaging.OpenStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	_ = store.Close()

	// Set up environment
	t.Setenv("AW_MSG_DB", dbPath)
	t.Setenv("AW_AGENT_NAME", "developer-1")
	t.Setenv("AW_TEAM_NAME", "test-team")
	t.Setenv("AW_REVIEWERS", "reviewer-1,reviewer-2")

	// Create a git repo with a commit
	gitDir := t.TempDir()
	run := func(name string, args ...string) {
		cmd := exec.Command(name, args...)
		cmd.Dir = gitDir
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
	testFile := filepath.Join(gitDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	run("git", "add", "test.txt")
	run("git", "commit", "-m", "test commit message")

	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(gitDir)

	code := runInternalOnCommit(nil)
	if code != 0 {
		t.Fatalf("runInternalOnCommit returned %d", code)
	}

	// Verify messages were sent
	store, err = messaging.OpenStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	inbox1, err := store.ReadInbox("test-team", "reviewer-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(inbox1) != 1 {
		t.Errorf("reviewer-1 should have 1 message, got %d", len(inbox1))
	}

	inbox2, err := store.ReadInbox("test-team", "reviewer-2")
	if err != nil {
		t.Fatal(err)
	}
	if len(inbox2) != 1 {
		t.Errorf("reviewer-2 should have 1 message, got %d", len(inbox2))
	}
}

func TestOnCommitNoReviewers(t *testing.T) {
	t.Setenv("AW_MSG_DB", "/tmp/test.db")
	t.Setenv("AW_AGENT_NAME", "developer-1")
	t.Setenv("AW_TEAM_NAME", "test-team")
	t.Setenv("AW_REVIEWERS", "")

	code := runInternalOnCommit(nil)
	if code != 0 {
		t.Errorf("expected 0 exit code when no reviewers, got %d", code)
	}
}
