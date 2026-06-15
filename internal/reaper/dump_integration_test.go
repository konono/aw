//go:build integration

package reaper

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIntegration_CollectDump(t *testing.T) {
	runtime := os.Getenv("AW_TEST_RUNTIME")
	if runtime == "" {
		runtime = "podman"
	}
	containerName := os.Getenv("AW_TEST_CONTAINER")
	if containerName == "" {
		t.Skip("AW_TEST_CONTAINER not set; run: podman run --name aw-dump-test alpine sh -c 'echo test && exit 42'")
	}

	dumpPath := collectDump(runtime, containerName, 42, "on_failure")
	if dumpPath == "" {
		t.Fatal("collectDump returned empty path for on_failure with exit 42")
	}
	t.Logf("dump path: %s", dumpPath)
	defer os.RemoveAll(dumpPath)

	for _, name := range []string{"logs.txt", "inspect.json"} {
		path := filepath.Join(dumpPath, name)
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("%s not created: %v", name, err)
			continue
		}
		if info.Size() == 0 {
			t.Errorf("%s is empty", name)
		}
		t.Logf("%s: %d bytes", name, info.Size())
	}

	noDump := collectDump(runtime, containerName, 42, "never")
	if noDump != "" {
		t.Errorf("collectDump with 'never' should return empty, got: %s", noDump)
		os.RemoveAll(noDump)
	}

	noDump2 := collectDump(runtime, containerName, 0, "on_failure")
	if noDump2 != "" {
		t.Errorf("collectDump with exit 0 + on_failure should return empty, got: %s", noDump2)
		os.RemoveAll(noDump2)
	}
}
