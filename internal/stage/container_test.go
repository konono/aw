package stage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/konono/aw/internal/config"
	"github.com/konono/aw/internal/docker"
	"github.com/konono/aw/internal/mount"
	"github.com/konono/aw/internal/pipeline"
	"github.com/konono/aw/internal/profile"
)

type mockDockerClient struct {
	available           bool
	buildCalled         bool
	buildImageName      string
	buildContextDir     string
	buildArgs           map[string]string
	buildContextFiles   map[string][]byte
	volumeCalled        bool
	runCalled           bool
	runConfig           docker.RunConfig
	imageExists         bool
	imageExistsCalled   bool
	imageExistsErr      error
	saveCalled          bool
	saveImageName       string
	saveOutputPath      string
}

func (m *mockDockerClient) CheckAvailable() error {
	if !m.available {
		return fmt.Errorf("docker not available")
	}
	return nil
}

func (m *mockDockerClient) Build(_ context.Context, imageName, contextDir, _ string, buildArgs map[string]string) error {
	m.buildCalled = true
	m.buildImageName = imageName
	m.buildContextDir = contextDir
	m.buildArgs = buildArgs
	m.buildContextFiles = make(map[string][]byte)
	entries, _ := os.ReadDir(contextDir)
	for _, e := range entries {
		if !e.IsDir() {
			data, _ := os.ReadFile(filepath.Join(contextDir, e.Name()))
			m.buildContextFiles[e.Name()] = data
		}
	}
	return nil
}

func (m *mockDockerClient) ImageExists(_ context.Context, _ string) (bool, error) {
	m.imageExistsCalled = true
	return m.imageExists, m.imageExistsErr
}

func (m *mockDockerClient) Save(_ context.Context, imageName, outputPath string) error {
	m.saveCalled = true
	m.saveImageName = imageName
	m.saveOutputPath = outputPath
	return nil
}

func (m *mockDockerClient) VolumeCreate(_ context.Context, _ string) error {
	m.volumeCalled = true
	return nil
}

func (m *mockDockerClient) Run(_ context.Context, config docker.RunConfig) error {
	m.runCalled = true
	m.runConfig = config
	return nil
}

func (m *mockDockerClient) RunOneShot(_ context.Context, config docker.RunConfig) (string, error) {
	return "aw-snapshot-mock", nil
}

func (m *mockDockerClient) Commit(_ context.Context, _, _ string, _ []string) error {
	return nil
}

func (m *mockDockerClient) RemoveContainer(_ context.Context, _ string) error {
	return nil
}

type mockConfigSyncer struct {
	syncCalled    bool
	onboardCalled bool
	syncErr       error
	onboardErr    error
}

func (m *mockConfigSyncer) SyncToolSettings(_, _ string, _ config.ToolSyncSpec) error {
	m.syncCalled = true
	return m.syncErr
}

func (m *mockConfigSyncer) EnsureOnboardingState(_ string) error {
	m.onboardCalled = true
	return m.onboardErr
}

type mockMountBuilder struct {
	mounts []docker.Mount
	err    error
}

func (m *mockMountBuilder) BuildMounts(_ mount.MountOptions) ([]docker.Mount, error) {
	return m.mounts, m.err
}

func TestDockerStage_DockerNotAvailable(t *testing.T) {
	s := &DockerStage{
		DockerClient: &mockDockerClient{available: false},
		ConfigSyncer: &mockConfigSyncer{},
		MountBuilder: &mockMountBuilder{},
	}

	ec := &pipeline.ExecutionContext{
		Profile: profile.Profile{Environment: profile.EnvironmentContainer},
		HomeDir: "/home/test",
		WorkDir: "/workspace",
	}

	err := s.Run(context.Background(), ec)
	if err == nil {
		t.Fatal("expected error when docker not available")
	}
	if !strings.Contains(err.Error(), "container runtime is not available") {
		t.Errorf("error = %q, want containing 'container runtime is not available'", err.Error())
	}
}

func TestAppendContainerContext_EmptyStageDir(t *testing.T) {
	ec := &pipeline.ExecutionContext{}
	err := appendContainerContext("", ec)
	if err != nil {
		t.Fatalf("appendContainerContext() error: %v", err)
	}
}

