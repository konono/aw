package stage

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/konono/aw/v4/internal/pipeline"
	"github.com/konono/aw/v4/internal/platform"
	"github.com/konono/aw/v4/internal/profile"
)

// realPath resolves symlinks so that pwd comparisons work on macOS
// where /var is a symlink to /private/var.
func realPath(t *testing.T, path string) string {
	t.Helper()
	real, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q): %v", path, err)
	}
	return real
}

func TestRunOnCreateHook_ShellInvocation(t *testing.T) {
	origExecCommand := execCommand
	defer func() { execCommand = origExecCommand }()

	var capturedName string
	var capturedArgs []string
	execCommand = func(name string, args ...string) *exec.Cmd {
		capturedName = name
		capturedArgs = args
		return exec.Command("echo")
	}

	ec := &pipeline.ExecutionContext{
		Profile: profile.Profile{
			Worktree:    &profile.WorktreeConfig{OnCreate: "./setup.sh"},
			Environment: profile.EnvironmentContainer,
		},
		ProfileName:    "test-profile",
		WorktreePath:   t.TempDir(),
		WorktreeBranch: "test-branch",
	}

	err := runWorktreeHook(ec.Profile.Worktree.OnCreate, ec, "/fake/repo")
	if err != nil {
		t.Fatalf("runWorktreeHook() error: %v", err)
	}

	shell, shellFlags := platform.ShellCommand()
	if capturedName != shell {
		t.Errorf("expected command %q, got %q", shell, capturedName)
	}
	wantArgs := append(shellFlags, "./setup.sh")
	if len(capturedArgs) != len(wantArgs) || capturedArgs[0] != wantArgs[0] || capturedArgs[len(capturedArgs)-1] != "./setup.sh" {
		t.Errorf("expected args %v, got %v", wantArgs, capturedArgs)
	}
}

func TestRunOnCreateHook_SetsEnvironmentAndDir(t *testing.T) {
	origExecCommand := execCommand
	defer func() { execCommand = origExecCommand }()

	tmpDir := t.TempDir()

	// Use a real command that prints env vars
	execCommand = func(name string, args ...string) *exec.Cmd {
		// Create a real command that will succeed and let us inspect env via the Cmd struct
		cmd := exec.Command("echo")
		return cmd
	}

	ec := &pipeline.ExecutionContext{
		Profile: profile.Profile{
			Worktree:    &profile.WorktreeConfig{OnCreate: "echo test"},
			Environment: profile.EnvironmentContainer,
		},
		ProfileName:    "my-profile",
		WorktreePath:   tmpDir,
		WorktreeBranch: "feature-branch",
	}

	// Instead of mocking, we test with a real shell command that verifies env vars
	execCommand = exec.Command
	// Use a command that checks env vars exist
	ec.Profile.Worktree.OnCreate = "test -n \"$AW_WORKTREE_PATH\" && test -n \"$AW_WORKTREE_BRANCH\" && test -n \"$AW_REPO_ROOT\" && test -n \"$AW_PROFILE_NAME\" && test -n \"$AW_ENVIRONMENT\""

	err := runWorktreeHook(ec.Profile.Worktree.OnCreate, ec, "/fake/repo")
	if err != nil {
		t.Fatalf("runWorktreeHook() error (env vars missing): %v", err)
	}
}

func TestRunOnCreateHook_EnvVarValues(t *testing.T) {
	origExecCommand := execCommand
	defer func() { execCommand = origExecCommand }()

	tmpDir := t.TempDir()
	execCommand = exec.Command

	ec := &pipeline.ExecutionContext{
		Profile: profile.Profile{
			Worktree:    &profile.WorktreeConfig{OnCreate: "test \"$AW_PROFILE_NAME\" = \"special-profile\" && test \"$AW_ENVIRONMENT\" = \"host\" && test \"$AW_WORKTREE_BRANCH\" = \"my-branch\""},
			Environment: profile.EnvironmentHost,
		},
		ProfileName:    "special-profile",
		WorktreePath:   tmpDir,
		WorktreeBranch: "my-branch",
	}

	err := runWorktreeHook(ec.Profile.Worktree.OnCreate, ec, "/some/repo")
	if err != nil {
		t.Fatalf("runWorktreeHook() env var values mismatch: %v", err)
	}
}

