package image

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/konono/aw/internal/containerenv"
	"github.com/konono/aw/internal/profile"
)

func TestEmbeddedTemplatesNotEmpty(t *testing.T) {
	if len(dockerfileDebian12Tmpl) == 0 {
		t.Error("embedded Dockerfile.debian12.tmpl is empty")
	}
	if len(dockerfileUBI9Tmpl) == 0 {
		t.Error("embedded Dockerfile.ubi9.tmpl is empty")
	}
	if len(dockerfileUBI10Tmpl) == 0 {
		t.Error("embedded Dockerfile.ubi10.tmpl is empty")
	}
	if len(dockerfileUbuntu2604Tmpl) == 0 {
		t.Error("embedded Dockerfile.ubuntu2604.tmpl is empty")
	}
	if len(entrypointShTmpl) == 0 {
		t.Error("embedded entrypoint.sh.tmpl is empty")
	}
}

func TestRenderDockerfile(t *testing.T) {
	cenv := containerenv.Default()

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
			df, err := RenderDockerfile(tt.os, profile.PackageManagerApt, cenv)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error for unknown OS template")
				}
				return
			}
			if err != nil {
				t.Fatalf("RenderDockerfile() error: %v", err)
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
				t.Error("Dockerfile should create user")
			}
			if !strings.Contains(string(df), `BASH_ENV="/home/agent/.aw_env.sh"`) {
				t.Error("Dockerfile should set BASH_ENV to /home/agent/.aw_env.sh")
			}
			if !strings.Contains(string(df), `HOME="/home/agent"`) {
				t.Error("Dockerfile should set HOME to /home/agent")
			}
		})
	}
}

func TestRenderDockerfile_CustomUser(t *testing.T) {
	cenv := containerenv.FromUser("dev")
	df, err := RenderDockerfile(profile.OSDebian12, profile.PackageManagerApt, cenv)
	if err != nil {
		t.Fatalf("RenderDockerfile() error: %v", err)
	}
	content := string(df)
	if !strings.Contains(content, "useradd -m -s /bin/bash -u 1001 -g 0 dev") {
		t.Error("Dockerfile should create 'dev' user with fixed UID 1001 and GID 0")
	}
	if !strings.Contains(content, `HOME="/home/dev"`) {
		t.Error("Dockerfile should set HOME to /home/dev")
	}
	if strings.Contains(content, "/home/agent") {
		t.Error("Dockerfile should not contain /home/agent when using custom user")
	}
}

func TestRenderEntrypoint(t *testing.T) {
	cenv := containerenv.Default()
	ep, err := RenderEntrypoint(profile.PackageManagerApt, cenv)
	if err != nil {
		t.Fatalf("RenderEntrypoint() error: %v", err)
	}
	content := string(ep)
	if !strings.HasPrefix(content, "#!/bin/bash") {
		t.Error("entrypoint.sh should start with shebang")
	}
	if !strings.Contains(content, "/home/agent/.aw_env.sh") {
		t.Error("entrypoint.sh should reference /home/agent/.aw_env.sh")
	}
	if !strings.Contains(content, "AW_BASH_ENV_LOADED") {
		t.Error("entrypoint.sh should guard against reloading .aw_env.sh")
	}
}

func TestRenderEntrypoint_CustomUser(t *testing.T) {
	cenv := containerenv.FromUser("dev")
	ep, err := RenderEntrypoint(profile.PackageManagerApt, cenv)
	if err != nil {
		t.Fatalf("RenderEntrypoint() error: %v", err)
	}
	content := string(ep)
	if !strings.Contains(content, "/home/dev/.aw_env.sh") {
		t.Error("entrypoint.sh should reference /home/dev/.aw_env.sh")
	}
	if strings.Contains(content, "/home/agent") {
		t.Error("entrypoint.sh should not contain /home/agent when using custom user")
	}
}

func TestDefaultDockerfile(t *testing.T) {
	content := DefaultDockerfile()
	if len(content) == 0 {
		t.Error("DefaultDockerfile() returned empty content")
	}
	expected, _ := RenderDockerfile(profile.OSDebian12, profile.PackageManagerApt, containerenv.Default())
	if string(content) != string(expected) {
		t.Error("DefaultDockerfile() content does not match rendered debian12 dockerfile")
	}
}

