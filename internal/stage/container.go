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

	buildDir, cleanup, err := image.PrepareBuildContext(customDockerfile)
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
	toolPkg := toolDevboxPkg(tool)
	hashInput += "\n" + toolPkg
	if devboxData, err := os.ReadFile(userDevboxJSON); err == nil {
		hashInput += "\n" + string(devboxData)
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
		fmt.Fprintf(os.Stderr, "Building Docker image '%s'...\n", imageName)
	}
	if err := s.DockerClient.Build(ctx, imageName, buildDir, dockerfilePath, buildArgs); err != nil {
		return fmt.Errorf("building image: %w", err)
	}

	stageDir := filepath.Join(ec.HomeDir, ".agent-workspace")
	toolStageDir := ""
	toolContainerDir := ""

	spec, containerDir := toolSyncConfig(tool)
	if spec != nil {
		srcDir := toolHomePath(tool, ec.HomeDir)
		toolStageDir = filepath.Join(stageDir, tool)
		toolContainerDir = containerDir
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
			ReadOnly: m.ReadOnly,
		})
	}

	mounts, err := s.MountBuilder.BuildMounts(mount.MountOptions{
		HomeDir:          ec.HomeDir,
		WorkDir:          ec.WorkDir,
		ToolStageDir:     toolStageDir,
		ToolContainerDir: toolContainerDir,
		ExtraMounts:      extraMounts,
	})
	if err != nil {
		return fmt.Errorf("building mounts: %w", err)
	}

	ec.DockerImage = imageName
	ec.DockerMounts = mounts

	return nil
}

func toolSyncConfig(tool string) (*config.ToolSyncSpec, string) {
	switch tool {
	case "claude":
		return &config.ClaudeSyncSpec, "/home/agent/.claude"
	case "codex":
		return &config.CodexSyncSpec, "/home/agent/.codex"
	case "opencode":
		return &config.OpenCodeSyncSpec, "/home/agent/.config/opencode"
	default:
		return nil, ""
	}
}

func toolDevboxPkg(tool string) string {
	switch tool {
	case "claude":
		return "claude-code"
	case "codex":
		return "codex"
	case "opencode":
		return "opencode"
	default:
		return ""
	}
}

func toolHomePath(tool, homeDir string) string {
	switch tool {
	case "claude":
		if v := os.Getenv("CLAUDE_HOME"); v != "" {
			return v
		}
		return filepath.Join(homeDir, ".claude")
	case "codex":
		if v := os.Getenv("CODEX_HOME"); v != "" {
			return v
		}
		return filepath.Join(homeDir, ".codex")
	case "opencode":
		if v := os.Getenv("OPENCODE_CONFIG_DIR"); v != "" {
			return v
		}
		return filepath.Join(homeDir, ".config", "opencode")
	default:
		return ""
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
