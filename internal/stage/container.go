package stage

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/konono/aw/internal/config"
	"github.com/konono/aw/internal/docker"
	"github.com/konono/aw/internal/image"
	"github.com/konono/aw/internal/mount"
	"github.com/konono/aw/internal/pipeline"
	"github.com/konono/aw/internal/profile"
	"github.com/konono/aw/internal/sshagent"
	"github.com/konono/aw/internal/toolinfo"
)

const defaultImageName = "aw-container"

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

	customDockerfile := ""
	if ec.Profile.Dockerfile != "" {
		resolved, err := resolveDockerfilePath(ec.Profile.Dockerfile)
		if err != nil {
			return fmt.Errorf("resolving dockerfile path: %w", err)
		}
		customDockerfile = resolved
	}

	tool := ec.Profile.EffectiveTool()
	osTemplate := ec.Profile.EffectiveOS()

	buildDir, cleanup, err := image.PrepareBuildContext(customDockerfile, osTemplate)
	if err != nil {
		return fmt.Errorf("preparing build context: %w", err)
	}
	defer cleanup()

	// Copy user's global devbox.json into build context if it exists
	userDevboxJSON := filepath.Join(ec.HomeDir, ".config", "aw", "devbox.json")
	if data, err := os.ReadFile(userDevboxJSON); err == nil {
		if err := os.WriteFile(filepath.Join(buildDir, "devbox.json"), data, 0644); err != nil {
			return fmt.Errorf("copying user devbox.json to build context: %w", err)
		}
	}

	// Copy user's global mise.toml into build context if it exists
	userMiseToml := filepath.Join(ec.HomeDir, ".config", "aw", "mise.toml")
	if data, err := os.ReadFile(userMiseToml); err == nil {
		if err := os.WriteFile(filepath.Join(buildDir, "mise.toml"), data, 0644); err != nil {
			return fmt.Errorf("copying user mise.toml to build context: %w", err)
		}
	}

	dockerfilePath := customDockerfile
	hashSource := filepath.Join(buildDir, "Dockerfile")
	if dockerfilePath != "" {
		hashSource = dockerfilePath
	}

	// Include tool name and user devbox.json in image hash
	hashInput := ""
	if dfBytes, err := os.ReadFile(hashSource); err == nil {
		hashInput = string(dfBytes)
	}
	hashInput += "\n" + string(osTemplate)
	toolPkg := toolinfo.DevboxPkg(tool)
	hashInput += "\n" + toolPkg
	if devboxData, err := os.ReadFile(userDevboxJSON); err == nil {
		hashInput += "\n" + string(devboxData)
	}
	if miseData, err := os.ReadFile(userMiseToml); err == nil {
		hashInput += "\n" + string(miseData)
	}

	imageName := defaultImageName
	if hashInput != "" {
		hash := fmt.Sprintf("%x", sha256.Sum256([]byte(hashInput)))[:12]
		imageName = fmt.Sprintf("%s:%s", defaultImageName, hash)
	}

	buildArgs := map[string]string{}
	if toolPkg != "" {
		buildArgs["AW_TOOL_PKG"] = toolPkg
	}

	if customDockerfile != "" {
		fmt.Fprintf(os.Stderr, "Building Docker image '%s' (custom Dockerfile: %s)...\n", imageName, ec.Profile.Dockerfile)
	} else {
		fmt.Fprintf(os.Stderr, "Building Docker image '%s' (os: %s)...\n", imageName, osTemplate)
	}
	if err := s.DockerClient.Build(ctx, imageName, buildDir, dockerfilePath, buildArgs); err != nil {
		return fmt.Errorf("building image: %w", err)
	}

	stageDir := filepath.Join(ec.HomeDir, ".agent-workspace")
	toolStageDir := ""
	toolContainerDir := ""

	spec := toolSyncSpec(tool, ec.Profile)
	if spec != nil {
		srcDir := toolinfo.HomePath(tool, ec.HomeDir)
		toolStageDir = filepath.Join(stageDir, tool)
		toolContainerDir = toolinfo.ContainerDir(tool)
		if err := s.ConfigSyncer.SyncToolSettings(srcDir, toolStageDir, *spec); err != nil {
			return fmt.Errorf("syncing %s settings: %w", tool, err)
		}
	}

	if tool == "claude" && toolStageDir != "" {
		onboardingPath := filepath.Join(toolStageDir, ".claude.json")
		if err := s.ConfigSyncer.EnsureOnboardingState(onboardingPath); err != nil {
			return fmt.Errorf("ensuring onboarding state: %w", err)
		}
	}

	var extraMounts []docker.Mount
	for _, m := range ec.Profile.Mounts {
		source := expandTilde(m.Source, ec.HomeDir)
		extraMounts = append(extraMounts, docker.Mount{
			Source:   source,
			Target:   m.Target,
			ReadOnly: m.IsReadOnly(),
			Options:  m.Options,
		})
	}

	sshAgentFwd := ec.Profile.EffectiveSSHAgentForwarding()
	sshAuthSock := ""
	if sshAgentFwd && !ec.Profile.EffectiveMountSSH() {
		agent, err := sshagent.Setup(ec.Profile.EffectiveContainerRuntime())
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: ssh_agent_forwarding: %v\n", err)
		} else {
			sshAuthSock = agent.SocketPath
			ec.SSHAgentReady = true
			ec.SSHAgentCleanup = agent.Cleanup
		}
	}

	mounts, err := s.MountBuilder.BuildMounts(mount.MountOptions{
		HomeDir:            ec.HomeDir,
		WorkDir:            ec.WorkDir,
		ToolStageDir:       toolStageDir,
		ToolContainerDir:   toolContainerDir,
		MountGH:            ec.Profile.EffectiveMountGH(),
		MountSSH:           ec.Profile.EffectiveMountSSH(),
		SSHAgentForwarding: sshAgentFwd,
		SSHAuthSock:        sshAuthSock,
		ExtraMounts:        extraMounts,
	})
	if err != nil {
		return fmt.Errorf("building mounts: %w", err)
	}

	ec.DockerImage = imageName
	ec.DockerMounts = mounts

	return nil
}

func toolSyncSpec(tool string, p profile.Profile) *config.ToolSyncSpec {
	switch tool {
	case "claude":
		return &config.ClaudeSyncSpec
	case "codex":
		credentialsStore := "file"
		seedFromHost := "if_missing"
		if p.Auth != nil && p.Auth.Codex != nil {
			if p.Auth.Codex.CredentialsStore != "" {
				credentialsStore = string(p.Auth.Codex.CredentialsStore)
			}
			if p.Auth.Codex.SeedFromHost != "" {
				seedFromHost = string(p.Auth.Codex.SeedFromHost)
			}
		}
		spec := config.CodexSyncSpecWithOptions(credentialsStore, seedFromHost)
		return &spec
	case "opencode":
		return &config.OpenCodeSyncSpec
	default:
		return nil
	}
}

func resolveDockerfilePath(dockerfilePath string) (string, error) {
	if filepath.IsAbs(dockerfilePath) {
		return dockerfilePath, nil
	}

	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("finding git root to resolve dockerfile path: %w", err)
	}
	repoRoot := strings.TrimSpace(string(out))
	return filepath.Join(repoRoot, dockerfilePath), nil
}

func expandTilde(path, homeDir string) string {
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(homeDir, path[2:])
	}
	return path
}
