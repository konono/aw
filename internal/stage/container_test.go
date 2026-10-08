package stage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/konono/aw/v4/internal/config"
	"github.com/konono/aw/v4/internal/docker"
	"github.com/konono/aw/v4/internal/mount"
	"github.com/konono/aw/v4/internal/pipeline"
	"github.com/konono/aw/v4/internal/profile"
)

type mockDockerClient struct {
	available         bool
	buildCalled       bool
	buildImageName    string
	buildContextDir   string
	buildArgs         map[string]string
	buildContextFiles map[string][]byte
	runCalled         bool
	runConfig         docker.RunConfig
	imageExists       bool
	imageExistsCalled bool
	imageExistsErr    error
	saveCalled        bool
	saveImageName     string
	saveOutputPath    string
	pullCalled        bool
	pullImages        []string
	pullSucceeds      bool
}

func (m *mockDockerClient) CheckAvailable() error {
	if !m.available {
		return fmt.Errorf("docker not available")
	}
	return nil
}

func (m *mockDockerClient) Build(_ context.Context, imageName, contextDir, _ string, buildArgs map[string]string, _ bool) error {
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

func (m *mockDockerClient) Pull(_ context.Context, imageName string) error {
	m.pullCalled = true
	m.pullImages = append(m.pullImages, imageName)
	if m.pullSucceeds {
		return nil
	}
	return fmt.Errorf("image not found in registry")
}

func (m *mockDockerClient) RemoveContainer(_ context.Context, _ string) error {
	return nil
}

func (m *mockDockerClient) Tag(_ context.Context, _, _ string) error {
	return nil
}

func (m *mockDockerClient) Push(_ context.Context, _ string) error {
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
	mounts   []docker.Mount
	err      error
	lastOpts mount.MountOptions
}

func (m *mockMountBuilder) BuildMounts(opts mount.MountOptions) ([]docker.Mount, error) {
	m.lastOpts = opts
	result := append(m.mounts, opts.ExtraMounts...)
	return result, m.err
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
	if !strings.Contains(content, "CONTAINER_HOST") {
		t.Error("Docker section should mention CONTAINER_HOST")
	}
	if !strings.Contains(content, "mise-installed podman") {
		t.Error("Docker section should include mise shim naming note")
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

func TestAppendContainerContext_AutoDepsInstall(t *testing.T) {
	tmpDir := t.TempDir()
	autoDeps := true
	ec := &pipeline.ExecutionContext{
		Profile: profile.Profile{
			AutoDepsInstall: &autoDeps,
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

	if !strings.Contains(content, "## Language Dependencies") {
		t.Error("missing Language Dependencies section when auto_deps_install is enabled")
	}
	if !strings.Contains(content, "auto_deps_install is enabled") {
		t.Error("Language Dependencies section should mention auto_deps_install")
	}
}

func TestAppendContainerContext_NoAutoDepsInstall(t *testing.T) {
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

	if strings.Contains(content, "## Language Dependencies") {
		t.Error("Language Dependencies section should not appear when auto_deps_install is disabled")
	}
}

func TestAppendContainerContext_ImageWithGhToken(t *testing.T) {
	tmpDir := t.TempDir()
	ghToken := true
	ec := &pipeline.ExecutionContext{
		Profile: profile.Profile{
			Image:   "my-image:latest",
			GhToken: &ghToken,
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

	if !strings.Contains(content, "## GitHub Token") {
		t.Error("image mode should use 'GitHub Token' section header")
	}
	if !strings.Contains(content, "gh CLI may not be available") {
		t.Error("image mode should warn that gh CLI may not be available")
	}
	if strings.Contains(content, "## GitHub CLI") {
		t.Error("image mode should not use 'GitHub CLI' section header")
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

	if err := s.Run(context.Background(), ec); err != nil {
		t.Fatalf("Run() should warn and fall back, got error: %v", err)
	}
	if !dc.buildCalled {
		t.Error("Build should be called when the pinned image is missing")
	}
	if ec.DockerImage == "nonexistent:v1" {
		t.Error("DockerImage should not be the missing pinned image")
	}
}

func TestDockerStage_PrebuiltImage_NotFound_PullAlwaysIsFatal(t *testing.T) {
	dc := &mockDockerClient{available: true, imageExists: false, pullSucceeds: false}
	s := &DockerStage{
		DockerClient: dc,
		ConfigSyncer: &mockConfigSyncer{},
		MountBuilder: &mockMountBuilder{},
	}

	ec := &pipeline.ExecutionContext{
		Profile: profile.Profile{
			Environment:     profile.EnvironmentContainer,
			Launch:          profile.LaunchShell,
			Image:           "ghcr.io/konono/gone:v1",
			ImagePullPolicy: profile.ImagePullPolicyAlways,
		},
		HomeDir: t.TempDir(),
		WorkDir: t.TempDir(),
	}

	err := s.Run(context.Background(), ec)
	if err == nil {
		t.Fatal("Run() should return error when image_pull_policy: always and the pull fails")
	}
	if !strings.Contains(err.Error(), "pulling image") {
		t.Errorf("error = %q, want containing 'pulling image'", err.Error())
	}
}

func TestDockerStage_PrebuiltImage_NotFound_PullsValidRefs(t *testing.T) {
	// Any reference docker/podman can resolve must be pulled before giving up
	// on it -- including bare Docker Hub names, which carry no registry host.
	tests := []struct {
		name  string
		image string
	}{
		{"registry host", "ghcr.io/konono/aw-claude:1.0.0-debian12"},
		{"docker hub official", "alpine:latest"},
		{"docker hub namespaced", "myorg/image:tag"},
		{"locally built tag", "aw-container:08b2c728bba5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dc := &mockDockerClient{available: true, imageExists: false, pullSucceeds: true}
			s := &DockerStage{
				DockerClient: dc,
				ConfigSyncer: &mockConfigSyncer{},
				MountBuilder: &mockMountBuilder{},
			}

			ec := &pipeline.ExecutionContext{
				Profile: profile.Profile{
					Environment: profile.EnvironmentContainer,
					Launch:      profile.LaunchShell,
					Image:       tt.image,
				},
				HomeDir: t.TempDir(),
				WorkDir: t.TempDir(),
			}

			if err := s.Run(context.Background(), ec); err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if !slices.Contains(dc.pullImages, tt.image) {
				t.Errorf("Pull should be called for %q, got %v", tt.image, dc.pullImages)
			}
			if ec.DockerImage != tt.image {
				t.Errorf("DockerImage = %q, want %q", ec.DockerImage, tt.image)
			}
			if dc.buildCalled {
				t.Error("Build should not be called when the pinned image pulls successfully")
			}
		})
	}
}

func TestDockerStage_PrebuiltImage_NotFound_InvalidRefSkipsPull(t *testing.T) {
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
			Image:       "Bad::Ref",
		},
		HomeDir: t.TempDir(),
		WorkDir: t.TempDir(),
	}

	if err := s.Run(context.Background(), ec); err != nil {
		t.Fatalf("Run() should warn and fall back, got error: %v", err)
	}
	if slices.Contains(dc.pullImages, "Bad::Ref") {
		t.Error("Pull should not be attempted for an unparseable reference")
	}
	if !dc.buildCalled {
		t.Error("Build should be called after falling back")
	}
}

// A profile can pin `image:` together with an install opt-out, because that
// snapshot image has the tools baked in. When the image is gone we fall back
// to one without them, so the skips must be dropped too.
func TestDockerStage_PrebuiltImage_NotFound_ReenablesInstallsFromApply(t *testing.T) {
	dc := &mockDockerClient{available: true, imageExists: false}
	s := &DockerStage{
		DockerClient: dc,
		ConfigSyncer: &mockConfigSyncer{},
		MountBuilder: &mockMountBuilder{},
	}

	skip := true
	ec := &pipeline.ExecutionContext{
		Profile: profile.Profile{
			Environment:     profile.EnvironmentContainer,
			Launch:          profile.LaunchShell,
			Image:           "aw-container-myprofile:08b2c728bba5",
			SkipMiseInstall: &skip,
		},
		HomeDir: t.TempDir(),
		WorkDir: t.TempDir(),
	}

	if err := s.Run(context.Background(), ec); err != nil {
		t.Fatalf("Run() should warn and fall back, got error: %v", err)
	}
	if !dc.buildCalled {
		t.Fatal("Build should be called after falling back")
	}
	if ec.Profile.EffectiveSkipMiseInstall() {
		t.Error("mise install should be re-enabled after falling back")
	}

	env := pipeline.ContainerEnvVars(ec, "claude")
	if _, ok := env["AW_SKIP_MISE_INSTALL"]; ok {
		t.Error("AW_SKIP_MISE_INSTALL should not be set for the fallback image")
	}
}

// A pinned image that is present keeps its snapshot install skips.
func TestDockerStage_PrebuiltImage_Found_KeepsInstallSkips(t *testing.T) {
	dc := &mockDockerClient{available: true, imageExists: true}
	s := &DockerStage{
		DockerClient: dc,
		ConfigSyncer: &mockConfigSyncer{},
		MountBuilder: &mockMountBuilder{},
	}

	skip := true
	ec := &pipeline.ExecutionContext{
		Profile: profile.Profile{
			Environment:     profile.EnvironmentContainer,
			Launch:          profile.LaunchShell,
			Image:           "aw-container-myprofile:08b2c728bba5",
			SkipMiseInstall: &skip,
		},
		HomeDir: t.TempDir(),
		WorkDir: t.TempDir(),
	}

	if err := s.Run(context.Background(), ec); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !ec.Profile.EffectiveSkipMiseInstall() {
		t.Error("mise install skip should be preserved when the pinned image is used")
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

	if err := s.Run(context.Background(), ec); err != nil {
		t.Fatalf("Run() should warn and fall back on inspect errors, got: %v", err)
	}
	if !dc.buildCalled {
		t.Error("Build should be called after an image inspect error")
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

func TestDockerStage_MountsAwInitScript(t *testing.T) {
	dc := &mockDockerClient{available: true}
	s := &DockerStage{
		DockerClient: dc,
		ConfigSyncer: &mockConfigSyncer{},
		MountBuilder: &mockMountBuilder{},
	}

	homeDir := t.TempDir()
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

	initScriptPath := filepath.Join(homeDir, ".agent-workspace", "aw-init.sh")
	if _, err := os.Stat(initScriptPath); err != nil {
		t.Fatalf("aw-init.sh should be written to staging dir: %v", err)
	}

	found := false
	for _, m := range ec.DockerMounts {
		if m.Target == "/aw-init.sh" {
			found = true
			if m.Source != initScriptPath {
				t.Errorf("aw-init.sh mount source = %q, want %q", m.Source, initScriptPath)
			}
			if !m.ReadOnly {
				t.Error("aw-init.sh mount should be read-only")
			}
			break
		}
	}
	if !found {
		t.Error("DockerMounts should contain /aw-init.sh mount")
	}
}

func TestDockerStage_BuildArgs_AptMode(t *testing.T) {
	for _, tool := range []string{"claude", "codex", "opencode", "cursor"} {
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
			case "cursor":
				launch = profile.LaunchCursor
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

func TestDockerStage_ExtraPackages_BuildArg(t *testing.T) {
	workDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(workDir, "packages.txt"), []byte("jq\ntree\n"), 0644); err != nil {
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
			Environment:     profile.EnvironmentContainer,
			Launch:          profile.LaunchClaude,
			ImagePullPolicy: profile.ImagePullPolicyBuild,
		},
		HomeDir:     t.TempDir(),
		WorkDir:     workDir,
		OrigWorkDir: workDir,
	}

	if err := s.Run(context.Background(), ec); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if dc.buildArgs["AW_EXTRA_PACKAGES"] != "jq tree" {
		t.Errorf("AW_EXTRA_PACKAGES = %q, want %q", dc.buildArgs["AW_EXTRA_PACKAGES"], "jq tree")
	}
}

func TestDockerStage_ProfilePackages_BuildArg(t *testing.T) {
	dc := &mockDockerClient{available: true}
	s := &DockerStage{
		DockerClient: dc,
		ConfigSyncer: &mockConfigSyncer{},
		MountBuilder: &mockMountBuilder{},
	}

	ec := &pipeline.ExecutionContext{
		Profile: profile.Profile{
			Environment:     profile.EnvironmentContainer,
			Launch:          profile.LaunchClaude,
			ImagePullPolicy: profile.ImagePullPolicyBuild,
			Packages:        []string{"curl", "wget"},
		},
		HomeDir: t.TempDir(),
		WorkDir: t.TempDir(),
	}

	if err := s.Run(context.Background(), ec); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if dc.buildArgs["AW_EXTRA_PACKAGES"] != "curl wget" {
		t.Errorf("AW_EXTRA_PACKAGES = %q, want %q", dc.buildArgs["AW_EXTRA_PACKAGES"], "curl wget")
	}
}

func TestDockerStage_NoPackages_NoBuildArg(t *testing.T) {
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
		HomeDir: t.TempDir(),
		WorkDir: t.TempDir(),
	}

	if err := s.Run(context.Background(), ec); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if _, ok := dc.buildArgs["AW_EXTRA_PACKAGES"]; ok {
		t.Error("AW_EXTRA_PACKAGES should not be set when no packages")
	}
}

func TestDockerStage_Packages_CustomDockerfile_NoBuildArg(t *testing.T) {
	homeDir := t.TempDir()

	dockerfileDir := filepath.Join(homeDir, "docker")
	if err := os.MkdirAll(dockerfileDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dockerfileDir, "Dockerfile"), []byte("FROM ubuntu:22.04\n"), 0644); err != nil {
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
			Dockerfile:  filepath.Join(dockerfileDir, "Dockerfile"),
			Packages:    []string{"jq"},
		},
		HomeDir: homeDir,
		WorkDir: t.TempDir(),
	}

	if err := s.Run(context.Background(), ec); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if _, ok := dc.buildArgs["AW_EXTRA_PACKAGES"]; ok {
		t.Error("AW_EXTRA_PACKAGES should not be set with custom Dockerfile")
	}
}

func TestDockerStage_ImageHash_DiffersByPackages(t *testing.T) {
	dc1 := &mockDockerClient{available: true}
	s1 := &DockerStage{
		DockerClient: dc1,
		ConfigSyncer: &mockConfigSyncer{},
		MountBuilder: &mockMountBuilder{},
	}

	ec1 := &pipeline.ExecutionContext{
		Profile: profile.Profile{
			Environment:     profile.EnvironmentContainer,
			Launch:          profile.LaunchClaude,
			ImagePullPolicy: profile.ImagePullPolicyBuild,
		},
		HomeDir: t.TempDir(),
		WorkDir: t.TempDir(),
	}

	if err := s1.Run(context.Background(), ec1); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	dc2 := &mockDockerClient{available: true}
	s2 := &DockerStage{
		DockerClient: dc2,
		ConfigSyncer: &mockConfigSyncer{},
		MountBuilder: &mockMountBuilder{},
	}

	ec2 := &pipeline.ExecutionContext{
		Profile: profile.Profile{
			Environment:     profile.EnvironmentContainer,
			Launch:          profile.LaunchClaude,
			ImagePullPolicy: profile.ImagePullPolicyBuild,
			Packages:        []string{"jq"},
		},
		HomeDir: t.TempDir(),
		WorkDir: t.TempDir(),
	}

	if err := s2.Run(context.Background(), ec2); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if dc1.buildImageName == dc2.buildImageName {
		t.Errorf("image hashes should differ with/without packages, both got %q", dc1.buildImageName)
	}
}

func TestDockerStage_BuildEnv_PassedAsBuildArgs(t *testing.T) {
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
			BuildEnv:    map[string]string{"HTTP_PROXY": "http://proxy:8080", "HTTPS_PROXY": "http://proxy:8080"},
		},
		HomeDir: t.TempDir(),
		WorkDir: t.TempDir(),
	}

	if err := s.Run(context.Background(), ec); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if dc.buildArgs["HTTP_PROXY"] != "http://proxy:8080" {
		t.Errorf("HTTP_PROXY build arg = %q, want %q", dc.buildArgs["HTTP_PROXY"], "http://proxy:8080")
	}
	if dc.buildArgs["HTTPS_PROXY"] != "http://proxy:8080" {
		t.Errorf("HTTPS_PROXY build arg = %q, want %q", dc.buildArgs["HTTPS_PROXY"], "http://proxy:8080")
	}
}

func TestDockerStage_CACert_CopiedToBuildContext(t *testing.T) {
	homeDir := t.TempDir()
	certContent := []byte("-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----\n")
	certPath := filepath.Join(homeDir, "corp-ca.pem")
	if err := os.WriteFile(certPath, certContent, 0644); err != nil {
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
			CACert:      certPath,
		},
		HomeDir: homeDir,
		WorkDir: t.TempDir(),
	}

	if err := s.Run(context.Background(), ec); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	got, ok := dc.buildContextFiles["ca-cert.pem"]
	if !ok {
		t.Fatal("ca-cert.pem not found in build context")
	}
	if string(got) != string(certContent) {
		t.Errorf("ca-cert.pem content = %q, want %q", string(got), string(certContent))
	}
}

func TestDockerStage_BuildEnv_AffectsImageHash(t *testing.T) {
	dc1 := &mockDockerClient{available: true}
	s1 := &DockerStage{
		DockerClient: dc1,
		ConfigSyncer: &mockConfigSyncer{},
		MountBuilder: &mockMountBuilder{},
	}

	ec1 := &pipeline.ExecutionContext{
		Profile: profile.Profile{
			Environment: profile.EnvironmentContainer,
			Launch:      profile.LaunchClaude,
		},
		HomeDir: t.TempDir(),
		WorkDir: t.TempDir(),
	}

	if err := s1.Run(context.Background(), ec1); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	dc2 := &mockDockerClient{available: true}
	s2 := &DockerStage{
		DockerClient: dc2,
		ConfigSyncer: &mockConfigSyncer{},
		MountBuilder: &mockMountBuilder{},
	}

	ec2 := &pipeline.ExecutionContext{
		Profile: profile.Profile{
			Environment: profile.EnvironmentContainer,
			Launch:      profile.LaunchClaude,
			BuildEnv:    map[string]string{"HTTP_PROXY": "http://proxy:8080"},
		},
		HomeDir: t.TempDir(),
		WorkDir: t.TempDir(),
	}

	if err := s2.Run(context.Background(), ec2); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if dc1.buildImageName == dc2.buildImageName {
		t.Errorf("image hashes should differ with/without build_env, both got %q", dc1.buildImageName)
	}
}

func TestResolveOfficialImage_AutoLocalExists(t *testing.T) {
	tmpDir := t.TempDir()
	dc := &mockDockerClient{available: true, imageExists: true}
	setupToolConfig(t, tmpDir, "claude")

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
		HomeDir: tmpDir,
		WorkDir: tmpDir,
	}

	if err := s.Run(context.Background(), ec); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if dc.pullCalled {
		t.Error("auto + local exists: Pull should not be called")
	}
	if dc.buildCalled {
		t.Error("auto + local exists: Build should not be called")
	}
}

func TestResolveOfficialImage_AutoPullSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	dc := &mockDockerClient{available: true, imageExists: false, pullSucceeds: true}
	setupToolConfig(t, tmpDir, "claude")

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
		HomeDir: tmpDir,
		WorkDir: tmpDir,
	}

	if err := s.Run(context.Background(), ec); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if !dc.pullCalled {
		t.Error("auto + not local: Pull should be called")
	}
	if dc.buildCalled {
		t.Error("auto + pull success: Build should not be called")
	}
}

