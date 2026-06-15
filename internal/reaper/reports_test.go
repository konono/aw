package reaper

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testReaperHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	return filepath.Join(dir, ".config", "aw", "reaper")
}

func TestListReportsExcludesSpecs(t *testing.T) {
	dir := testReaperHome(t)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "2026-01-01T00-00-00-aw-a.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "aw-a.spec.json"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}

	reports := ListReports()
	if len(reports) != 1 {
		t.Fatalf("len(reports) = %d, want 1", len(reports))
	}
}

func TestRotateReports(t *testing.T) {
	dir := testReaperHome(t)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 12; i++ {
		name := filepath.Join(dir, fmt.Sprintf("2026-01-01T00-00-%02d-aw-a.json", i))
		if err := os.WriteFile(name, []byte("{}"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	rotateReports(10)
	reports := ListReports()
	if len(reports) != 10 {
		t.Fatalf("len(reports) = %d, want 10", len(reports))
	}
}

func TestSaveSpecOmitsTTY(t *testing.T) {
	_ = testReaperHome(t)
	spec := ReaperSpec{
		ContainerName: "aw-tty-test",
		Runtime:       "podman",
		TTY:           "/dev/ttys001",
	}
	path := saveSpec(spec)
	if path == "" {
		t.Fatal("saveSpec returned empty path")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), `"tty"`) {
		t.Fatalf("spec on disk should not contain tty field, got %s", data)
	}
	var saved ReaperSpec
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	if saved.TTY != "" {
		t.Fatalf("saved TTY = %q, want empty", saved.TTY)
	}
}

func TestDetectTTYNonInteractive(t *testing.T) {
	if got := DetectTTY(); got != "" {
		t.Fatalf("DetectTTY() = %q, want empty in non-interactive test", got)
	}
}

func TestRecoverReaperSkipsSucceededTasks(t *testing.T) {
	dir := testReaperHome(t)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}

	container := "aw-recover-skip"
	reportPath := filepath.Join(dir, "2026-06-15T00-00-00-"+container+".json")
	report := RunReport{
		StartedAt:     time.Now(),
		ContainerName: container,
		Tasks: []TaskResult{
			{Type: "shell", Label: "done", Status: "ok"},
		},
	}
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(reportPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	specPath := writeTestSpec(t, dir, container, []ReaperTask{
		{Type: "shell", Label: "done", Config: json.RawMessage(`{"command":"true"}`)},
		{Type: "shell", Label: "fail", Config: json.RawMessage(`{"command":"exit 1"}`)},
	})

	if err := recoverReaper(specPath, container, false); err == nil {
		t.Fatal("expected recover error from fail task")
	}
	if _, err := os.Stat(specPath); err != nil {
		t.Fatal("spec should be kept when recovery fails")
	}
}

func TestSucceededTaskKeys(t *testing.T) {
	dir := testReaperHome(t)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	container := "aw-keys"
	report := RunReport{
		ContainerName: container,
		Tasks: []TaskResult{
			{Type: "kill_process", Label: "ssh-tunnel", Status: "ok"},
			{Type: "remove_file", Label: "vm-socket", Status: "failed"},
		},
	}
	data, _ := json.Marshal(report)
	_ = os.WriteFile(filepath.Join(dir, "2026-06-15T00-00-00-"+container+".json"), data, 0644)

	keys := succeededTaskKeys(container)
	if !keys["ssh-tunnel|kill_process"] {
		t.Fatal("expected ssh-tunnel to be marked succeeded")
	}
	if keys["vm-socket|remove_file"] {
		t.Fatal("vm-socket should not be marked succeeded")
	}
}
