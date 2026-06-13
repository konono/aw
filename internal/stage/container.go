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

	if ec.Profile.EffectiveGhToken() {
		token, err := detectGhToken()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: gh_token: %v\n", err)
		} else {
			ec.GhTokenValue = token
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
	pkgMgr := ec.Profile.EffectivePackageManager()

	buildDir, cleanup, err := image.PrepareBuildContext(customDockerfile, osTemplate, pkgMgr, cenv)
	if err != nil {
		return "", fmt.Errorf("preparing build context: %w", err)
	}
	defer cleanup()

	userMiseToml := filepath.Join(ec.HomeDir, ".config", "aw", "mise.toml")

	if customDockerfile == "" {
		if pkgMgr == profile.PackageManagerDevbox {
			userDevboxJSON := filepath.Join(ec.HomeDir, ".config", "aw", "devbox.json")
			if data, err := os.ReadFile(userDevboxJSON); err == nil {
				if err := os.WriteFile(filepath.Join(buildDir, "devbox.json"), data, 0644); err != nil {
					return "", fmt.Errorf("copying user devbox.json to build context: %w", err)
				}
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
	if epBytes, err := os.ReadFile(filepath.Join(buildDir, "entrypoint.sh")); err == nil {
		hashInput += "\n" + string(epBytes)
	}
	hashInput += "\n" + string(osTemplate)
	hashInput += "\n" + cenv.User

	toolPkg := ""
	toolInstallScript := ""
	if customDockerfile == "" {
		if pkgMgr == profile.PackageManagerDevbox {
			toolPkg = toolinfo.DevboxPkg(tool)
		} else {
			toolInstallScript = toolinfo.InstallScript(tool)
		}
		hashInput += "\n" + toolPkg
		hashInput += "\n" + toolInstallScript
		hashInput += "\n" + string(pkgMgr)
		if miseData, err := os.ReadFile(userMiseToml); err == nil {
			hashInput += "\n" + string(miseData)
		}
		if pkgMgr == profile.PackageManagerDevbox {
			if devboxData, err := os.ReadFile(filepath.Join(ec.HomeDir, ".config", "aw", "devbox.json")); err == nil {
				hashInput += "\n" + string(devboxData)
			}
		}
	}

	ghInstallScript := ""
	if ec.Profile.EffectiveGhToken() && customDockerfile == "" {
		ghInstallScript = ghCLIInstallScript()
		hashInput += "\n" + ghInstallScript
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
	if toolInstallScript != "" {
		buildArgs["AW_TOOL_INSTALL_SCRIPT"] = toolInstallScript
	}
	if ghInstallScript != "" {
		buildArgs["AW_GH_INSTALL_SCRIPT"] = ghInstallScript
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


const containerContextMarker = "\n# aw Container Environment\n"

func appendContainerContext(toolStageDir string, ec *pipeline.ExecutionContext) error {
	if toolStageDir == "" {
		return nil
	}
	claudeMD := filepath.Join(toolStageDir, "CLAUDE.md")

	var sections []string

	if ec.Profile.EffectivePackageManager() == profile.PackageManagerDevbox {
		sections = append(sections, `## Package Managers

- npm: Node.js package manager. Use "npm install -g <pkg>" to install global packages
- mise: polyglot runtime manager. Use "mise install" / "mise use" for language runtimes
- Both are pre-installed and available in PATH`)
	} else {
		sections = append(sections, `## Package Managers

- mise: polyglot runtime manager. Use "mise install" / "mise use" for language runtimes
- Pre-installed and available in PATH`)
	}

	if ec.ContainerSockReady {
		sections = append(sections, `## Docker / Podman (DooD)

Container runtime socket is mounted at /run/container.sock.
Before running docker/docker-compose commands, set: export DOCKER_HOST=unix:///run/container.sock

- Use docker-compose / docker commands directly — do NOT try to install Docker or start a daemon
- Containers created via docker-compose are sibling containers on the host`)
	}

	if ec.Profile.EffectiveGhToken() {
		sections = append(sections, `## GitHub CLI

GITHUB_TOKEN is set. gh commands (gh pr, gh issue, etc.) work directly.
Git HTTPS operations (clone, push, fetch) also work directly via credential helper.`)
	} else if ec.Profile.EffectiveMountGH() {
		sections = append(sections, `## GitHub CLI

Host gh configuration is mounted (read-only). gh commands (gh pr, gh issue, etc.) work directly.`)
	}

	if ec.SSHAgentReady {
		sections = append(sections, `## SSH Agent

SSH agent is forwarded. Git SSH operations (push, clone, fetch) work without additional setup.`)
	}

	suffix := "\n# aw Container Environment\n\nThis session runs inside an aw container.\n\n" +
		strings.Join(sections, "\n\n") + "\n"

	existing, _ := os.ReadFile(claudeMD)
	base := string(existing)
	if idx := strings.Index(base, containerContextMarker); idx >= 0 {
		base = base[:idx]
	}

	return os.WriteFile(claudeMD, []byte(base+suffix), 0644)
}

func detectGhToken() (string, error) {
	out, err := exec.Command("gh", "auth", "token").Output()
	if err != nil {
		return "", fmt.Errorf("failed to get token from 'gh auth token': %w (is gh CLI installed and authenticated?)", err)
	}
	token := strings.TrimSpace(string(out))
	if token == "" {
		return "", fmt.Errorf("'gh auth token' returned empty token")
	}
	return token, nil
}

func ghCLIInstallScript() string {
	return `GH_VER=$(curl -fsSL https://api.github.com/repos/cli/cli/releases/latest | awk -F'"' '/"tag_name"/{print $4}' | sed 's/^v//') && ` +
		`ARCH=$(uname -m); case $ARCH in aarch64) ARCH=arm64;; x86_64) ARCH=amd64;; esac && ` +
		`curl -fsSL "https://github.com/cli/cli/releases/download/v${GH_VER}/gh_${GH_VER}_linux_${ARCH}.tar.gz" | tar xz -C /tmp && ` +
		`mv /tmp/gh_${GH_VER}_linux_${ARCH}/bin/gh /usr/local/bin/gh && ` +
		`rm -rf /tmp/gh_${GH_VER}_linux_${ARCH}`
}

func expandTilde(path, homeDir string) string {
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(homeDir, path[2:])
	}
	return path
}