func TestResolveOfficialImage_AutoPullFail(t *testing.T) {
	tmpDir := t.TempDir()
	dc := &mockDockerClient{available: true, imageExists: false}
	setupToolConfig(t, tmpDir, "claude")

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
		HomeDir: tmpDir,
		WorkDir: tmpDir,
	}

	if err := s.Run(context.Background(), ec); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if !dc.pullCalled {
		t.Error("auto + not local: Pull should be called")
	}
	if !dc.buildCalled {
		t.Error("auto + pull fail: Build should be called as fallback")
	}
}

func TestResolveOfficialImage_BuildPolicy(t *testing.T) {
	tmpDir := t.TempDir()
	dc := &mockDockerClient{available: true, imageExists: true}
	setupToolConfig(t, tmpDir, "claude")

	s := &DockerStage{
		DockerClient: dc,
		ConfigSyncer: &mockConfigSyncer{},
		MountBuilder: &mockMountBuilder{},
	}
	ec := &pipeline.ExecutionContext{
		Profile: profile.Profile{
			Environment:     profile.EnvironmentContainer,
			Launch:          profile.LaunchClaude,
			ImagePullPolicy: profile.ImagePullPolicyBuild,
		},
		HomeDir: tmpDir,
		WorkDir: tmpDir,
	}

	if err := s.Run(context.Background(), ec); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if dc.pullCalled {
		t.Error("build policy: Pull should not be called")
	}
	if !dc.buildCalled {
		t.Error("build policy: Build should be called")
	}
}

