package reaper

import (
	"os"
	"path/filepath"
	"testing"
)

func TestShouldCollectDump(t *testing.T) {
	tests := []struct {
		name        string
		exitCode    int
		collectLogs string
		want        bool
	}{
		{"on_failure with exit 0", 0, "on_failure", false},
		{"on_failure with exit 1", 1, "on_failure", true},
		{"on_failure with exit 137", 137, "on_failure", true},
		{"always with exit 0", 0, "always", true},
		{"always with exit 1", 1, "always", true},
		{"never with exit 0", 0, "never", false},
		{"never with exit 1", 1, "never", false},
		{"empty string defaults to on_failure exit 0", 0, "", false},
		{"empty string defaults to on_failure exit 1", 1, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldCollectDump(tt.exitCode, tt.collectLogs)
			if got != tt.want {
				t.Errorf("shouldCollectDump(%d, %q) = %v, want %v",
					tt.exitCode, tt.collectLogs, got, tt.want)
			}
		})
	}
}

func TestShowDump_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	code := ShowDump(dir)
	if code != 0 {
		t.Errorf("ShowDump on empty dir returned %d, want 0", code)
	}
}

func TestShowDump_WithFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "logs.txt"), []byte("test log output"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "inspect.json"), []byte(`{"test": true}`), 0644); err != nil {
		t.Fatal(err)
	}

	code := ShowDump(dir)
	if code != 0 {
		t.Errorf("ShowDump returned %d, want 0", code)
	}
}

func TestShowDump_NonexistentDir(t *testing.T) {
	code := ShowDump(filepath.Join(t.TempDir(), "nonexistent-subdir"))
	if code != 1 {
		t.Errorf("ShowDump on nonexistent dir returned %d, want 1", code)
	}
}

func TestDumpDir(t *testing.T) {
	dir := DumpDir()
	if dir == "" {
		t.Error("DumpDir() returned empty string")
	}
	if !filepath.IsAbs(dir) {
		t.Errorf("DumpDir() = %q, want absolute path", dir)
	}
}
