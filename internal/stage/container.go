package stage

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/konono/aw/internal/config"
	"github.com/konono/aw/internal/containerenv"
	"github.com/konono/aw/internal/docker"
	"github.com/konono/aw/internal/gitroot"
	"github.com/konono/aw/internal/image"
	"github.com/konono/aw/internal/mount"
	"github.com/konono/aw/internal/pathutil"
	"github.com/konono/aw/internal/pipeline"
	"github.com/konono/aw/internal/platform"
	"github.com/konono/aw/internal/profile"
	"github.com/konono/aw/internal/sshagent"
	"github.com/konono/aw/internal/toolinfo"
	"github.com/konono/aw/internal/version"
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
	ConfigDir    string // override for platform.ConfigDir(); empty = default
}

func (s *DockerStage) configDir() string {
	if s.ConfigDir != "" {
		return s.ConfigDir
	}
	return platform.ConfigDir()
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

	if ec.Profile.Dockerfile == "" && !HasBuildCustomizations(ec, s.configDir()) {
		if _, err := os.Stat(filepath.Join(s.configDir(), "mise.toml")); err == nil {
			fmt.Fprintln(os.Stderr, "Warning: ~/.config/aw/mise.toml found but ignored by official image.")
			fmt.Fprintln(os.Stderr, "  To use mise tools, set image_pull_policy: build or place mise.toml in your workspace.")
		}
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
func HasBuildCustomizations(ec *pipeline.ExecutionContext, configDir string) bool {
	p := ec.Profile
	if len(p.Packages) > 0 || len(p.BuildEnv) > 0 || p.CACert != "" ||
		p.PackageManager == profile.PackageManagerDevbox || p.EffectiveGhToken() ||
		(p.ContainerUser != "" && p.ContainerUser != "agent") {
		return true
	}
	if pkgs := pipeline.CollectPackages(configDir, nil); len(pkgs) > 0 {
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

	tool := ec.Profile.EffectiveTool()
	if tool == "" {
		return "", nil
	}

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

type buildInputs struct {
	toolPkg           string
	toolInstallScript string
	ghInstallScript   string
	extraPackages     string
}

func resolveBuildInputs(customDockerfile string, tool string, pkgMgr profile.PackageManager, ec *pipeline.ExecutionContext) buildInputs {
	var bi buildInputs
	if customDockerfile != "" {
		return bi
	}
	if pkgMgr == profile.PackageManagerDevbox {
		bi.toolPkg = toolinfo.DevboxPkg(tool)
	} else {
		bi.toolInstallScript = toolinfo.InstallScript(tool)
	}
	if ec.Profile.EffectiveGhToken() {
		bi.ghInstallScript = ghCLIInstallScript()
	}
	packages := pipeline.CollectPackages(platform.ConfigDir(), ec.Profile.Packages)
	if len(packages) > 0 {
		bi.extraPackages = strings.Join(packages, " ")
	}
	return bi
}

func computeImageTag(buildDir, dockerfilePath, customDockerfile string, ec *pipeline.ExecutionContext, cenv containerenv.Config, pkgMgr profile.PackageManager, bi buildInputs) string {
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
	if initBytes, err := os.ReadFile(filepath.Join(buildDir, "aw-init.sh")); err == nil {
		hashInput += "\n" + string(initBytes)
	}
	hashInput += "\n" + string(ec.Profile.EffectiveOS())
	hashInput += "\n" + cenv.User

	if customDockerfile == "" {
		userMiseToml := filepath.Join(platform.ConfigDir(), "mise.toml")
		hashInput += "\n" + bi.toolPkg
		hashInput += "\n" + bi.toolInstallScript
		hashInput += "\n" + string(pkgMgr)
		if miseData, err := os.ReadFile(userMiseToml); err == nil {
			hashInput += "\n" + string(miseData)
		}
		if pkgMgr == profile.PackageManagerDevbox {
			if devboxData, err := os.ReadFile(filepath.Join(platform.ConfigDir(), "devbox.json")); err == nil {
				hashInput += "\n" + string(devboxData)
			}
		}
	}

	if bi.ghInstallScript != "" {
		hashInput += "\n" + bi.ghInstallScript
	}
	if bi.extraPackages != "" {
		hashInput += "\n" + bi.extraPackages
	}

	if ec.Profile.CACert != "" {
		certPath := pathutil.ExpandTilde(ec.Profile.CACert, ec.HomeDir)
		if certData, err := os.ReadFile(certPath); err == nil {
			hashInput += "\n" + string(certData)
		}
	}
	for _, k := range slices.Sorted(maps.Keys(ec.Profile.BuildEnv)) {
		hashInput += "\n" + k + "=" + ec.Profile.BuildEnv[k]
	}

	if hashInput != "" {
		hash := fmt.Sprintf("%x", sha256.Sum256([]byte(hashInput)))[:12]
		return fmt.Sprintf("%s:%s", defaultImageName, hash)
	}
	return defaultImageName
}

func collectBuildArgs(customDockerfile string, ec *pipeline.ExecutionContext, bi buildInputs) map[string]string {
	buildArgs := map[string]string{}
	if bi.toolPkg != "" {
		buildArgs["AW_TOOL_PKG"] = bi.toolPkg
	}
	if bi.toolInstallScript != "" {
		buildArgs["AW_TOOL_INSTALL_SCRIPT"] = bi.toolInstallScript
	}
	if bi.ghInstallScript != "" {
		buildArgs["AW_GH_INSTALL_SCRIPT"] = bi.ghInstallScript
	}
	if bi.extraPackages != "" && customDockerfile == "" {
		buildArgs["AW_EXTRA_PACKAGES"] = bi.extraPackages
	}
	for k, v := range ec.Profile.BuildEnv {
		buildArgs[k] = v
	}
	return buildArgs
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

	if err := copyUserConfigs(buildDir, customDockerfile, pkgMgr); err != nil {
		return "", err
	}

	caCertInBuildDir, err := copyCACert(buildDir, customDockerfile, ec)
	if err != nil {
		return "", err
	}
	if caCertInBuildDir != "" {
		defer func() { _ = os.Remove(caCertInBuildDir) }()
	}

	dockerfilePath := customDockerfile
	bi := resolveBuildInputs(customDockerfile, tool, pkgMgr, ec)
	imageName := computeImageTag(buildDir, dockerfilePath, customDockerfile, ec, cenv, pkgMgr, bi)
	buildArgs := collectBuildArgs(customDockerfile, ec, bi)

	if customDockerfile != "" {
		fmt.Fprintf(os.Stderr, "Building Docker image '%s' (custom Dockerfile: %s)...\n", imageName, ec.Profile.Dockerfile)
	} else {
		fmt.Fprintf(os.Stderr, "Building Docker image '%s' (os: %s)...\n", imageName, osTemplate)
	}
	if err := s.DockerClient.Build(ctx, imageName, buildDir, dockerfilePath, buildArgs, ec.NoCache); err != nil {
		return "", fmt.Errorf("building image: %w", err)
	}

	return imageName, nil
}

func copyUserConfigs(buildDir, customDockerfile string, pkgMgr profile.PackageManager) error {
	if customDockerfile != "" {
		return nil
	}
	userMiseToml := filepath.Join(platform.ConfigDir(), "mise.toml")
	if pkgMgr == profile.PackageManagerDevbox {
		userDevboxJSON := filepath.Join(platform.ConfigDir(), "devbox.json")
		if data, err := os.ReadFile(userDevboxJSON); err == nil {
			if err := os.WriteFile(filepath.Join(buildDir, "devbox.json"), data, 0644); err != nil {
				return fmt.Errorf("copying user devbox.json to build context: %w", err)
			}
		}
	}
	if data, err := os.ReadFile(userMiseToml); err == nil {
		if err := os.WriteFile(filepath.Join(buildDir, "mise.toml"), data, 0644); err != nil {
			return fmt.Errorf("copying user mise.toml to build context: %w", err)
		}
	}
	return nil
}

func copyCACert(buildDir, customDockerfile string, ec *pipeline.ExecutionContext) (caCertInBuildDir string, err error) {
	if ec.Profile.CACert == "" {
		return "", nil
	}
	certPath := pathutil.ExpandTilde(ec.Profile.CACert, ec.HomeDir)
	data, err := os.ReadFile(certPath)
	if err != nil {
		return "", fmt.Errorf("reading ca_cert %q: %w", ec.Profile.CACert, err)
	}
	dst := filepath.Join(buildDir, "ca-cert.pem")
	if err := os.WriteFile(dst, data, 0644); err != nil {
		return "", fmt.Errorf("copying ca_cert to build context: %w", err)
	}
	if customDockerfile != "" {
		return dst, nil
	}
	return "", nil
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
	case "cursor":
		return &config.CursorSyncSpec
	default:
		return nil
	}
}

func resolveDockerfilePath(dockerfilePath string) (string, error) {
	if filepath.IsAbs(dockerfilePath) {
		return dockerfilePath, nil
	}

	repoRoot, err := gitroot.FindRoot()
	if err != nil {
		return "", fmt.Errorf("finding git root to resolve dockerfile path: %w", err)
	}
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
		if ec.Profile.Image != "" {
			sections = append(sections, `## GitHub Token

GITHUB_TOKEN is set. Git HTTPS operations (clone, push, fetch) to github.com work directly via credential helper.
Note: gh CLI may not be available in this pre-built image.`)
		} else {
			sections = append(sections, `## GitHub CLI

GITHUB_TOKEN is set. gh commands (gh pr, gh issue, etc.) work directly.
Git HTTPS operations (clone, push, fetch) to github.com also work directly via credential helper.`)
		}
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

func DetectGhToken() (string, error) {
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

type cursorAuth struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
}

// seedCursorAuth populates auth.json in the staging directory if missing.
//
// Token sources tried in order:
//  1. macOS Keychain (cursor-access-token / cursor-refresh-token)
//  2. ~/.config/cursor/auth.json (Linux file-based storage, also works
//     as a macOS fallback when Keychain is unavailable)
func seedCursorAuth(stageDir, homeDir string) error {
	authPath := filepath.Join(stageDir, "auth.json")
	if _, err := os.Stat(authPath); err == nil {
		return nil
	}

	if data := cursorAuthFromKeychain(); data != nil {
		if err := os.MkdirAll(stageDir, 0755); err != nil {
			return err
		}
		return os.WriteFile(authPath, data, 0600)
	}

	if data := cursorAuthFromFile(homeDir); data != nil {
		if err := os.MkdirAll(stageDir, 0755); err != nil {
			return err
		}
		return os.WriteFile(authPath, data, 0600)
	}

	return nil
}

func cursorAuthFromKeychain() []byte {
	if runtime.GOOS != "darwin" {
		return nil
	}

	access, err := readKeychainPassword("cursor-access-token", "cursor-user")
	if err != nil || access == "" {
		return nil
	}
	refresh, err := readKeychainPassword("cursor-refresh-token", "cursor-user")
	if err != nil || refresh == "" {
		return nil
	}

	data, err := json.MarshalIndent(cursorAuth{
		AccessToken:  access,
		RefreshToken: refresh,
	}, "", "  ")
	if err != nil {
		return nil
	}
	return append(data, '\n')
}

func cursorAuthFromFile(homeDir string) []byte {
	path := filepath.Join(homeDir, ".config", "cursor", "auth.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var auth cursorAuth
	if err := json.Unmarshal(data, &auth); err != nil {
		return nil
	}
	if auth.AccessToken == "" {
		return nil
	}
	return data
}

func readKeychainPassword(service, account string) (string, error) {
	out, err := exec.Command("security", "find-generic-password",
		"-s", service, "-a", account, "-w").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func ghCLIInstallScript() string {
	return `GH_VER=$(curl -fsSL https://api.github.com/repos/cli/cli/releases/latest | awk -F'"' '/"tag_name"/{print $4}' | sed 's/^v//') && ` +
		`ARCH=$(uname -m); case $ARCH in aarch64) ARCH=arm64;; x86_64) ARCH=amd64;; esac && ` +
		`curl -fsSL "https://github.com/cli/cli/releases/download/v${GH_VER}/gh_${GH_VER}_linux_${ARCH}.tar.gz" | tar xz -C /tmp && ` +
		`mv /tmp/gh_${GH_VER}_linux_${ARCH}/bin/gh /usr/local/bin/gh && ` +
		`rm -rf /tmp/gh_${GH_VER}_linux_${ARCH}`
}

