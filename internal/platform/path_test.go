package platform

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestWindowsToContainerPath(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"standard path", `C:\Users\foo\project`, "/c/Users/foo/project"},
		{"different drive", `D:\work`, "/d/work"},
		{"drive root", `C:\`, "/c/"},
		{"linux path passthrough", "/already/linux/path", "/already/linux/path"},
		{"empty string", "", ""},
		{"drive letter only", `C:`, "/c"},
		{"lowercase drive", `c:\foo`, "/c/foo"},
		{"mixed separators", `C:\foo/bar\baz`, "/c/foo/bar/baz"},
		{"UNC path", `\\server\share\file`, "//server/share/file"},
		{"deeply nested", `C:\a\b\c\d\e\f`, "/c/a/b/c/d/e/f"},
		{"with spaces", `C:\Program Files\app`, "/c/Program Files/app"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := windowsToContainerPath(tt.input)
			if got != tt.want {
				t.Errorf("windowsToContainerPath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestContainerPathToWindows(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"standard path", "/c/Users/foo/project", `C:\Users\foo\project`},
		{"different drive", "/d/work", `D:\work`},
		{"drive root", "/c/", `C:\`},
		{"linux path no drive", "/home/user/project", `\home\user\project`},
		{"empty string", "", ""},
		{"single slash", "/", `\`},
		{"short path", "/c", `\c`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containerPathToWindows(tt.input)
			if got != tt.want {
				t.Errorf("containerPathToWindows(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFromContainerPath_NoopOnUnix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix-only test")
	}
	paths := []string{
		"/home/user/project",
		"/c/Users/foo",
		"/tmp/foo",
	}
	for _, path := range paths {
		got := FromContainerPath(path)
		if got != path {
			t.Errorf("FromContainerPath(%q) = %q, want unchanged on unix", path, got)
		}
	}
}

func TestRoundTrip_WindowsPaths(t *testing.T) {
	paths := []struct {
		windows   string
		container string
	}{
		{`C:\Users\foo\project`, "/c/Users/foo/project"},
		{`D:\work`, "/d/work"},
		{`C:\`, "/c/"},
	}
	for _, tt := range paths {
		t.Run(tt.windows, func(t *testing.T) {
			got := windowsToContainerPath(tt.windows)
			if got != tt.container {
				t.Errorf("windowsToContainerPath(%q) = %q, want %q", tt.windows, got, tt.container)
			}
			back := containerPathToWindows(got)
			if back != tt.windows {
				t.Errorf("containerPathToWindows(%q) = %q, want roundtrip to %q", got, back, tt.windows)
			}
		})
	}
}

func TestToContainerPath_NoopOnUnix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix-only test")
	}
	paths := []string{
		"/home/user/project",
		"/var/run/docker.sock",
		"/tmp/foo",
		"relative/path",
	}
	for _, path := range paths {
		got := ToContainerPath(path)
		if got != path {
			t.Errorf("ToContainerPath(%q) = %q, want unchanged on unix", path, got)
		}
	}
}

func TestConfigDir_ReturnsNonEmpty(t *testing.T) {
	dir := ConfigDir()
	if dir == "" {
		t.Fatal("ConfigDir() returned empty string")
	}
	if !filepath.IsAbs(dir) {
		t.Errorf("ConfigDir() = %q, want absolute path", dir)
	}
	if !strings.HasSuffix(dir, "aw") {
		t.Errorf("ConfigDir() = %q, want path ending with 'aw'", dir)
	}
}

func TestStateDir_ReturnsNonEmpty(t *testing.T) {
	dir := StateDir()
	if dir == "" {
		t.Fatal("StateDir() returned empty string")
	}
	if !filepath.IsAbs(dir) {
		t.Errorf("StateDir() = %q, want absolute path", dir)
	}
	if !strings.HasSuffix(dir, "aw") {
		t.Errorf("StateDir() = %q, want path ending with 'aw'", dir)
	}
}

func TestConfigDir_And_StateDir_AreDifferent(t *testing.T) {
	config := ConfigDir()
	state := StateDir()
	if config == state {
		t.Errorf("ConfigDir() and StateDir() returned the same path: %q", config)
	}
}

func TestStateDir_RespectsXDG(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("XDG is not used on Windows")
	}
	customDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", customDir)

	dir := StateDir()
	if !strings.HasPrefix(dir, customDir) {
		t.Errorf("StateDir() = %q, want prefix %q when XDG_STATE_HOME is set", dir, customDir)
	}
}

func TestStateDir_ChangesWithXDG(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("XDG is not used on Windows")
	}
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	t.Setenv("XDG_STATE_HOME", dir1)
	state1 := StateDir()

	t.Setenv("XDG_STATE_HOME", dir2)
	state2 := StateDir()

	if state1 == state2 {
		t.Error("StateDir() returned same path for different XDG_STATE_HOME values")
	}
}

func TestTempSocketPath_ContainsContainerName(t *testing.T) {
	name := "my-container"
	path := TempSocketPath(name)
	if !strings.Contains(path, name) {
		t.Errorf("TempSocketPath(%q) = %q, want path containing container name", name, path)
	}
}

func TestTempSocketPath_UniquePerContainer(t *testing.T) {
	path1 := TempSocketPath("container-a")
	path2 := TempSocketPath("container-b")
	if path1 == path2 {
		t.Errorf("TempSocketPath returned same path for different containers: %q", path1)
	}
}

func TestTempSocketPath_HasSockExtension(t *testing.T) {
	path := TempSocketPath("test")
	if !strings.HasSuffix(path, ".sock") {
		t.Errorf("TempSocketPath() = %q, want .sock extension", path)
	}
}