func TestAppendContainerContext_BaseOnly(t *testing.T) {
	tmpDir := t.TempDir()
	ec := &pipeline.ExecutionContext{
		Profile: profile.Profile{},
	}

	if err := appendContainerContext(tmpDir, ec); err != nil {
		t.Fatalf("appendContainerContext() error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmpDir, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("reading CLAUDE.md: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "# aw Container Environment") {
		t.Error("missing header")
	}
	if !strings.Contains(content, "## Package Managers") {
		t.Error("missing Package Managers section")
	}
	if !strings.Contains(content, "mise") {
		t.Error("apt mode should mention mise")
	}
	if strings.Contains(content, "npm") {
		t.Error("apt mode should not mention npm")
	}
	if strings.Contains(content, "## Docker / Podman") {
		t.Error("Docker section should not be present")
	}
	if strings.Contains(content, "## GitHub CLI") {
		t.Error("GitHub CLI section should not be present")
	}
	if strings.Contains(content, "## SSH Agent") {
		t.Error("SSH Agent section should not be present")
	}
}

func TestAppendContainerContext_DevboxMode(t *testing.T) {
	tmpDir := t.TempDir()
	ec := &pipeline.ExecutionContext{
		Profile: profile.Profile{
			PackageManager: profile.PackageManagerDevbox,
		},
	}

	if err := appendContainerContext(tmpDir, ec); err != nil {
		t.Fatalf("appendContainerContext() error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmpDir, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("reading CLAUDE.md: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "npm") {
		t.Error("devbox mode should mention npm")
	}
	if !strings.Contains(content, "mise") {
		t.Error("devbox mode should mention mise")
	}
}

func TestAppendContainerContext_AllFeatures(t *testing.T) {
	tmpDir := t.TempDir()
	ghToken := true
	ec := &pipeline.ExecutionContext{
		Profile: profile.Profile{
			GhToken: &ghToken,
		},
		SSHAgentReady:      true,
		ContainerSockReady: true,
	}

	if err := appendContainerContext(tmpDir, ec); err != nil {
		t.Fatalf("appendContainerContext() error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmpDir, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("reading CLAUDE.md: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "## Package Managers") {
		t.Error("missing Package Managers section")
	}
	if !strings.Contains(content, "## Docker / Podman (DooD)") {
		t.Error("missing Docker section")
	}
	if !strings.Contains(content, "## GitHub CLI") {
		t.Error("missing GitHub CLI section")
	}
	if !strings.Contains(content, "GITHUB_TOKEN is set") {
		t.Error("GitHub CLI section should mention GITHUB_TOKEN")
	}
	if !strings.Contains(content, "## SSH Agent") {
		t.Error("missing SSH Agent section")
	}
}

func TestAppendContainerContext_MountGHLegacy(t *testing.T) {
	tmpDir := t.TempDir()
	mountGH := true
	ec := &pipeline.ExecutionContext{
		Profile: profile.Profile{
			MountGH: &mountGH,
		},
	}

	if err := appendContainerContext(tmpDir, ec); err != nil {
		t.Fatalf("appendContainerContext() error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmpDir, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("reading CLAUDE.md: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "## GitHub CLI") {
		t.Error("missing GitHub CLI section")
	}
	if !strings.Contains(content, "mounted (read-only)") {
		t.Error("legacy mount_gh should mention mounted")
	}
}

func TestAppendContainerContext_PreservesExistingContent(t *testing.T) {
	tmpDir := t.TempDir()
	existing := "# Existing CLAUDE.md content\n\nSome instructions here.\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "CLAUDE.md"), []byte(existing), 0644); err != nil {
		t.Fatalf("writing existing CLAUDE.md: %v", err)
	}

	ec := &pipeline.ExecutionContext{
		Profile:            profile.Profile{},
		ContainerSockReady: true,
	}

	if err := appendContainerContext(tmpDir, ec); err != nil {
		t.Fatalf("appendContainerContext() error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmpDir, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("reading CLAUDE.md: %v", err)
	}
	content := string(data)

	if !strings.HasPrefix(content, existing) {
		t.Error("existing content should be preserved at the beginning")
	}
	if !strings.Contains(content, "# aw Container Environment") {
		t.Error("container context should be appended")
	}
	if !strings.Contains(content, "## Docker / Podman (DooD)") {
		t.Error("Docker section should be present")
	}
}

func TestAppendContainerContext_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	existing := "# Existing CLAUDE.md content\n\nSome instructions here.\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "CLAUDE.md"), []byte(existing), 0644); err != nil {
		t.Fatalf("writing existing CLAUDE.md: %v", err)
	}

	ec := &pipeline.ExecutionContext{
		Profile:            profile.Profile{},
		ContainerSockReady: true,
	}

	for i := 0; i < 5; i++ {
		if err := appendContainerContext(tmpDir, ec); err != nil {
			t.Fatalf("appendContainerContext() iteration %d error: %v", i, err)
		}
	}

	data, err := os.ReadFile(filepath.Join(tmpDir, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("reading CLAUDE.md: %v", err)
	}
	content := string(data)

	count := strings.Count(content, "# aw Container Environment")
	if count != 1 {
		t.Errorf("expected exactly 1 container context block, got %d", count)
	}
	if !strings.HasPrefix(content, existing) {
		t.Error("existing content should be preserved at the beginning")
	}
}

func TestDockerStage_PrebuiltImage_SkipsBuild(t *testing.T) {
	dc := &mockDockerClient{available: true, imageExists: true}
	s := &DockerStage{
		DockerClient: dc,
		ConfigSyncer: &mockConfigSyncer{},
		MountBuilder: &mockMountBuilder{},
	}

	ec := &pipeline.ExecutionContext{
		Profile: profile.Profile{
			Environment: profile.EnvironmentContainer,
			Launch:      profile.LaunchShell,
			Image:       "my-image:latest",
		},
		HomeDir: t.TempDir(),
		WorkDir: t.TempDir(),
	}

	err := s.Run(context.Background(), ec)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if dc.buildCalled {
		t.Error("Build should not be called when image is set")
	}
	if !dc.imageExistsCalled {
		t.Error("ImageExists should be called when image is set")
	}
	if ec.DockerImage != "my-image:latest" {
		t.Errorf("DockerImage = %q, want %q", ec.DockerImage, "my-image:latest")
	}
}

func TestDockerStage_PrebuiltImage_NotFound(t *testing.T) {
	dc := &mockDockerClient{available: true, imageExists: false}
	s := &DockerStage{
		DockerClient: dc,
		ConfigSyncer: &mockConfigSyncer{},
		MountBuilder: &mockMountBuilder{},
	}

	ec := &pipeline.ExecutionContext{
		Profile: profile.Profile{
			Environment: profile.EnvironmentContainer,
			Launch:      profile.LaunchShell,
			Image:       "nonexistent:v1",
		},
		HomeDir: t.TempDir(),
		WorkDir: t.TempDir(),
	}

	err := s.Run(context.Background(), ec)
	if err == nil {
		t.Fatal("Run() should return error when image not found")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want containing 'not found'", err.Error())
	}
	if !strings.Contains(err.Error(), "docker load") {
		t.Errorf("error = %q, want containing 'docker load'", err.Error())
	}
}

func TestDockerStage_PrebuiltImage_ImageInspectError(t *testing.T) {
	dc := &mockDockerClient{
		available:      true,
		imageExistsErr: fmt.Errorf("docker image inspect \"bad::ref\" failed: invalid reference format"),
	}
	s := &DockerStage{
		DockerClient: dc,
		ConfigSyncer: &mockConfigSyncer{},
		MountBuilder: &mockMountBuilder{},
	}

	ec := &pipeline.ExecutionContext{
		Profile: profile.Profile{
			Environment: profile.EnvironmentContainer,
			Launch:      profile.LaunchShell,
			Image:       "bad::ref",
		},
		HomeDir: t.TempDir(),
		WorkDir: t.TempDir(),
	}

	err := s.Run(context.Background(), ec)
	if err == nil {
		t.Fatal("Run() should return error when image inspect fails")
	}
	if !strings.Contains(err.Error(), "invalid reference format") {
		t.Errorf("error = %q, want containing 'invalid reference format'", err.Error())
	}
	if strings.Contains(err.Error(), "docker load") {
		t.Errorf("error = %q, should not suggest docker load for inspect errors", err.Error())
	}
}

func TestDockerStage_SPCTWhenContainerSockReady(t *testing.T) {
	dc := &mockDockerClient{available: true, imageExists: true}
	s := &DockerStage{
		DockerClient: dc,
		ConfigSyncer: &mockConfigSyncer{},
		MountBuilder: &mockMountBuilder{},
	}

	ec := &pipeline.ExecutionContext{
		Profile: profile.Profile{
			Environment: profile.EnvironmentContainer,
			Launch:      profile.LaunchShell,
			Image:       "test-image:latest",
		},
		HomeDir:            t.TempDir(),
		WorkDir:            t.TempDir(),
		ContainerSockReady: true,
	}

	if err := s.Run(context.Background(), ec); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	found := false
	for _, opt := range ec.DockerSecurityOpts {
		if opt == "label=type:spc_t" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("DockerSecurityOpts should contain spc_t when ContainerSockReady, got %v", ec.DockerSecurityOpts)
	}
}

func TestDockerStage_SPCTWhenWorkDirEqualsHomeDir(t *testing.T) {
	homeDir := t.TempDir()
	dc := &mockDockerClient{available: true, imageExists: true}
	s := &DockerStage{
		DockerClient: dc,
		ConfigSyncer: &mockConfigSyncer{},
		MountBuilder: &mockMountBuilder{},
	}

	ec := &pipeline.ExecutionContext{
		Profile: profile.Profile{
			Environment: profile.EnvironmentContainer,
			Launch:      profile.LaunchShell,
			Image:       "test-image:latest",
		},
		HomeDir: homeDir,
		WorkDir: homeDir,
	}

	if err := s.Run(context.Background(), ec); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	found := false
	for _, opt := range ec.DockerSecurityOpts {
		if opt == "label=type:spc_t" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("DockerSecurityOpts should contain spc_t when WorkDir == HomeDir, got %v", ec.DockerSecurityOpts)
	}
}

func TestDockerStage_NoSPCTWhenNotNeeded(t *testing.T) {
	dc := &mockDockerClient{available: true, imageExists: true}
	s := &DockerStage{
		DockerClient: dc,
		ConfigSyncer: &mockConfigSyncer{},
		MountBuilder: &mockMountBuilder{},
	}

	ec := &pipeline.ExecutionContext{
		Profile: profile.Profile{
			Environment: profile.EnvironmentContainer,
			Launch:      profile.LaunchShell,
			Image:       "test-image:latest",
		},
		HomeDir: t.TempDir(),
		WorkDir: t.TempDir(),
	}

	if err := s.Run(context.Background(), ec); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	for _, opt := range ec.DockerSecurityOpts {
		if opt == "label=type:spc_t" {
			t.Errorf("DockerSecurityOpts should NOT contain spc_t when no sockets and WorkDir != HomeDir, got %v", ec.DockerSecurityOpts)
		}
	}
}

func TestDockerStage_BuildArgs_AptMode(t *testing.T) {
	for _, tool := range []string{"claude", "codex", "opencode"} {
		t.Run(tool, func(t *testing.T) {
			dc := &mockDockerClient{available: true}
			s := &DockerStage{
				DockerClient: dc,
				ConfigSyncer: &mockConfigSyncer{},
				MountBuilder: &mockMountBuilder{},
			}

			var launch profile.LaunchMode
			switch tool {
			case "claude":
				launch = profile.LaunchClaude
			case "codex":
				launch = profile.LaunchCodex
			case "opencode":
				launch = profile.LaunchOpenCode
			}

			ec := &pipeline.ExecutionContext{
				Profile: profile.Profile{
					Environment: profile.EnvironmentContainer,
					Launch:      launch,
				},
				HomeDir: t.TempDir(),
				WorkDir: t.TempDir(),
			}

			if err := s.Run(context.Background(), ec); err != nil {
				t.Fatalf("Run() error: %v", err)
			}

			if !dc.buildCalled {
				t.Fatal("Build should be called")
			}

			if dc.buildArgs["AW_TOOL_INSTALL_SCRIPT"] == "" {
				t.Error("AW_TOOL_INSTALL_SCRIPT should be set")
			}
			if _, ok := dc.buildArgs["AW_TOOL_PKG"]; ok {
				t.Error("AW_TOOL_PKG should not be set in apt mode")
			}
		})
	}
}

func TestDockerStage_BuildArgs_DevboxMode(t *testing.T) {
	tests := []struct {
		tool    string
		wantPkg string
	}{
		{"claude", "claude-code"},
		{"codex", "codex"},
		{"opencode", "opencode"},
	}

	for _, tt := range tests {
		t.Run(tt.tool, func(t *testing.T) {
			dc := &mockDockerClient{available: true}
			s := &DockerStage{
				DockerClient: dc,
				ConfigSyncer: &mockConfigSyncer{},
				MountBuilder: &mockMountBuilder{},
			}

			var launch profile.LaunchMode
			switch tt.tool {
			case "claude":
				launch = profile.LaunchClaude
			case "codex":
				launch = profile.LaunchCodex
			case "opencode":
				launch = profile.LaunchOpenCode
			}

			ec := &pipeline.ExecutionContext{
				Profile: profile.Profile{
					Environment:    profile.EnvironmentContainer,
					Launch:         launch,
					PackageManager: profile.PackageManagerDevbox,
				},
				HomeDir: t.TempDir(),
				WorkDir: t.TempDir(),
			}

			if err := s.Run(context.Background(), ec); err != nil {
				t.Fatalf("Run() error: %v", err)
			}

			if dc.buildArgs["AW_TOOL_PKG"] != tt.wantPkg {
				t.Errorf("AW_TOOL_PKG = %q, want %q", dc.buildArgs["AW_TOOL_PKG"], tt.wantPkg)
			}
			if _, ok := dc.buildArgs["AW_TOOL_INSTALL_SCRIPT"]; ok {
				t.Error("AW_TOOL_INSTALL_SCRIPT should not be set in devbox mode")
			}
		})
	}
}

func TestDockerStage_ImageHash_DiffersByPackageManager(t *testing.T) {
	aptDC := &mockDockerClient{available: true}
	aptStage := &DockerStage{
		DockerClient: aptDC,
		ConfigSyncer: &mockConfigSyncer{},
		MountBuilder: &mockMountBuilder{},
	}

	aptEC := &pipeline.ExecutionContext{
		Profile: profile.Profile{
			Environment: profile.EnvironmentContainer,
			Launch:      profile.LaunchClaude,
		},
		HomeDir: t.TempDir(),
		WorkDir: t.TempDir(),
	}

	if err := aptStage.Run(context.Background(), aptEC); err != nil {
		t.Fatalf("apt Run() error: %v", err)
	}

	devboxDC := &mockDockerClient{available: true}
	devboxStage := &DockerStage{
		DockerClient: devboxDC,
		ConfigSyncer: &mockConfigSyncer{},
		MountBuilder: &mockMountBuilder{},
	}

	devboxEC := &pipeline.ExecutionContext{
		Profile: profile.Profile{
			Environment:    profile.EnvironmentContainer,
			Launch:         profile.LaunchClaude,
			PackageManager: profile.PackageManagerDevbox,
		},
		HomeDir: t.TempDir(),
		WorkDir: t.TempDir(),
	}

	if err := devboxStage.Run(context.Background(), devboxEC); err != nil {
		t.Fatalf("devbox Run() error: %v", err)
	}

	if aptDC.buildImageName == devboxDC.buildImageName {
		t.Errorf("apt and devbox should produce different image hashes, both got %q", aptDC.buildImageName)
	}
}

func TestDockerStage_DevboxMode_CopiesDevboxJSON(t *testing.T) {
	homeDir := t.TempDir()
	configDir := filepath.Join(homeDir, ".config", "aw")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}
	devboxJSON := `{"packages":["hello@latest"]}`
	if err := os.WriteFile(filepath.Join(configDir, "devbox.json"), []byte(devboxJSON), 0644); err != nil {
		t.Fatal(err)
	}

	dc := &mockDockerClient{available: true}
	s := &DockerStage{
		DockerClient: dc,
		ConfigSyncer: &mockConfigSyncer{},
		MountBuilder: &mockMountBuilder{},
	}

	ec := &pipeline.ExecutionContext{
		Profile: profile.Profile{
			Environment:    profile.EnvironmentContainer,
			Launch:         profile.LaunchClaude,
			PackageManager: profile.PackageManagerDevbox,
		},
		HomeDir: homeDir,
		WorkDir: t.TempDir(),
	}

	if err := s.Run(context.Background(), ec); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	data, ok := dc.buildContextFiles["devbox.json"]
	if !ok {
		t.Fatal("devbox.json should be copied to build context")
	}
	if string(data) != devboxJSON {
		t.Errorf("devbox.json content = %q, want %q", string(data), devboxJSON)
	}
}

func TestDockerStage_AptMode_NoDevboxJSON(t *testing.T) {
	homeDir := t.TempDir()
	configDir := filepath.Join(homeDir, ".config", "aw")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "devbox.json"), []byte(`{"packages":[]}`), 0644); err != nil {
		t.Fatal(err)
	}

	dc := &mockDockerClient{available: true}
	s := &DockerStage{
		DockerClient: dc,
		ConfigSyncer: &mockConfigSyncer{},
		MountBuilder: &mockMountBuilder{},
	}

	ec := &pipeline.ExecutionContext{
		Profile: profile.Profile{
			Environment: profile.EnvironmentContainer,
			Launch:      profile.LaunchClaude,
		},
		HomeDir: homeDir,
		WorkDir: t.TempDir(),
	}

	if err := s.Run(context.Background(), ec); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if _, ok := dc.buildContextFiles["devbox.json"]; ok {
		t.Error("devbox.json should not be copied to build context in apt mode")
	}
}

