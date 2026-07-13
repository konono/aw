package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestApplyTargetFile_MissingConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir := filepath.Join(home, "work")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(orig) })

	_, err = applyTargetFile("nonexistent-profile-xyz")
	if err == nil {
		t.Fatal("expected error when config file is missing")
	}
}

func TestApplyTargetFile_FromProjectConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir := filepath.Join(home, "work")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(dir, ".aw.yml")
	if err := os.WriteFile(cfgPath, []byte("profiles:\n  test:\n    launch: claude\n"), 0644); err != nil {
		t.Fatal(err)
	}

	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(orig) })

	got, err := applyTargetFile("test")
	if err != nil {
		t.Fatalf("applyTargetFile: %v", err)
	}
	if got != cfgPath {
		t.Fatalf("got %q, want %q", got, cfgPath)
	}
}