func TestResolveOfficialImage_PackagesInstallAtStartup(t *testing.T) {
	for _, source := range []string{"profile", "packages.txt"} {
		t.Run(source, func(t *testing.T) {
			workDir := t.TempDir()
			homeDir := t.TempDir()
			p := profile.Profile{Environment: profile.EnvironmentContainer, Launch: profile.LaunchClaude}
			if source == "profile" {
				p.Packages = []string{"jq"}
			} else if err := os.WriteFile(filepath.Join(workDir, "packages.txt"), []byte("jq\n"), 0644); err != nil {
				t.Fatal(err)
			}

			dc := &mockDockerClient{available: true, imageExists: true}
			setupToolConfig(t, homeDir, "claude")
			s := &DockerStage{
				DockerClient: dc,
				ConfigSyncer: &mockConfigSyncer{},
				MountBuilder: &mockMountBuilder{},
			}
			ec := &pipeline.ExecutionContext{
				Profile: p, HomeDir: homeDir, WorkDir: workDir, OrigWorkDir: workDir,
			}

			if err := s.Run(context.Background(), ec); err != nil {
				t.Fatalf("Run() error: %v", err)
			}
			if dc.buildCalled || dc.pullCalled || !dc.imageExistsCalled {
				t.Errorf("expected local official image without a build, got build=%v pull=%v inspect=%v", dc.buildCalled, dc.pullCalled, dc.imageExistsCalled)
			}
			if ec.DockerImage != OfficialImageName("claude", profile.OSDebian12) {
				t.Errorf("DockerImage = %q, want official image", ec.DockerImage)
			}
			if got := pipeline.ContainerEnvVars(ec, "claude")["AW_PACKAGES"]; got != "jq" {
				t.Errorf("AW_PACKAGES = %q, want jq", got)
			}
		})
	}
}

