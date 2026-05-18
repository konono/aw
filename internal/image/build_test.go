package image

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEmbeddedFilesNotEmpty(t *testing.T) {
	if len(dockerfile) == 0 {
		t.Error("embedded Dockerfile is empty")
	}
	if len(entrypointSh) == 0 {
		t.Error("embedded entrypoint.sh is empty")
	}
}

func TestEmbeddedDockerfileContent(t *testing.T) {
	content := string(dockerfile)
	if !strings.Contains(content, "FROM debian:bookworm-slim") {
		t.Error("Dockerfile should start with FROM debian:bookworm-slim")
	}
	if !strings.Contains(content, "ENTRYPOINT") {
		t.Error("Dockerfile should contain ENTRYPOINT")
	}
	if !strings.Contains(content, "useradd") {
		t.Error("Dockerfile should create claude user")
	}
}

func TestEmbeddedEntrypointContent(t *testing.T) {
	content := string(entrypointSh)
	if !strings.HasPrefix(content, "#!/bin/bash") {
		t.Error("entrypoint.sh should start with shebang")
	}
	if !strings.Contains(content, "setpriv") {
		t.Error("entrypoint.sh should use setpriv to switch user")
	}
	if !strings.Contains(content, "AW_HOST_CONFIG_HOME") {
		t.Error("entrypoint.sh should reference AW_HOST_CONFIG_HOME")
	}
}

func TestDefaultDockerfile(t *testing.T) {
	content := DefaultDockerfile()
	if len(content) == 0 {
		t.Error("DefaultDockerfile() returned empty content")
	}
	if string(content) != string(dockerfile) {
		t.Error("DefaultDockerfile() content does not match embedded dockerfile")
	}
}

func TestPrepareBuildContext(t *testing.T) {
	dir, cleanup, err := PrepareBuildContext("")
	if err != nil {
		t.Fatalf("PrepareBuildContext() error: %v", err)
	}
	defer cleanup()

	// Directory should exist
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("build context dir does not exist: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("build context path is not a directory")
	}

	// Dockerfile should exist with correct content
	dfContent, err := os.ReadFile(filepath.Join(dir, "Dockerfile"))
	if err != nil {
		t.Fatalf("reading Dockerfile: %v", err)
	}
	if string(dfContent) != string(dockerfile) {
		t.Error("Dockerfile content does not match embedded content")
	}

	// entrypoint.sh should exist with correct content and be executable
	epContent, err := os.ReadFile(filepath.Join(dir, "entrypoint.sh"))
	if err != nil {
		t.Fatalf("reading entrypoint.sh: %v", err)
	}
	if string(epContent) != string(entrypointSh) {
		t.Error("entrypoint.sh content does not match embedded content")
	}

	epInfo, err := os.Stat(filepath.Join(dir, "entrypoint.sh"))
	if err != nil {
		t.Fatalf("stat entrypoint.sh: %v", err)
	}
	if epInfo.Mode().Perm()&0111 == 0 {
		t.Error("entrypoint.sh should be executable")
	}
}

func TestPrepareBuildContext_CustomDockerfile(t *testing.T) {
	customDir := t.TempDir()
	customContent := []byte("FROM alpine:latest\nCOPY entrypoint.sh /entrypoint.sh\nRUN echo custom\n")
	customPath := filepath.Join(customDir, "Dockerfile.custom")
	if err := os.WriteFile(customPath, customContent, 0644); err != nil {
		t.Fatal(err)
	}
	// Place additional files alongside the Dockerfile
	if err := os.WriteFile(filepath.Join(customDir, "entrypoint.sh"), []byte("#!/bin/bash\necho hello"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(customDir, "scripts"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(customDir, "scripts", "setup.sh"), []byte("#!/bin/bash\necho setup"), 0755); err != nil {
		t.Fatal(err)
	}

	dir, cleanup, err := PrepareBuildContext(customPath)
	if err != nil {
		t.Fatalf("PrepareBuildContext() error: %v", err)
	}
	defer cleanup()

	// Build context should be the directory containing the Dockerfile
	if dir != customDir {
		t.Errorf("build context dir = %q, want %q", dir, customDir)
	}

	// All files in the directory should be accessible
	if _, err := os.Stat(filepath.Join(dir, "entrypoint.sh")); err != nil {
		t.Errorf("entrypoint.sh should be accessible in build context: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "scripts", "setup.sh")); err != nil {
		t.Errorf("scripts/setup.sh should be accessible in build context: %v", err)
	}
}

func TestPrepareBuildContext_CustomDockerfileCleanupIsNoop(t *testing.T) {
	customDir := t.TempDir()
	customPath := filepath.Join(customDir, "Dockerfile")
	if err := os.WriteFile(customPath, []byte("FROM alpine:latest\n"), 0644); err != nil {
		t.Fatal(err)
	}

	dir, cleanup, err := PrepareBuildContext(customPath)
	if err != nil {
		t.Fatalf("PrepareBuildContext() error: %v", err)
	}

	cleanup()

	// Directory should still exist after cleanup (it's the user's directory)
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("user's directory should not be deleted by cleanup: %v", err)
	}
}

func TestPrepareBuildContext_CustomDockerfileNotFound(t *testing.T) {
	_, _, err := PrepareBuildContext("/nonexistent/Dockerfile")
	if err == nil {
		t.Fatal("expected error for nonexistent custom Dockerfile")
	}
	if !strings.Contains(err.Error(), "reading custom Dockerfile") {
		t.Errorf("error = %q, want containing 'reading custom Dockerfile'", err.Error())
	}
}

func TestPrepareBuildContextCleanup(t *testing.T) {
	dir, cleanup, err := PrepareBuildContext("")
	if err != nil {
		t.Fatalf("PrepareBuildContext() error: %v", err)
	}

	// Directory should exist before cleanup
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("dir should exist before cleanup: %v", err)
	}

	cleanup()

	// Directory should not exist after cleanup
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Error("dir should not exist after cleanup")
	}
}
