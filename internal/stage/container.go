package stage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/konono/aw/v4/internal/config"
	"github.com/konono/aw/v4/internal/containerenv"
	"github.com/konono/aw/v4/internal/docker"
	"github.com/konono/aw/v4/internal/image"
	"github.com/konono/aw/v4/internal/mount"
	"github.com/konono/aw/v4/internal/pathutil"
	"github.com/konono/aw/v4/internal/pipeline"
	"github.com/konono/aw/v4/internal/profile"
	"github.com/konono/aw/v4/internal/sshagent"
	"github.com/konono/aw/v4/internal/toolinfo"
	"github.com/konono/aw/v4/internal/version"
)

const (
	defaultImageName        = "aw-container"
	initScriptContainerPath = "/aw-init.sh"
)

// DockerStage builds the Docker image, creates volumes, syncs config, and builds mounts.
type DockerStage struct {
	DockerClient docker.Client
	ConfigSyncer config.Syncer
	MountBuilder mount.Builder
}

// NewDockerStage creates a DockerStage with default implementations.
func NewDockerStage() *DockerStage {
	return &DockerStage{
		ConfigSyncer: config.NewSyncer(),
		MountBuilder: mount.NewBuilder(),
	}
}

func (s *DockerStage) Name() string { return "container" }

func (s *DockerStage) Run(ctx context.Context, ec *pipeline.ExecutionContext) error {
	if s.DockerClient == nil {
		s.DockerClient = docker.NewShellClient(ec.Profile.EffectiveContainerRuntime())
	}

	if err := s.DockerClient.CheckAvailable(); err != nil {
		return fmt.Errorf("container runtime is not available: %w", err)
	}

	cenv := containerenv.FromUser(ec.Profile.EffectiveContainerUser())
	if ec.Profile.Kubernetes != nil && ec.Profile.Kubernetes.SessionLog {
		cenv.SessionLog = true
	}
	ec.ContainerEnv = cenv

	imageName, err := s.resolveImage(ctx, ec, cenv)
	if err != nil {
		return err
	}

	tool := ec.Profile.EffectiveTool()
	toolStageDir, toolContainerDir, err := s.syncToolConfig(ec, tool, cenv)
	if err != nil {
		return err
	}

	extraMounts := s.buildExtraMounts(ec)
	sshAuthSock, containerSockPath := s.setupContainerFeatures(ec)

	if err := appendContainerContext(toolStageDir, ec); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: appending container context: %v\n", err)
	}

	sshAgentFwd := ec.Profile.EffectiveSSHAgentForwarding()
	mounts, err := s.MountBuilder.BuildMounts(mount.MountOptions{
		HomeDir:            ec.HomeDir,
		WorkDir:            ec.WorkDir,
		ContainerHome:      cenv.Home,
		ToolStageDir:       toolStageDir,
		ToolContainerDir:   toolContainerDir,
		MountGH:            ec.Profile.EffectiveMountGH(),
		MountSSH:           ec.Profile.EffectiveMountSSH(),
		SSHAgentForwarding: sshAgentFwd,
		SSHAuthSock:        sshAuthSock,
		MountContainerSock: ec.Profile.EffectiveMountContainerSock(),
		ContainerSockPath:  containerSockPath,
		ExtraMounts:        extraMounts,
	})
	if err != nil {
		return fmt.Errorf("building mounts: %w", err)
	}

	// spc_t grants connectto permissions for socket mounts (SSH agent,
	// container runtime) where container_t lacks connectto to
	// unconfined_t, and file access when :z can't be applied (home
	// directory). label=disable is NOT used because it breaks overlay
	// flock on RHEL10.
	if ec.SSHAgentReady || ec.ContainerSockReady || filepath.Clean(ec.WorkDir) == filepath.Clean(ec.HomeDir) {
		ec.DockerSecurityOpts = append(ec.DockerSecurityOpts, "label=type:spc_t")
	}

	ec.DockerImage = imageName
	ec.DockerMounts = mounts
	ec.DockerCapAdd = append(ec.DockerCapAdd, "AUDIT_WRITE")

	return nil
}