func TestRunOnCreateHook_WorkingDirectory(t *testing.T) {
	origExecCommand := execCommand
	defer func() { execCommand = origExecCommand }()

	tmpDir := realPath(t, t.TempDir())
	execCommand = exec.Command

	// pwd should match the worktree path
	ec := &pipeline.ExecutionContext{
		Profile: profile.Profile{
			Worktree:    &profile.WorktreeConfig{OnCreate: "test \"$(pwd)\" = \"" + tmpDir + "\""},
			Environment: profile.EnvironmentHost,
		},
		ProfileName:    "test",
		WorktreePath:   tmpDir,
		WorktreeBranch: "branch",
	}

	err := runWorktreeHook(ec.Profile.Worktree.OnCreate, ec, "/repo")
	if err != nil {
		t.Fatalf("runWorktreeHook() working directory mismatch: %v", err)
	}
}

func TestRunOnCreateHook_FailureReturnsError(t *testing.T) {
	origExecCommand := execCommand
	defer func() { execCommand = origExecCommand }()

	execCommand = exec.Command

	ec := &pipeline.ExecutionContext{
		Profile: profile.Profile{
			Worktree:    &profile.WorktreeConfig{OnCreate: "exit 1"},
			Environment: profile.EnvironmentHost,
		},
		ProfileName:    "test",
		WorktreePath:   t.TempDir(),
		WorktreeBranch: "branch",
	}

	err := runWorktreeHook(ec.Profile.Worktree.OnCreate, ec, "/repo")
	if err == nil {
		t.Fatal("expected error from failing hook, got nil")
	}
}

func TestRunOnEndHook_ShellInvocation(t *testing.T) {
	origExecCommand := execCommand
	defer func() { execCommand = origExecCommand }()

	var capturedName string
	var capturedArgs []string
	execCommand = func(name string, args ...string) *exec.Cmd {
		capturedName = name
		capturedArgs = args
		return exec.Command("echo")
	}

	ec := &pipeline.ExecutionContext{
		Profile: profile.Profile{
			Worktree:    &profile.WorktreeConfig{OnEnd: "./cleanup.sh"},
			Environment: profile.EnvironmentContainer,
		},
		ProfileName:    "test-profile",
		WorktreePath:   t.TempDir(),
		WorktreeBranch: "test-branch",
		RepoRoot:       "/fake/repo",
	}

	err := RunOnEndHook(ec)
	if err != nil {
		t.Fatalf("RunOnEndHook() error: %v", err)
	}

	shell, shellFlags := platform.ShellCommand()
	if capturedName != shell {
		t.Errorf("expected command %q, got %q", shell, capturedName)
	}
	wantArgs := append(shellFlags, "./cleanup.sh")
	if len(capturedArgs) != len(wantArgs) || capturedArgs[0] != wantArgs[0] || capturedArgs[len(capturedArgs)-1] != "./cleanup.sh" {
		t.Errorf("expected args %v, got %v", wantArgs, capturedArgs)
	}
}

func TestRunOnEndHook_SetsEnvironmentAndDir(t *testing.T) {
	origExecCommand := execCommand
	defer func() { execCommand = origExecCommand }()

	tmpDir := t.TempDir()
	execCommand = exec.Command

	ec := &pipeline.ExecutionContext{
		Profile: profile.Profile{
			Worktree:    &profile.WorktreeConfig{OnEnd: "test -n \"$AW_WORKTREE_PATH\" && test -n \"$AW_WORKTREE_BRANCH\" && test -n \"$AW_REPO_ROOT\" && test -n \"$AW_PROFILE_NAME\" && test -n \"$AW_ENVIRONMENT\""},
			Environment: profile.EnvironmentContainer,
		},
		ProfileName:    "my-profile",
		WorktreePath:   tmpDir,
		WorktreeBranch: "feature-branch",
		RepoRoot:       "/fake/repo",
	}

	err := RunOnEndHook(ec)
	if err != nil {
		t.Fatalf("RunOnEndHook() error (env vars missing): %v", err)
	}
}

