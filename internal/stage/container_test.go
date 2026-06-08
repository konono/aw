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
	available         bool
	buildCalled       bool
	volumeCalled      bool
	runCalled         bool
	runConfig         docker.RunConfig
	imageExists       bool
	imageExistsCalled bool
	imageExistsErr    error
	saveCalled        bool
	saveImageName     string
	saveOutputPath    string
}

func (m *mockDockerClient) CheckAvailable() error {
	if !m.available {
		return fmt.Errorf("docker not available")
	}
	return nil
}

func (m *mockDockerClient) Build(_ context.Context, _, _, _ string, _ map[string]string) error {
	m.buildCalled = true
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

func TestResolveDockerfilePath_Absolute(t *testing.T) {
	absPath := "/absolute/path/Dockerfile"
	resolved, err := resolveDockerfilePath(absPath)
	if err != nil {
		t.Fatalf("resolveDockerfilePath() error: %v", err)
	}
	if resolved != absPath {
		t.Errorf("resolved = %q, want %q", resolved, absPath)
	}
}

func TestDockerStage_Name(t *testing.T) {
	s := &DockerStage{}
	if s.Name() != "container" {
		t.Errorf("Name() = %q, want %q", s.Name(), "container")
	}
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

func TestAppendContainerContext_AllFeatures(t *testing.T) {
	tmpDir := t.TempDir()
	mountGH := true
	ec := &pipeline.ExecutionContext{
		Profile: profile.Profile{
			MountGH: &mountGH,
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
	if !strings.Contains(content, "## SSH Agent") {
		t.Error("missing SSH Agent section")
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

func TestDockerStage_NewDockerStage(t *testing.T) {
	s := NewDockerStage()
	// DockerClient is nil by default; initialized lazily in Run() from profile's container_runtime
	if s.DockerClient != nil {
		t.Error("DockerClient should be nil (lazy init)")
	}
	if s.ConfigSyncer == nil {
		t.Error("ConfigSyncer should not be nil")
	}
	if s.MountBuilder == nil {
		t.Error("MountBuilder should not be nil")
	}
}