func (s *DockerStage) resolveImage(ctx context.Context, ec *pipeline.ExecutionContext, cenv containerenv.Config) (string, error) {
	if ec.Profile.Image != "" {
		imageName := ec.Profile.Image
		exists, err := s.DockerClient.ImageExists(ctx, imageName)
		if err != nil {
			return "", fmt.Errorf("checking image %q: %w", imageName, err)
		}
		if !exists {
			return "", fmt.Errorf("image %q not found; load it via '%s load'",
				imageName, ec.Profile.EffectiveContainerRuntime())
		}
		fmt.Fprintf(os.Stderr, "Using pre-built image '%s'...\n", imageName)
		return imageName, nil
	}

	if ec.Profile.Dockerfile == "" && !HasBuildCustomizations(ec) {
		imageName, err := s.resolveOfficialImage(ctx, ec)
		if err != nil && ec.Profile.EffectiveImagePullPolicy() == profile.ImagePullPolicyAlways {
			return "", err
		}
		if err == nil && imageName != "" {
			return imageName, nil
		}
	}

	return s.buildImage(ctx, ec, cenv)
}

// HasBuildCustomizations reports whether the profile has settings that require
// building from template instead of using the official prebuilt image.
func HasBuildCustomizations(ec *pipeline.ExecutionContext) bool {
	p := ec.Profile
	if len(p.Packages) > 0 || len(p.BuildEnv) > 0 || p.CACert != "" ||
		p.PackageManager == profile.PackageManagerDevbox ||
		(p.ContainerUser != "" && p.ContainerUser != "agent") {
		return true
	}
	if p.Kubernetes != nil && p.Kubernetes.SessionLog {
		return true
	}
	if pkgs := pipeline.CollectPackages(nil, ec.OrigWorkDir); len(pkgs) > 0 {
		return true
	}
	return false
}

const OfficialImageRegistry = "ghcr.io/konono"

// OfficialImageName returns the GHCR image reference for the given tool and OS.
func OfficialImageName(tool string, os profile.OSTemplate) string {
	return fmt.Sprintf("%s/aw-%s:%s-%s", OfficialImageRegistry, tool, version.Version, os)
}

func (s *DockerStage) resolveOfficialImage(ctx context.Context, ec *pipeline.ExecutionContext) (string, error) {
	policy := ec.Profile.EffectiveImagePullPolicy()
	if policy == profile.ImagePullPolicyBuild {
		return "", nil
	}

	tool := toolinfo.ImageTool(ec.Profile.EffectiveTool())

	imageName := OfficialImageName(tool, ec.Profile.EffectiveOS())

	switch policy {
	case profile.ImagePullPolicyAlways:
		fmt.Fprintf(os.Stderr, "Pulling official image '%s'...\n", imageName)
		if err := s.DockerClient.Pull(ctx, imageName); err != nil {
			return "", fmt.Errorf("pulling official image: %w", err)
		}
		return imageName, nil

	case profile.ImagePullPolicyNever:
		exists, err := s.DockerClient.ImageExists(ctx, imageName)
		if err != nil || !exists {
			return "", nil
		}
		fmt.Fprintf(os.Stderr, "Using official image '%s'...\n", imageName)
		return imageName, nil

	default: // auto
		exists, err := s.DockerClient.ImageExists(ctx, imageName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: checking official image: %v; falling back to build\n", err)
			return "", nil
		}
		if exists {
			fmt.Fprintf(os.Stderr, "Using official image '%s'...\n", imageName)
			return imageName, nil
		}
		fmt.Fprintf(os.Stderr, "Pulling official image '%s'...\n", imageName)
		if err := s.DockerClient.Pull(ctx, imageName); err != nil {
			fmt.Fprintf(os.Stderr, "Official image not available; building from template\n")
			return "", nil
		}
		return imageName, nil
	}
}

