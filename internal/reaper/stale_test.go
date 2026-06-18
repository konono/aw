package reaper

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == "--internal-reaper" {
		os.Exit(Run())
	}
	os.Exit(m.Run())
}

func TestCheckStaleContainer(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires shell script execution")
	}
	dir := t.TempDir()
	runtimePath := filepath.Join(dir, "fake-runtime")
	script := `#!/bin/sh
case "$1" in
inspect)
  case "$2" in
  aw-running) echo "true"; exit 0 ;;
  aw-stopped) echo "false"; exit 0 ;;
  esac
  exit 1
  ;;
rm)
  exit 0
  ;;
esac
exit 1
`
	if err := os.WriteFile(runtimePath, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}

	t.Run("missing container is ok", func(t *testing.T) {
		if err := CheckStaleContainer(runtimePath, "aw-missing"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("running container returns error", func(t *testing.T) {
		err := CheckStaleContainer(runtimePath, "aw-running")
		if err == nil {
			t.Fatal("expected error for running container")
		}
	})

	t.Run("stopped container is removed", func(t *testing.T) {
		if err := CheckStaleContainer(runtimePath, "aw-stopped"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestHandleAbort(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	if runtime.GOOS == "windows" {
		t.Setenv("APPDATA", filepath.Join(dir, ".config"))
	}

	specPath := filepath.Join(dir, ".config", "aw", "reaper", "aw-abort-test.spec.json")
	if err := os.MkdirAll(filepath.Dir(specPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(specPath, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}

	handle, err := Spawn(ReaperSpec{
		ContainerName: "aw-abort-test",
		Runtime:       "podman",
	})
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}

	handle.Abort()

	if _, err := os.Stat(specPath); !os.IsNotExist(err) {
		t.Fatalf("spec file should be removed, stat err=%v", err)
	}
}
