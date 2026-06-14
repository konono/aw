package pathutil

import (
	"path/filepath"
	"testing"
)

func TestExpandTilde(t *testing.T) {
	home := "/home/agent"

	tests := []struct {
		name string
		path string
		want string
	}{
		{"absolute unchanged", "/absolute/path", "/absolute/path"},
		{"relative unchanged", "relative/path", "relative/path"},
		{"home slash expanded", "~/somewhere", filepath.Join(home, "somewhere")},
		{"bare tilde unchanged", "~", "~"},
		{"other user unchanged", "~otheruser/dir", "~otheruser/dir"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExpandTilde(tt.path, home)
			if got != tt.want {
				t.Errorf("ExpandTilde(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}