func (s *DockerStage) syncToolConfig(ec *pipeline.ExecutionContext, tool string, cenv containerenv.Config) (toolStageDir, toolContainerDir string, err error) {
	stageDir := filepath.Join(ec.HomeDir, ".agent-workspace")

	initScriptPath := filepath.Join(stageDir, "aw-init.sh")
	if err := os.MkdirAll(stageDir, 0755); err != nil {
		return "", "", fmt.Errorf("creating stage dir: %w", err)
	}
	if err := os.WriteFile(initScriptPath, image.InitScript(), 0755); err != nil {
		return "", "", fmt.Errorf("writing aw-init.sh: %w", err)
	}

	spec := toolSyncSpec(tool, ec.Profile)
	if spec != nil {
		srcDir := toolinfo.HomePath(tool, ec.HomeDir)
		toolStageDir = filepath.Join(stageDir, tool)
		toolContainerDir = toolinfo.ContainerDirFor(tool, cenv)
		if err := s.ConfigSyncer.SyncToolSettings(srcDir, toolStageDir, *spec); err != nil {
			return "", "", fmt.Errorf("syncing %s settings: %w", tool, err)
		}
	}

	if tool == "cursor" && toolStageDir != "" {
		if err := seedCursorAuth(toolStageDir, ec.HomeDir); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: cursor auth seed: %v\n", err)
		}
	}

	if tool == "claude" && toolStageDir != "" {
		onboardingPath := filepath.Join(toolStageDir, ".claude.json")
		if err := s.ConfigSyncer.EnsureOnboardingState(onboardingPath); err != nil {
			return "", "", fmt.Errorf("ensuring onboarding state: %w", err)
		}
	}

	return toolStageDir, toolContainerDir, nil
}

func (s *DockerStage) buildExtraMounts(ec *pipeline.ExecutionContext) []docker.Mount {
	stageDir := filepath.Join(ec.HomeDir, ".agent-workspace")
	initScriptPath := filepath.Join(stageDir, "aw-init.sh")

	extraMounts := []docker.Mount{
		{
			Source:   initScriptPath,
			Target:   initScriptContainerPath,
			ReadOnly: true,
		},
	}
	for _, m := range ec.Profile.Mounts {
		source := pathutil.ExpandTilde(m.Source, ec.HomeDir)
		extraMounts = append(extraMounts, docker.Mount{
			Source:   source,
			Target:   m.Target,
			ReadOnly: m.IsReadOnly(),
			Options:  m.Options,
		})
	}
	return extraMounts
}

func (s *DockerStage) setupContainerFeatures(ec *pipeline.ExecutionContext) (sshAuthSock, containerSockPath string) {
	sshAgentFwd := ec.Profile.EffectiveSSHAgentForwarding()
	if sshAgentFwd && !ec.Profile.EffectiveMountSSH() {
		agent, err := sshagent.Setup(ec.Profile.EffectiveContainerRuntime(), ec.ContainerName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: ssh_agent_forwarding: %v\n", err)
		} else {
			sshAuthSock = agent.SocketPath
			ec.SSHAgentReady = true
			ec.SSHAgentCleanup = agent.Cleanup
			ec.SSHReaperInfo = agent
		}
	}

	if ec.Profile.EffectiveMountContainerSock() {
		sockPath, err := mount.DetectContainerSock(ec.Profile.EffectiveContainerRuntime())
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: mount_container_sock: %v\n", err)
		} else {
			containerSockPath = sockPath
			ec.ContainerSockReady = true
			fmt.Fprintf(os.Stderr, "Warning: mount_container_sock is enabled — the AI agent has full access to the container runtime\n")
		}
	}

	if ec.Profile.EffectiveGhToken() {
		token, err := DetectGhToken()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: gh_token: %v\n", err)
		} else {
			ec.GhTokenValue = token
		}
	}

	return sshAuthSock, containerSockPath
}

func toolSyncSpec(tool string, p profile.Profile) *config.ToolSyncSpec {
	credStore, seedHost := p.CodexSyncOptions()
	return config.ToolSyncSpecFor(tool, credStore, seedHost)
}
