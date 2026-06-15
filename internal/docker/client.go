package docker

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"os/signal"
	"slices"
	"strings"
	"syscall"
)

// Mount represents a Docker mount (bind mount or named volume).
type Mount struct {
	Source   string
	Target   string
	ReadOnly bool
	IsVolume bool   // true = named volume, false = bind mount
	Options  string // extra mount options (e.g. "z", "Z,nocopy")
}

// RunConfig holds the configuration for running a Docker container.
type RunConfig struct {
	ImageName    string
	Mounts       []Mount
	EnvVars      map[string]string
	WorkDir      string
	Command      []string
	Entrypoint   string   // overrides the image's ENTRYPOINT when non-empty
	SecurityOpts []string // --security-opt values (e.g. "label=disable")
	CapAdd       []string // --cap-add values (e.g. "AUDIT_WRITE")
	User         string   // overrides the image's USER (e.g. "0:0" for root)
	Userns       string   // --userns value (e.g. "keep-id" for rootless Podman)
}

// Client is the interface for Docker operations.
type Client interface {
	CheckAvailable() error
	Build(ctx context.Context, imageName, contextDir, dockerfilePath string, buildArgs map[string]string) error
	ImageExists(ctx context.Context, imageName string) (bool, error)
	Save(ctx context.Context, imageName, outputPath string) error
	Run(ctx context.Context, config RunConfig) error
	RunOneShot(ctx context.Context, config RunConfig) (containerID string, err error)
	Commit(ctx context.Context, containerID, imageName string, changes []string) error
	RemoveContainer(ctx context.Context, containerID string) error
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
	args := []string{"build", "--load", "-t", imageName}
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

// ImageExists checks whether an image exists locally.
func (c *ShellClient) ImageExists(ctx context.Context, imageName string) (bool, error) {
	cmd := exec.CommandContext(ctx, c.dockerCmd(), "image", "inspect", imageName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if isImageInspectNotFound(output) {
			return false, nil
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			msg := strings.TrimSpace(string(output))
			if msg == "" {
				msg = exitErr.Error()
			}
			return false, fmt.Errorf("%s image inspect %q failed: %s", c.dockerCmd(), imageName, msg)
		}
		return false, err
	}
	return true, nil
}

func isImageInspectNotFound(output []byte) bool {
	msg := strings.ToLower(strings.TrimSpace(string(output)))
	for _, marker := range []string{
		"no such image",
		"no such object",
		"image not known",
		"image unknown",
		"unable to find image",
	} {
		if strings.Contains(msg, marker) {
			return true
		}
	}
	return false
}

// Save exports a container image to a tar archive.
func (c *ShellClient) Save(ctx context.Context, imageName, outputPath string) error {
	cmd := exec.CommandContext(ctx, c.dockerCmd(), "save", "-o", outputPath, imageName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

type runMode struct {
	interactive bool
	autoRemove  bool
	initProcess bool
	name        string
}

// BuildRunArgs constructs the docker CLI arguments for a RunConfig.
// This is exported for testing.
func BuildRunArgs(config RunConfig) []string {
	return buildRunArgs(config, runMode{interactive: true, autoRemove: true, initProcess: true})
}

// BuildOneShotRunArgs constructs docker CLI arguments for a non-interactive,
// non-auto-remove container run. The container is given a unique name so it
// can be committed and removed later.
func BuildOneShotRunArgs(containerName string, config RunConfig) []string {
	return buildRunArgs(config, runMode{name: containerName})
}

func buildRunArgs(config RunConfig, mode runMode) []string {
	args := []string{"run"}
	if mode.interactive {
		args = append(args, "-it")
	}
	if mode.autoRemove {
		args = append(args, "--rm")
	}
	if mode.initProcess {
		args = append(args, "--init")
	}
	if mode.name != "" {
		args = append(args, "--name", mode.name)
	}
	args = append(args, "--pids-limit", "8192")

	if config.Userns != "" {
		args = append(args, "--userns", config.Userns)
	}

	for _, opt := range config.SecurityOpts {
		args = append(args, "--security-opt", opt)
	}

	for _, cap := range config.CapAdd {
		args = append(args, "--cap-add", cap)
	}

	for key, val := range config.EnvVars {
		args = append(args, "-e", fmt.Sprintf("%s=%s", key, val))
	}

	for _, m := range config.Mounts {
		mountArg := fmt.Sprintf("%s:%s", m.Source, m.Target)
		if suffix := mountSuffix(m); suffix != "" {
			mountArg += ":" + suffix
		}
		args = append(args, "-v", mountArg)
	}

	if config.User != "" {
		args = append(args, "--user", config.User)
	}

	if config.WorkDir != "" {
		args = append(args, "--workdir", config.WorkDir)
	}

	if config.Entrypoint != "" {
		args = append(args, "--entrypoint", config.Entrypoint)
	}

	args = append(args, config.ImageName)
	args = append(args, config.Command...)

	return args
}

// mountSuffix builds the colon-separated suffix for a -v mount argument
// by combining the access mode (ro/rw) with any extra options (z, Z, nocopy, etc.).
func mountSuffix(m Mount) string {
	var parts []string
	if m.ReadOnly {
		parts = append(parts, "ro")
	}
	if m.Options != "" {
		parts = append(parts, m.Options)
	}
	return strings.Join(parts, ",")
}

// BuildExecRunArgs constructs docker CLI arguments for ExecRun.
// Unlike BuildRunArgs, it omits --rm and includes --name.
func BuildExecRunArgs(containerName string, config RunConfig) []string {
	return buildRunArgs(config, runMode{interactive: true, initProcess: true, name: containerName})
}

// ExecRun replaces the current process with the container runtime via syscall.Exec.
// This function does not return on success. Any deferred cleanup in the call stack
// will NOT execute. Post-container tasks must be registered in ReaperSpec.Tasks.
func (c *ShellClient) ExecRun(containerName string, config RunConfig, spawnReaper func() (*os.File, error)) error {
	args := BuildExecRunArgs(containerName, config)
	binPath, err := exec.LookPath(c.dockerCmd())
	if err != nil {
		return fmt.Errorf("%s not found: %w", c.dockerCmd(), err)
	}

	writeFd, err := spawnReaper()
	if err != nil {
		return fmt.Errorf("reaper: %w", err)
	}

	// Clear O_CLOEXEC so the pipe write side is inherited by the container runtime
	_, _, errno := syscall.RawSyscall(syscall.SYS_FCNTL, writeFd.Fd(), syscall.F_SETFD, 0)
	if errno != 0 {
		return fmt.Errorf("fcntl F_SETFD: %w", errno)
	}

	argv := append([]string{c.dockerCmd()}, args...)
	return syscall.Exec(binPath, argv, os.Environ())
}

// Run runs a Docker container interactively with the given RunConfig.
// SIGINT is forwarded to the child so Ctrl+C works as expected.
// SIGTERM and SIGHUP are absorbed to prevent the wrapper from killing
// the container when the terminal closes or the wrapper is signalled.
func (c *ShellClient) Run(ctx context.Context, config RunConfig) error {
	args := BuildRunArgs(config)
	cmd := exec.CommandContext(ctx, c.dockerCmd(), args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return err
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGINT)
	go func() {
		for sig := range sigCh {
			if sig == syscall.SIGINT {
				_ = cmd.Process.Signal(sig)
			}
		}
	}()

	err := cmd.Wait()
	signal.Stop(sigCh)
	close(sigCh)
	return err
}

// RunOneShot runs a container non-interactively and returns the container name.
// The container is NOT automatically removed so it can be committed.
func (c *ShellClient) RunOneShot(ctx context.Context, config RunConfig) (string, error) {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	containerName := fmt.Sprintf("aw-snapshot-%x", b)

	args := BuildOneShotRunArgs(containerName, config)
	cmd := exec.CommandContext(ctx, c.dockerCmd(), args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return containerName, err
	}
	return containerName, nil
}

// Commit creates an image from a container's changes.
// changes are Dockerfile instructions (e.g. "ENV FOO=bar").
func (c *ShellClient) Commit(ctx context.Context, containerID, imageName string, changes []string) error {
	args := []string{"commit"}
	for _, ch := range changes {
		args = append(args, "--change", ch)
	}
	args = append(args, containerID, imageName)
	cmd := exec.CommandContext(ctx, c.dockerCmd(), args...)
	cmd.Stdout = nil
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// RemoveContainer removes a stopped container.
func (c *ShellClient) RemoveContainer(ctx context.Context, containerID string) error {
	cmd := exec.CommandContext(ctx, c.dockerCmd(), "rm", containerID)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}