func TestResolveOfficialImage_PackagesFilePullsOfficial(t *testing.T) {
	workDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(workDir, "packages.txt"), []byte("gcc\n"), 0644); err != nil {
		t.Fatal(err)
	}
	homeDir := t.TempDir()
	setupToolConfig(t, homeDir, "claude")
	dc := &mockDockerClient{available: true, pullSucceeds: true}
	s := &DockerStage{DockerClient: dc, ConfigSyncer: &mockConfigSyncer{}, MountBuilder: &mockMountBuilder{}}
	ec := &pipeline.ExecutionContext{
		Profile: profile.Profile{Environment: profile.EnvironmentContainer, Launch: profile.LaunchClaude},
		HomeDir: homeDir, WorkDir: workDir, OrigWorkDir: workDir,
	}

	if err := s.Run(context.Background(), ec); err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if dc.buildCalled || !dc.pullCalled {
		t.Errorf("packages.txt should pull the official image without building, got build=%v pull=%v", dc.buildCalled, dc.pullCalled)
	}
	if got := pipeline.ContainerEnvVars(ec, "claude")["AW_PACKAGES"]; got != "gcc" {
		t.Errorf("AW_PACKAGES = %q, want gcc", got)
	}
}

func TestResolveOfficialImage_ShellUsesBase(t *testing.T) {
	tmpDir := t.TempDir()
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
		},
		HomeDir: tmpDir,
		WorkDir: tmpDir,
	}

	if err := s.Run(context.Background(), ec); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if dc.pullCalled {
		t.Error("shell + base local exists: Pull should not be called")
	}
	if dc.buildCalled {
		t.Error("shell + base local exists: Build should not be called")
	}
}

