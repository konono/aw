package image

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/konono/aw/v4/internal/containerenv"
	"github.com/konono/aw/v4/internal/profile"
)

func TestAllOSTemplates_RenderValidDockerfiles(t *testing.T) {
	cenv := containerenv.Default()
	for _, os := range []profile.OSTemplate{
		profile.OSDebian12, profile.OSUBI9, profile.OSUBI10, profile.OSUbuntu2604,
	} {
		t.Run(string(os), func(t *testing.T) {
			df, err := RenderDockerfile(os, profile.PackageManagerApt, cenv)
			if err != nil {
				t.Fatalf("RenderDockerfile: %v", err)
			}
			content := string(df)
			for _, want := range []string{"FROM", "ENTRYPOINT", "useradd"} {
				if !strings.Contains(content, want) {
					t.Errorf("rendered Dockerfile missing %q", want)
				}
			}
		})
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

func TestEntrypoint(t *testing.T) {
	ep := Entrypoint(profile.PackageManagerApt)
	content := string(ep)
	if !strings.HasPrefix(content, "#!/bin/bash") {
		t.Error("entrypoint.sh should start with shebang")
	}
	if !strings.Contains(content, ". /aw-init.sh") {
		t.Error("entrypoint.sh should source /aw-init.sh")
	}
	if strings.Contains(content, "{{") {
		t.Error("entrypoint.sh should not contain Go template variables")
	}
	if !strings.Contains(content, "aw_exec") {
		t.Error("entrypoint.sh should call aw_exec")
	}
}

func TestEntrypoint_Devbox(t *testing.T) {
	ep := Entrypoint(profile.PackageManagerDevbox)
	content := string(ep)
	if !strings.HasPrefix(content, "#!/bin/bash") {
		t.Error("devbox entrypoint should start with shebang")
	}
	if !strings.Contains(content, ". /aw-init.sh") {
		t.Error("devbox entrypoint should source /aw-init.sh")
	}
	if strings.Contains(content, "{{") {
		t.Error("devbox entrypoint should not contain Go template variables")
	}
	if !strings.Contains(content, "devbox") {
		t.Error("devbox entrypoint should handle devbox packages")
	}
	if !strings.Contains(content, "/nix/var") {
		t.Error("devbox entrypoint should fix /nix/var ownership")
	}
	if !strings.Contains(content, "aw_exec") {
		t.Error("devbox entrypoint should call aw_exec")
	}
}

func TestRenderDockerfile_ContainsExtraPackagesArg(t *testing.T) {
	cenv := containerenv.Default()
	for _, tmplOS := range []profile.OSTemplate{
		profile.OSDebian12, profile.OSUBI9, profile.OSUBI10, profile.OSUbuntu2604,
	} {
		for _, pkgMgr := range []profile.PackageManager{
			profile.PackageManagerApt, profile.PackageManagerDevbox,
		} {
			name := string(tmplOS) + "_" + string(pkgMgr)
			t.Run(name, func(t *testing.T) {
				df, err := RenderDockerfile(tmplOS, pkgMgr, cenv)
				if err != nil {
					t.Fatalf("RenderDockerfile() error: %v", err)
				}
				content := string(df)
				if !strings.Contains(content, `ARG AW_EXTRA_PACKAGES=""`) {
					t.Error("Dockerfile should contain ARG AW_EXTRA_PACKAGES")
				}
				if !strings.Contains(content, "AW_EXTRA_PACKAGES") {
					t.Error("Dockerfile should reference AW_EXTRA_PACKAGES in a RUN command")
				}
			})
		}
	}
}

func TestInitScript_ContainsAWPackages(t *testing.T) {
	content := string(InitScript())
	for _, want := range []string{
		"AW_PACKAGES",
		"apt-get",
		"dnf",
		"dpkg -s",
		"rpm -q",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("aw-init.sh should contain %q", want)
		}
	}
}

func TestEntrypoint_DiffersPerPackageManager(t *testing.T) {
	apt := Entrypoint(profile.PackageManagerApt)
	devbox := Entrypoint(profile.PackageManagerDevbox)
	if string(apt) == string(devbox) {
		t.Error("apt and devbox entrypoints should differ")
	}
}

func TestInitScript(t *testing.T) {
	content := string(InitScript())
	if len(content) == 0 {
		t.Fatal("InitScript() returned empty content")
	}
	if !strings.HasPrefix(content, "#!/bin/bash") {
		t.Error("aw-init.sh should start with shebang")
	}
	for _, want := range []string{
		"AW_USER",
		"AW_HOME",
		"AW_WORKSPACE",
		"aw_exec",
		"/etc/passwd",
		"AW_BASH_ENV_LOADED",
		".ssh-host",
		"GITHUB_TOKEN",
		"AW_CONTAINER_CONFIG_DIR",
		"AW_DATA_SYMLINKS",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("aw-init.sh should contain %q", want)
		}
	}
	if strings.Contains(content, "{{") {
		t.Error("aw-init.sh should not contain Go template variables")
	}
}

func TestInitScript_DynamicPasswd(t *testing.T) {
	content := string(InitScript())
	if !strings.Contains(content, "grep -v") || !strings.Contains(content, "/etc/passwd") {
		t.Error("aw-init.sh should remove stale UID entries from /etc/passwd")
	}
	if !strings.Contains(content, ">> /etc/passwd") {
		t.Error("aw-init.sh should dynamically add UID to /etc/passwd")
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

	expectedEP := Entrypoint(profile.PackageManagerApt)
	epContent, err := os.ReadFile(filepath.Join(dir, "entrypoint.sh"))
	if err != nil {
		t.Fatalf("reading entrypoint.sh: %v", err)
	}
	if string(epContent) != string(expectedEP) {
		t.Error("entrypoint.sh content does not match embedded content")
	}

	epInfo, err := os.Stat(filepath.Join(dir, "entrypoint.sh"))
	if err != nil {
		t.Fatalf("stat entrypoint.sh: %v", err)
	}
	if runtime.GOOS != "windows" && epInfo.Mode().Perm()&0111 == 0 {
		t.Error("entrypoint.sh should be executable")
	}

	initContent, err := os.ReadFile(filepath.Join(dir, "aw-init.sh"))
	if err != nil {
		t.Fatalf("reading aw-init.sh: %v", err)
	}
	if string(initContent) != string(InitScript()) {
		t.Error("aw-init.sh content does not match embedded content")
	}
	initInfo, err := os.Stat(filepath.Join(dir, "aw-init.sh"))
	if err != nil {
		t.Fatalf("stat aw-init.sh: %v", err)
	}
	if runtime.GOOS != "windows" && initInfo.Mode().Perm()&0111 == 0 {
		t.Error("aw-init.sh should be executable")
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

