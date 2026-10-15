package cmd

import (
	"regexp"
	"strings"
	"testing"
)

func TestSanitizeNameSegment(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"already safe", "claude", "claude"},
		{"hyphenated profile", "test-debian12-claude", "test-debian12-claude"},
		{"underscores fold to hyphens", "20261009_aap_500_error_shm_resize", "20261009-aap-500-error-shm-resize"},
		{"uppercase folds down", "MyRepo", "myrepo"},
		{"spaces and dots", "my project.v2", "my-project.v2"},
		{"runs collapse", "a///b", "a-b"},
		{"edges trimmed", "-_repo_-", "repo"},
		{"non-ascii", "プロジェクト", ""},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sanitizeNameSegment(tt.input); got != tt.want {
				t.Errorf("sanitizeNameSegment(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// containerNameRe is the pattern podman and docker accept for --name, and the
// one the stale-container filters in internal/reaper look for.
var containerNameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`)

var awFilterRe = regexp.MustCompile(`^aw-.*-[0-9a-f]+$`)

func TestGenerateContainerName(t *testing.T) {
	tests := []struct {
		name       string
		profile    string
		workDir    string
		wantPrefix string
	}{
		{"profile and workspace", "claude", "/tmp/20261009_aap_500_error_shm_resize", "aw-claude-20261009-aap-500-error-shm-resize-"},
		{"hyphenated profile", "test-debian12-claude", "/tmp/myrepo", "aw-test-debian12-claude-myrepo-"},
		{"workspace unusable", "claude", "/", "aw-claude-"},
		{"no workdir", "claude", "", "aw-claude-"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := generateContainerName(tt.profile, tt.workDir)
			if !strings.HasPrefix(got, tt.wantPrefix) {
				t.Errorf("generateContainerName(%q, %q) = %q, want prefix %q", tt.profile, tt.workDir, got, tt.wantPrefix)
			}
			if suffix := strings.TrimPrefix(got, tt.wantPrefix); len(suffix) != 6 {
				t.Errorf("suffix of %q = %q, want 6 characters", got, suffix)
			}
			if !containerNameRe.MatchString(got) {
				t.Errorf("%q is not a name the container runtime accepts", got)
			}
			// The reaper only finds containers its name filter matches; a name
			// it misses would leak past stale detection and cleanup.
			if !awFilterRe.MatchString(got) {
				t.Errorf("%q does not match the aw container filter", got)
			}
		})
	}
}

func TestGenerateContainerNameIsUnique(t *testing.T) {
	seen := make(map[string]bool)
	for range 100 {
		name := generateContainerName("claude", "/tmp/myrepo")
		if seen[name] {
			t.Fatalf("generateContainerName produced a duplicate: %q", name)
		}
		seen[name] = true
	}
}