func TestResolveOfficialImage_ShellPullSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	dc := &mockDockerClient{available: true, imageExists: false, pullSucceeds: true}

	s := &DockerStage{
		DockerClient: dc,
		ConfigSyncer: &mockConfigSyncer{},
		MountBuilder: &mockMountBuilder{},
	}
	ec := &pipeline.ExecutionContext{
		Profile: profile.Profile{
			Environment: profile.EnvironmentContainer,
			Launch:      profile.LaunchShell,
		},
		HomeDir: tmpDir,
		WorkDir: tmpDir,
	}

	if err := s.Run(context.Background(), ec); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if !dc.pullCalled {
		t.Error("shell + not local: Pull should be called")
	}
	if dc.buildCalled {
		t.Error("shell + pull success: Build should not be called")
	}
}

func TestResolveOfficialImage_ShellPullFail(t *testing.T) {
	tmpDir := t.TempDir()
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
		},
		HomeDir: tmpDir,
		WorkDir: tmpDir,
	}

	if err := s.Run(context.Background(), ec); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if !dc.pullCalled {
		t.Error("shell + not local: Pull should be called")
	}
	if !dc.buildCalled {
		t.Error("shell + pull fail: Build should be called as fallback")
	}
}

func TestHasBuildCustomizations_SessionLog(t *testing.T) {
	tests := []struct {
		name string
		k    *profile.KubernetesConfig
		want bool
	}{
		{"nil kubernetes", nil, false},
		{"session_log false", &profile.KubernetesConfig{SessionLog: false}, false},
		{"session_log true", &profile.KubernetesConfig{SessionLog: true}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ec := &pipeline.ExecutionContext{
				Profile:     profile.Profile{Kubernetes: tt.k},
				OrigWorkDir: t.TempDir(),
			}
			got := HasBuildCustomizations(ec)
			if got != tt.want {
				t.Errorf("HasBuildCustomizations() = %v, want %v", got, tt.want)
			}
		})
	}
}

func setupToolConfig(t *testing.T, homeDir, tool string) {
	t.Helper()
	toolDir := filepath.Join(homeDir, ".agent-workspace", tool)
	if err := os.MkdirAll(toolDir, 0o755); err != nil {
		t.Fatalf("creating tool dir: %v", err)
	}
}
