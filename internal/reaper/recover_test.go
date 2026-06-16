package reaper

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func writeTestSpec(t *testing.T, dir, name string, tasks []ReaperTask) string {
	t.Helper()
	specPath := filepath.Join(dir, name+".spec.json")
	spec := ReaperSpec{
		ContainerName: name,
		Runtime:       "podman",
		Tasks:         tasks,
	}
	data, err := json.Marshal(spec)
	if err != nil {
		t.Fatalf("marshal spec: %v", err)
	}
	if err := os.WriteFile(specPath, data, 0600); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	return specPath
}

func TestRecoverReaperKeepsSpecOnFailure(t *testing.T) {
	dir := t.TempDir()
	reaperDirOverride = dir
	t.Cleanup(func() { reaperDirOverride = "" })

	specPath := writeTestSpec(t, dir, "aw-recover-fail", []ReaperTask{{
		Type:   "shell",
		Label:  "fail",
		Config: json.RawMessage(`{"command":"exit 1"}`),
	}})

	if err := recoverReaper(specPath, "aw-recover-fail", false); err == nil {
		t.Fatal("expected recover error")
	}
	if _, err := os.Stat(specPath); err != nil {
		t.Fatal("spec should be kept when recovery fails")
	}
}

func TestRecoverReaperRemovesSpecOnSuccess(t *testing.T) {
	dir := t.TempDir()
	reaperDirOverride = dir
	t.Cleanup(func() { reaperDirOverride = "" })

	specPath := writeTestSpec(t, dir, "aw-recover-ok", []ReaperTask{{
		Type:   "shell",
		Label:  "ok",
		Config: json.RawMessage(`{"command":"true"}`),
	}})

	if err := recoverReaper(specPath, "aw-recover-ok", false); err != nil {
		t.Fatalf("unexpected recover error: %v", err)
	}
	if _, err := os.Stat(specPath); !os.IsNotExist(err) {
		t.Fatal("spec should be removed when recovery succeeds")
	}
}