func TestRunOnEndHook_EnvVarValues(t *testing.T) {
	origExecCommand := execCommand
	defer func() { execCommand = origExecCommand }()

	tmpDir := t.TempDir()
	execCommand = exec.Command

	ec := &pipeline.ExecutionContext{
		Profile: profile.Profile{
			Worktree:    &profile.WorktreeConfig{OnEnd: "test \"$AW_PROFILE_NAME\" = \"special-profile\" && test \"$AW_ENVIRONMENT\" = \"host\" && test \"$AW_WORKTREE_BRANCH\" = \"my-branch\""},
			Environment: profile.EnvironmentHost,
		},
		ProfileName:    "special-profile",
		WorktreePath:   tmpDir,
		WorktreeBranch: "my-branch",
		RepoRoot:       "/some/repo",
	}

	err := RunOnEndHook(ec)
	if err != nil {
		t.Fatalf("RunOnEndHook() env var values mismatch: %v", err)
	}
}

func TestRunOnEndHook_WorkingDirectory(t *testing.T) {
	origExecCommand := execCommand
	defer func() { execCommand = origExecCommand }()

	tmpDir := realPath(t, t.TempDir())
	execCommand = exec.Command

	ec := &pipeline.ExecutionContext{
		Profile: profile.Profile{
			Worktree:    &profile.WorktreeConfig{OnEnd: "test \"$(pwd)\" = \"" + tmpDir + "\""},
			Environment: profile.EnvironmentHost,
		},
		ProfileName:    "test",
		WorktreePath:   tmpDir,
		WorktreeBranch: "branch",
		RepoRoot:       "/repo",
	}

	err := RunOnEndHook(ec)
	if err != nil {
		t.Fatalf("RunOnEndHook() working directory mismatch: %v", err)
	}
}

func TestRunOnEndHook_FailureReturnsError(t *testing.T) {
	origExecCommand := execCommand
	defer func() { execCommand = origExecCommand }()

	execCommand = exec.Command

	ec := &pipeline.ExecutionContext{
		Profile: profile.Profile{
			Worktree:    &profile.WorktreeConfig{OnEnd: "exit 1"},
			Environment: profile.EnvironmentHost,
		},
		ProfileName:    "test",
		WorktreePath:   t.TempDir(),
		WorktreeBranch: "branch",
		RepoRoot:       "/repo",
	}

	err := RunOnEndHook(ec)
	if err == nil {
		t.Fatal("expected error from failing hook, got nil")
	}
}

func TestResolveWorktreesDir_Default(t *testing.T) {
	repoRoot := filepath.Join(t.TempDir(), "repo")
	ec := &pipeline.ExecutionContext{
		Profile: profile.Profile{},
	}
	got, err := resolveWorktreesDir(ec, repoRoot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(repoRoot, "worktrees")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolveWorktreesDir_AbsoluteOverride(t *testing.T) {
	absDir := filepath.Join(t.TempDir(), "abs", "wt")
	ec := &pipeline.ExecutionContext{
		Profile: profile.Profile{
			Worktree: &profile.WorktreeConfig{Dir: absDir},
		},
	}
	got, err := resolveWorktreesDir(ec, filepath.Join(t.TempDir(), "repo"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != absDir {
		t.Errorf("got %q, want %q", got, absDir)
	}
}

func TestResolveWorktreesDir_RelativeIsRepoRelative(t *testing.T) {
	base := t.TempDir()
	repoSub := filepath.Join(base, "repo", "sub")
	ec := &pipeline.ExecutionContext{
		Profile: profile.Profile{
			Worktree: &profile.WorktreeConfig{Dir: "../shared-worktrees"},
		},
	}
	got, err := resolveWorktreesDir(ec, repoSub)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(base, "repo", "shared-worktrees")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolveWorktreesDir_TildeExpansion(t *testing.T) {
	home, _ := os.UserHomeDir()
	ec := &pipeline.ExecutionContext{
		HomeDir: home,
		Profile: profile.Profile{
			Worktree: &profile.WorktreeConfig{Dir: "~/aw-wt"},
		},
	}
	got, err := resolveWorktreesDir(ec, filepath.Join(t.TempDir(), "repo"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(home, "aw-wt")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