func TestPrepareBuildContext(t *testing.T) {
	cenv := containerenv.Default()
	dir, cleanup, err := PrepareBuildContext("", profile.OSDebian12, profile.PackageManagerApt, cenv)
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

	expectedDF, _ := RenderDockerfile(profile.OSDebian12, profile.PackageManagerApt, cenv)
	dfContent, err := os.ReadFile(filepath.Join(dir, "Dockerfile"))
	if err != nil {
		t.Fatalf("reading Dockerfile: %v", err)
	}
	if string(dfContent) != string(expectedDF) {
		t.Error("Dockerfile content does not match rendered debian12 content")
	}

	expectedEP, _ := RenderEntrypoint(profile.PackageManagerApt, cenv)
	epContent, err := os.ReadFile(filepath.Join(dir, "entrypoint.sh"))
	if err != nil {
		t.Fatalf("reading entrypoint.sh: %v", err)
	}
	if string(epContent) != string(expectedEP) {
		t.Error("entrypoint.sh content does not match rendered content")
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
	cenv := containerenv.Default()
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
			dir, cleanup, err := PrepareBuildContext("", tt.os, profile.PackageManagerApt, cenv)
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

	dir, cleanup, err := PrepareBuildContext(customPath, profile.OSDebian12, profile.PackageManagerApt, containerenv.Default())
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

	dir, cleanup, err := PrepareBuildContext(customPath, profile.OSDebian12, profile.PackageManagerApt, containerenv.Default())
	if err != nil {
		t.Fatalf("PrepareBuildContext() error: %v", err)
	}

	cleanup()

	if _, err := os.Stat(dir); err != nil {
		t.Errorf("user's directory should not be deleted by cleanup: %v", err)
	}
}

func TestPrepareBuildContext_CustomDockerfileNotFound(t *testing.T) {
	_, _, err := PrepareBuildContext("/nonexistent/Dockerfile", profile.OSDebian12, profile.PackageManagerApt, containerenv.Default())
	if err == nil {
		t.Fatal("expected error for nonexistent custom Dockerfile")
	}
	if !strings.Contains(err.Error(), "reading custom Dockerfile") {
		t.Errorf("error = %q, want containing 'reading custom Dockerfile'", err.Error())
	}
}

func TestPrepareBuildContextCleanup(t *testing.T) {
	dir, cleanup, err := PrepareBuildContext("", profile.OSDebian12, profile.PackageManagerApt, containerenv.Default())
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

func TestRenderDockerfile_GID0Pattern(t *testing.T) {
	cenv := containerenv.Default()
	for _, osTemplate := range []profile.OSTemplate{
		profile.OSDebian12,
		profile.OSUBI9,
		profile.OSUBI10,
		profile.OSUbuntu2604,
	} {
		t.Run(string(osTemplate), func(t *testing.T) {
			df, err := RenderDockerfile(osTemplate, profile.PackageManagerApt, cenv)
			if err != nil {
				t.Fatalf("RenderDockerfile() error: %v", err)
			}
			content := string(df)

			if !strings.Contains(content, "chmod -R g=u") {
				t.Error("Dockerfile should contain chmod -R g=u for GID 0 pattern")
			}
			if !strings.Contains(content, "chmod g=u /etc/passwd") {
				t.Error("Dockerfile should make /etc/passwd group-writable")
			}
			if strings.Contains(content, "HOST_UID") || strings.Contains(content, "HOST_GID") {
				t.Error("Dockerfile should not contain HOST_UID/HOST_GID build args")
			}
			if !strings.Contains(content, "useradd -m -s /bin/bash -u 1001 -g 0") {
				t.Error("Dockerfile should create user with fixed UID 1001 and GID 0")
			}
		})
	}
}

func TestRenderEntrypoint_DynamicPasswd(t *testing.T) {
	cenv := containerenv.Default()
	ep, err := RenderEntrypoint(profile.PackageManagerApt, cenv)
	if err != nil {
		t.Fatalf("RenderEntrypoint() error: %v", err)
	}
	content := string(ep)
	if !strings.Contains(content, "grep -v") || !strings.Contains(content, "/etc/passwd") {
		t.Error("entrypoint.sh should remove stale UID entries from /etc/passwd")
	}
	if !strings.Contains(content, ">> /etc/passwd") {
		t.Error("entrypoint.sh should dynamically add UID to /etc/passwd")
	}
	if strings.Contains(content, "sudo find") {
		t.Error("entrypoint.sh should not use chown-based ownership fix")
	}
}

func TestRenderDockerfile_ToolInstallScript(t *testing.T) {
	cenv := containerenv.Default()
	for _, osTemplate := range []profile.OSTemplate{
		profile.OSDebian12,
		profile.OSUBI9,
		profile.OSUBI10,
		profile.OSUbuntu2604,
	} {
		t.Run(string(osTemplate), func(t *testing.T) {
			df, err := RenderDockerfile(osTemplate, profile.PackageManagerApt, cenv)
			if err != nil {
				t.Fatalf("RenderDockerfile() error: %v", err)
			}
			content := string(df)

			if !strings.Contains(content, "AW_TOOL_INSTALL_SCRIPT") {
				t.Error("Dockerfile should contain AW_TOOL_INSTALL_SCRIPT for tool installation")
			}
		})
	}
}
