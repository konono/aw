package docker

import (
	"context"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"slices"
)

// Mount represents a Docker mount (bind mount or named volume).
type Mount struct {
	Source   string
	Target   string
	ReadOnly bool
	IsVolume bool // true = named volume, false = bind mount
}

// RunConfig holds the configuration for running a Docker container.
type RunConfig struct {
	ImageName string
	Mounts    []Mount
	EnvVars   map[string]string
	WorkDir   string
	Command   []string
}

// Client is the interface for Docker operations.
type Client interface {
	CheckAvailable() error
	Build(ctx context.Context, imageName, contextDir, dockerfilePath string, buildArgs map[string]string) error
	VolumeCreate(ctx context.Context, volumeName string) error
	Run(ctx context.Context, config RunConfig) error
}

// ShellClient implements Client by shelling out to the docker CLI.
type ShellClient struct {
	// DockerPath is the path to the docker binary. Defaults to "docker".
	DockerPath string
}

// NewShellClient creates a new ShellClient for the given container runtime.
// If runtime is empty, it defaults to "docker".
func NewShellClient(runtime string) *ShellClient {
	if runtime == "" {
		runtime = "docker"
	}
	return &ShellClient{DockerPath: runtime}
}

func (c *ShellClient) dockerCmd() string {
	if c.DockerPath != "" {
		return c.DockerPath
	}
	return "docker"
}

// CheckAvailable verifies that the container runtime is installed and running.
func (c *ShellClient) CheckAvailable() error {
	if _, err := exec.LookPath(c.dockerCmd()); err != nil {
		return fmt.Errorf("%s is not installed or not in PATH", c.dockerCmd())
	}

	cmd := exec.Command(c.dockerCmd(), "info")
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s daemon is not running", c.dockerCmd())
	}
	return nil
}

// Build builds a Docker image from the given build context directory.
// If dockerfilePath is non-empty, it is passed via -f to specify the Dockerfile.
// buildArgs are passed as --build-arg KEY=VALUE.
func (c *ShellClient) Build(ctx context.Context, imageName, contextDir, dockerfilePath string, buildArgs map[string]string) error {
	args := []string{"build", "-t", imageName}
	if dockerfilePath != "" {
		args = append(args, "-f", dockerfilePath)
	}
	for _, k := range slices.Sorted(maps.Keys(buildArgs)) {
		args = append(args, "--build-arg", fmt.Sprintf("%s=%s", k, buildArgs[k]))
	}
	args = append(args, contextDir)
	cmd := exec.CommandContext(ctx, c.dockerCmd(), args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// VolumeCreate creates a named container volume (idempotent).
func (c *ShellClient) VolumeCreate(ctx context.Context, volumeName string) error {
	args := []string{"volume", "create"}
	switch c.DockerPath {
	case "podman":
		args = append(args, "--ignore", volumeName)
	default:
		args = append(args, volumeName)
	}
	cmd := exec.CommandContext(ctx, c.dockerCmd(), args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

// BuildRunArgs constructs the docker CLI arguments for a RunConfig.
// This is exported for testing.
func BuildRunArgs(config RunConfig) []string {
	args := []string{"run", "-it", "--rm"}

	for key, val := range config.EnvVars {
		args = append(args, "-e", fmt.Sprintf("%s=%s", key, val))
	}

	for _, m := range config.Mounts {
		mountArg := fmt.Sprintf("%s:%s", m.Source, m.Target)
		if m.ReadOnly {
			mountArg += ":ro"
		}
		args = append(args, "-v", mountArg)
	}

	if config.WorkDir != "" {
		args = append(args, "--workdir", config.WorkDir)
	}

	args = append(args, config.ImageName)
	args = append(args, config.Command...)

	return args
}

// Run runs a Docker container interactively with the given RunConfig.
func (c *ShellClient) Run(ctx context.Context, config RunConfig) error {
	args := BuildRunArgs(config)
	cmd := exec.CommandContext(ctx, c.dockerCmd(), args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
