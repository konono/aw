package linuxbin

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestWalkUpForGoMod(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "project")
	nested := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module github.com/konono/aw\n"), 0644); err != nil {
		t.Fatal(err)
	}

	got := walkUpForGoMod(nested)
	if got != root {
		t.Errorf("walkUpForGoMod(%q) = %q, want %q", nested, got, root)
	}
}

func TestWalkUpForGoMod_WrongModule(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module github.com/other/project\n"), 0644); err != nil {
		t.Fatal(err)
	}

	got := walkUpForGoMod(tmp)
	if got != "" {
		t.Errorf("walkUpForGoMod(%q) = %q, want empty", tmp, got)
	}
}

func TestWalkUpForGoMod_NoGoMod(t *testing.T) {
	tmp := t.TempDir()
	got := walkUpForGoMod(tmp)
	if got != "" {
		t.Errorf("walkUpForGoMod(%q) = %q, want empty", tmp, got)
	}
}

func TestAtomicWrite(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "subdir", "testbin")
	data := []byte("binary content")

	if err := atomicWrite(path, data); err != nil {
		t.Fatalf("atomicWrite: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != string(data) {
		t.Errorf("content = %q, want %q", got, data)
	}

	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("Stat: %v", err)
		}
		if info.Mode().Perm() != 0755 {
			t.Errorf("permissions = %o, want 0755", info.Mode().Perm())
		}
	}
}

func TestResolve_CacheHit(t *testing.T) {
	if runtime.GOOS == "linux" {
		t.Skip("cache hit test only runs on non-Linux hosts")
	}

	tmp := t.TempDir()
	var cachePath string
	if runtime.GOOS == "windows" {
		t.Setenv("LOCALAPPDATA", tmp)
		cachePath = filepath.Join(tmp, "aw", "cache", "bin", "aw-linux-arm64-3.3.1")
	} else {
		t.Setenv("XDG_CACHE_HOME", tmp)
		cachePath = filepath.Join(tmp, "aw", "bin", "aw-linux-arm64-3.3.1")
	}

	if err := os.MkdirAll(filepath.Dir(cachePath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cachePath, []byte("cached binary"), 0755); err != nil {
		t.Fatal(err)
	}

	got, err := Resolve("arm64")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got != cachePath {
		t.Errorf("Resolve() = %q, want %q", got, cachePath)
	}
}
