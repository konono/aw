package cmd

import "testing"

func TestIsWorktreePath(t *testing.T) {
	tests := []struct {
		dir     string
		origDir string
		want    bool
	}{
		{"/home/user/src/aw", "/home/user/src/aw", false},
		{"/home/user/src/aw/worktrees/abc-def-ghi", "/home/user/src/aw", true},
		{"/tmp/worktrees/test", "/home/user/src/aw", true},
		{"/home/user/src/other", "/home/user/src/aw", false},
	}

	for _, tt := range tests {
		t.Run(tt.dir, func(t *testing.T) {
			got := isWorktreePath(tt.dir, tt.origDir)
			if got != tt.want {
				t.Errorf("isWorktreePath(%q, %q) = %v, want %v", tt.dir, tt.origDir, got, tt.want)
			}
		})
	}
}

func TestExpandTilde(t *testing.T) {
	result := expandTilde("/absolute/path")
	if result != "/absolute/path" {
		t.Errorf("expected no change for absolute path, got %q", result)
	}

	result = expandTilde("relative/path")
	if result != "relative/path" {
		t.Errorf("expected no change for relative path, got %q", result)
	}

	result = expandTilde("~/somewhere")
	if result == "~/somewhere" {
		t.Error("expected tilde expansion")
	}
}
