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

const (
	defaultImageName  = "claude-code-docker"
	defaultVolumeName = "claude-code-local"
)

// DockerStage builds the Docker image, creates volumes, syncs config, and builds mounts.
type DockerStage struct {
	DockerClient docker.Client
	ConfigSyncer config.Syncer
	MountBuilder mount.Builder
}

// NewDockerStage creates a DockerStage with default implementations.
// DockerClient is initialized lazily in Run() using the profile's container_runtime.
func NewDockerStage() *DockerStage {
	return &DockerStage{
		ConfigSyncer: config.NewSyncer(),
		MountBuilder: mount.NewBuilder(),
	}
}

func (s *DockerStage) Name() string { return "docker" }

func (s *DockerStage) Run(ctx context.Context, ec *pipeline.ExecutionContext) error {
	// 0. Initialize docker client with the configured container runtime
	if s.DockerClient == nil {
		s.DockerClient = docker.NewShellClient(ec.Profile.EffectiveContainerRuntime())
	}

	// 1. Check container runtime availability
	if err := s.DockerClient.CheckAvailable(); err != nil {
		return fmt.Errorf("container runtime is not available: %w", err)
	}

	// 2. Resolve custom Dockerfile path
	customDockerfile := ""
	if ec.Profile.Dockerfile != "" {
		resolved, err := resolveDockerfilePath(ec.Profile.Dockerfile)
		if err != nil {
			return fmt.Errorf("resolving dockerfile path: %w", err)
		}
		customDockerfile = resolved
	}

	// 3. Build Docker image
	buildDir, cleanup, err := image.PrepareBuildContext(customDockerfile)
	if err != nil {
		return fmt.Errorf("preparing build context: %w", err)
	}
	defer cleanup()

	// Compute image tag from Dockerfile content hash to bust Docker cache
	// when the Dockerfile changes.
	dockerfilePath := customDockerfile
	hashSource := filepath.Join(buildDir, "Dockerfile")
	if dockerfilePath != "" {
		hashSource = dockerfilePath
	}

	imageName := defaultImageName
	if dfBytes, err := os.ReadFile(hashSource); err == nil {
		hash := fmt.Sprintf("%x", sha256.Sum256(dfBytes))[:12]
		imageName = fmt.Sprintf("%s:%s", defaultImageName, hash)
	}

	if customDockerfile != "" {
		fmt.Fprintf(os.Stderr, "Building Docker image '%s' (custom Dockerfile: %s)...\n", imageName, ec.Profile.Dockerfile)
	} else {
		fmt.Fprintf(os.Stderr, "Building Docker image '%s'...\n", imageName)
	}
	if err := s.DockerClient.Build(ctx, imageName, buildDir, dockerfilePath); err != nil {
		return fmt.Errorf("building image: %w", err)
	}

	// 3. Create Docker volume
	if err := s.DockerClient.VolumeCreate(ctx, defaultVolumeName); err != nil {
		return fmt.Errorf("creating volume: %w", err)
	}

	// 4. Sync host settings
	claudeHome := claudeHomePath(ec.HomeDir)
	containerClaudeHome := filepath.Join(ec.HomeDir, ".agent-workspace")
	containerClaudeJSON := filepath.Join(ec.HomeDir, ".agent-workspace.json")

	if err := s.ConfigSyncer.SyncSettings(claudeHome, containerClaudeHome); err != nil {
		return fmt.Errorf("syncing settings: %w", err)
	}

	// 5. Ensure onboarding state
	if err := s.ConfigSyncer.EnsureOnboardingState(containerClaudeJSON); err != nil {
		return fmt.Errorf("ensuring onboarding state: %w", err)
	}

	// 6. Build mounts (including custom mounts from profile)
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
		HomeDir:             ec.HomeDir,
		WorkDir:             ec.WorkDir,
		ClaudeHome:          claudeHome,
		ContainerClaudeHome: containerClaudeHome,
		ContainerClaudeJSON: containerClaudeJSON,
		VolumeName:          defaultVolumeName,
		ExtraMounts:         extraMounts,
	})
	if err != nil {
		return fmt.Errorf("building mounts: %w", err)
	}

	// 7. Update execution context
	ec.DockerImage = imageName
	ec.DockerMounts = mounts
	ec.DockerVolume = defaultVolumeName

	return nil
}

// resolveDockerfilePath resolves a Dockerfile path.
// If the path is absolute, it is returned as-is.
// If relative, it is resolved against the git repo root.
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

func claudeHomePath(homeDir string) string {
	if v := os.Getenv("CLAUDE_HOME"); v != "" {
		return v
	}
	return filepath.Join(homeDir, ".claude")
}
