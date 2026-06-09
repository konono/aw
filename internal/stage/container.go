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
	"github.com/konono/aw/internal/containerenv"
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

	cenv := containerenv.FromUser(ec.Profile.EffectiveContainerUser())
	ec.ContainerEnv = cenv

	var imageName string
	var err error

	if ec.Profile.Image != "" {
		imageName = ec.Profile.Image
		exists, eerr := s.DockerClient.ImageExists(ctx, imageName)
		if eerr != nil {
			return fmt.Errorf("checking image %q: %w", imageName, eerr)
		}
		if !exists {
			return fmt.Errorf("image %q not found; load it via '%s load'",
				imageName, ec.Profile.EffectiveContainerRuntime())
		}
		fmt.Fprintf(os.Stderr, "Using pre-built image '%s'...\n", imageName)
	} else {
		imageName, err = s.buildImage(ctx, ec, cenv)
		if err != nil {
			return err
		}
	}

	tool := ec.Profile.EffectiveTool()
	stageDir := filepath.Join(ec.HomeDir, ".agent-workspace")
	toolStageDir := ""
	toolContainerDir := ""

	spec := toolSyncSpec(tool, ec.Profile)
	if spec != nil {
		srcDir := toolinfo.HomePath(tool, ec.HomeDir)
		toolStageDir = filepath.Join(stageDir, tool)
		toolContainerDir = toolinfo.ContainerDirFor(tool, cenv)
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

	containerSockPath := ""
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

	if err := appendContainerContext(toolStageDir, ec); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: appending container context: %v\n", err)
	}

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

	ec.DockerImage = imageName
	ec.DockerMounts = mounts
	if ec.ContainerSockReady {
		ec.DockerSecurityOpts = append(ec.DockerSecurityOpts, "label=disable")
	}

	return nil
}

func (s *DockerStage) buildImage(ctx context.Context, ec *pipeline.ExecutionContext, cenv containerenv.Config) (string, error) {
	customDockerfile := ""
	if ec.Profile.Dockerfile != "" {
		resolved, err := resolveDockerfilePath(ec.Profile.Dockerfile)
		if err != nil {
			return "", fmt.Errorf("resolving dockerfile path: %w", err)
		}
		customDockerfile = resolved
	}

	tool := ec.Profile.EffectiveTool()
	osTemplate := ec.Profile.EffectiveOS()

	buildDir, cleanup, err := image.PrepareBuildContext(customDockerfile, osTemplate, cenv)
	if err != nil {
		return "", fmt.Errorf("preparing build context: %w", err)
	}
	defer cleanup()

	userDevboxJSON := filepath.Join(ec.HomeDir, ".config", "aw", "devbox.json")
	userMiseToml := filepath.Join(ec.HomeDir, ".config", "aw", "mise.toml")

	if customDockerfile == "" {
		if data, err := os.ReadFile(userDevboxJSON); err == nil {
			if err := os.WriteFile(filepath.Join(buildDir, "devbox.json"), data, 0644); err != nil {
				return "", fmt.Errorf("copying user devbox.json to build context: %w", err)
			}
		}

		if data, err := os.ReadFile(userMiseToml); err == nil {
			if err := os.WriteFile(filepath.Join(buildDir, "mise.toml"), data, 0644); err != nil {
				return "", fmt.Errorf("copying user mise.toml to build context: %w", err)
			}
		}
	}

	dockerfilePath := customDockerfile
	hashSource := filepath.Join(buildDir, "Dockerfile")
	if dockerfilePath != "" {
		hashSource = dockerfilePath
	}

	hashInput := ""
	if dfBytes, err := os.ReadFile(hashSource); err == nil {
		hashInput = string(dfBytes)
	}
	hashInput += "\n" + string(osTemplate)
	hashInput += "\n" + cenv.User

	toolPkg := ""
	if customDockerfile == "" {
		toolPkg = toolinfo.DevboxPkg(tool)
		hashInput += "\n" + toolPkg
		if devboxData, err := os.ReadFile(userDevboxJSON); err == nil {
			hashInput += "\n" + string(devboxData)
		}
		if miseData, err := os.ReadFile(userMiseToml); err == nil {
			hashInput += "\n" + string(miseData)
		}
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
		return "", fmt.Errorf("building image: %w", err)
	}

	return imageName, nil
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


func appendContainerContext(toolStageDir string, ec *pipeline.ExecutionContext) error {
	if toolStageDir == "" {
		return nil
	}
	claudeMD := filepath.Join(toolStageDir, "CLAUDE.md")

	var sections []string

	sections = append(sections, `## Package Managers

- devbox: Nix-based package manager. Use "devbox global add <pkg>" to install packages
- mise: polyglot runtime manager. Use "mise install" / "mise use" for language runtimes
- Both are pre-installed and available in PATH`)

	if ec.ContainerSockReady {
		sections = append(sections, `## Docker / Podman (DooD)

Container runtime socket is mounted at /run/container.sock.
DOCKER_HOST is pre-configured. docker and docker-compose are available in PATH via devbox.

- Use docker-compose / docker commands directly — do NOT try to install Docker or start a daemon
- Containers created via docker-compose are sibling containers on the host`)
	}

	if ec.Profile.EffectiveMountGH() {
		sections = append(sections, `## GitHub CLI

Host gh configuration is mounted (read-only). gh commands (gh pr, gh issue, etc.) work directly.`)
	}

	if ec.SSHAgentReady {
		sections = append(sections, `## SSH Agent

SSH agent is forwarded. Git SSH operations (push, clone, fetch) work without additional setup.`)
	}

	content := "\n\n# aw Container Environment\n\nThis session runs inside an aw container.\n\n" +
		strings.Join(sections, "\n\n") + "\n"

	f, err := os.OpenFile(claudeMD, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	_, err = f.WriteString(content)
	return err
}

func expandTilde(path, homeDir string) string {
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(homeDir, path[2:])
	}
	return path
}
