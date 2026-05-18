package image

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/konono/aw/internal/profile"
)

func TestEmbeddedFilesNotEmpty(t *testing.T) {
	if len(dockerfileDebian12) == 0 {
		t.Error("embedded Dockerfile.debian12 is empty")
	}
	if len(dockerfileUBI9) == 0 {
		t.Error("embedded Dockerfile.ubi9 is empty")
	}
	if len(dockerfileUBI10) == 0 {
		t.Error("embedded Dockerfile.ubi10 is empty")
	}
	if len(dockerfileUbuntu2604) == 0 {
		t.Error("embedded Dockerfile.ubuntu2604 is empty")
	}
	if len(entrypointSh) == 0 {
		t.Error("embedded entrypoint.sh is empty")
	}
}

func TestDockerfileForOS(t *testing.T) {
	tests := []struct {
		name     string
		os       profile.OSTemplate
		contains string
		wantErr  bool
	}{
		{"debian12", profile.OSDebian12, "FROM debian:bookworm-slim", false},
		{"ubi9", profile.OSUBI9, "FROM registry.access.redhat.com/ubi9", false},
		{"ubi10", profile.OSUBI10, "FROM registry.access.redhat.com/ubi10", false},
		{"ubuntu2604", profile.OSUbuntu2604, "FROM ubuntu:26.04", false},
		{"unknown", profile.OSTemplate("centos7"), "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			df, err := DockerfileForOS(tt.os)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error for unknown OS template")
				}
				return
			}
			if err != nil {
				t.Fatalf("DockerfileForOS() error: %v", err)
			}
			if len(df) == 0 {
				t.Error("returned Dockerfile is empty")
			}
			if !strings.Contains(string(df), tt.contains) {
				t.Errorf("Dockerfile should contain %q", tt.contains)
			}
			if !strings.Contains(string(df), "ENTRYPOINT") {
				t.Error("Dockerfile should contain ENTRYPOINT")
			}
			if !strings.Contains(string(df), "useradd") {
				t.Error("Dockerfile should create agent user")
			}
		})
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
	if string(content) != string(dockerfileDebian12) {
		t.Error("DefaultDockerfile() content does not match embedded debian12 dockerfile")
	}
}

func TestPrepareBuildContext(t *testing.T) {
	dir, cleanup, err := PrepareBuildContext("", profile.OSDebian12)
	if err != nil {
		t.Fatalf("PrepareBuildContext() error: %v", err)
	}
	defer cleanup()

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("build context dir does not exist: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("build context path is not a directory")
	}

	dfContent, err := os.ReadFile(filepath.Join(dir, "Dockerfile"))
	if err != nil {
		t.Fatalf("reading Dockerfile: %v", err)
	}
	if string(dfContent) != string(dockerfileDebian12) {
		t.Error("Dockerfile content does not match embedded debian12 content")
	}

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

func TestPrepareBuildContext_WithOS(t *testing.T) {
	tests := []struct {
		name     string
		os       profile.OSTemplate
		contains string
	}{
		{"debian12", profile.OSDebian12, "FROM debian:bookworm-slim"},
		{"ubi9", profile.OSUBI9, "FROM registry.access.redhat.com/ubi9"},
		{"ubi10", profile.OSUBI10, "FROM registry.access.redhat.com/ubi10"},
		{"ubuntu2604", profile.OSUbuntu2604, "FROM ubuntu:26.04"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir, cleanup, err := PrepareBuildContext("", tt.os)
			if err != nil {
				t.Fatalf("PrepareBuildContext() error: %v", err)
			}
			defer cleanup()

			dfContent, err := os.ReadFile(filepath.Join(dir, "Dockerfile"))
			if err != nil {
				t.Fatalf("reading Dockerfile: %v", err)
			}
			if !strings.Contains(string(dfContent), tt.contains) {
				t.Errorf("Dockerfile should contain %q", tt.contains)
			}

			if _, err := os.Stat(filepath.Join(dir, "entrypoint.sh")); err != nil {
				t.Errorf("entrypoint.sh should exist: %v", err)
			}
		})
	}
}

func TestPrepareBuildContext_CustomDockerfile(t *testing.T) {
	customDir := t.TempDir()
	customContent := []byte("FROM alpine:latest\nCOPY entrypoint.sh /entrypoint.sh\nRUN echo custom\n")
	customPath := filepath.Join(customDir, "Dockerfile.custom")
	if err := os.WriteFile(customPath, customContent, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(customDir, "entrypoint.sh"), []byte("#!/bin/bash\necho hello"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(customDir, "scripts"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(customDir, "scripts", "setup.sh"), []byte("#!/bin/bash\necho setup"), 0755); err != nil {
		t.Fatal(err)
	}

	dir, cleanup, err := PrepareBuildContext(customPath, profile.OSDebian12)
	if err != nil {
		t.Fatalf("PrepareBuildContext() error: %v", err)
	}
	defer cleanup()

	if dir != customDir {
		t.Errorf("build context dir = %q, want %q", dir, customDir)
	}

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

	dir, cleanup, err := PrepareBuildContext(customPath, profile.OSDebian12)
	if err != nil {
		t.Fatalf("PrepareBuildContext() error: %v", err)
	}

	cleanup()

	if _, err := os.Stat(dir); err != nil {
		t.Errorf("user's directory should not be deleted by cleanup: %v", err)
	}
}

func TestPrepareBuildContext_CustomDockerfileNotFound(t *testing.T) {
	_, _, err := PrepareBuildContext("/nonexistent/Dockerfile", profile.OSDebian12)
	if err == nil {
		t.Fatal("expected error for nonexistent custom Dockerfile")
	}
	if !strings.Contains(err.Error(), "reading custom Dockerfile") {
		t.Errorf("error = %q, want containing 'reading custom Dockerfile'", err.Error())
	}
}

func TestPrepareBuildContextCleanup(t *testing.T) {
	dir, cleanup, err := PrepareBuildContext("", profile.OSDebian12)
	if err != nil {
		t.Fatalf("PrepareBuildContext() error: %v", err)
	}

	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("dir should exist before cleanup: %v", err)
	}

	cleanup()

	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Error("dir should not exist after cleanup")
	}
}
