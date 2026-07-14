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
	"strconv"
	"strings"
	"time"

	"github.com/konono/aw/v4/internal/platform"

	"golang.org/x/term"
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
	GroupAdd     []string // --group-add values (e.g. "0" for root group)
	User         string   // overrides the image's USER (e.g. "1000:1000")
	Userns       string   // --userns value (e.g. "keep-id" for rootless Podman)
}

// Client is the interface for Docker operations.
type Client interface {
	CheckAvailable() error
	Build(ctx context.Context, imageName, contextDir, dockerfilePath string, buildArgs map[string]string, noCache bool) error
	ImageExists(ctx context.Context, imageName string) (bool, error)
	Pull(ctx context.Context, imageName string) error
	Save(ctx context.Context, imageName, outputPath string) error
	Tag(ctx context.Context, source, target string) error
	Push(ctx context.Context, imageName string) error
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

// DockerCmd returns the container runtime command name (e.g. "docker" or "podman").
func (c *ShellClient) DockerCmd() string {
	return c.dockerCmd()
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
func (c *ShellClient) Build(ctx context.Context, imageName, contextDir, dockerfilePath string, buildArgs map[string]string, noCache bool) error {
	args := []string{"build", "--load", "-t", imageName}
	if noCache {
		args = append(args, "--no-cache")
	}
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

// Pull downloads a container image from a registry.
func (c *ShellClient) Pull(ctx context.Context, imageName string) error {
	cmd := exec.CommandContext(ctx, c.dockerCmd(), "pull", imageName)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s pull %q: %w", c.dockerCmd(), imageName, err)
	}
	return nil
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
	detached    bool
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
	if mode.detached {
		args = append(args, "-d")
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

	for _, group := range config.GroupAdd {
		args = append(args, "--group-add", group)
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

// BuildDetachedRunArgs constructs docker CLI arguments for a detached container start.
// Like BuildExecRunArgs but adds -d for detached mode.
func BuildDetachedRunArgs(containerName string, config RunConfig) []string {
	return buildRunArgs(config, runMode{interactive: true, initProcess: true, detached: true, name: containerName})
}

// ExecRunForeground runs a container in the foreground (no detach+attach).
// Use for one-shot commands passed via -- where the container exits quickly.
// Unlike ExecRun, signals are not absorbed — SIGTERM/SIGHUP terminate the
// process naturally, and the reaper handles container cleanup.
func (c *ShellClient) ExecRunForeground(containerName string, config RunConfig, spawnReaper func() (*os.File, func(), error)) error {
	writeFd, abort, err := spawnReaper()
	if err != nil {
		return fmt.Errorf("reaper: %w", err)
	}

	runArgs := BuildExecRunArgs(containerName, config)
	cmd := exec.Command(c.dockerCmd(), runArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	runErr := cmd.Run()

	exitCode := 0
	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			if abort != nil {
				abort()
			}
			return fmt.Errorf("running container: %w", runErr)
		}
	}

	_ = writeFd.Close()
	os.Exit(exitCode)
	return nil
}

// ExecRun starts a container in detached mode, then attaches to it.
// SIGTERM and SIGHUP are absorbed so the container survives external signals.
// If the attach session is interrupted, it auto-reconnects while the container
// is still running. This function does not return on success (calls os.Exit).
// Post-container tasks must be registered in ReaperSpec.Tasks.
func (c *ShellClient) ExecRun(containerName string, config RunConfig, spawnReaper func() (*os.File, func(), error)) error {
	// Spawn reaper before starting the container so there is no window
	// where a SIGKILL could leave an orphaned container with no reaper/spec.
	writeFd, abort, err := spawnReaper()
	if err != nil {
		return fmt.Errorf("reaper: %w", err)
	}

	startArgs := BuildDetachedRunArgs(containerName, config)
	startCmd := exec.Command(c.dockerCmd(), startArgs...)
	startCmd.Stdin = os.Stdin
	startCmd.Stdout = nil
	startCmd.Stderr = os.Stderr
	if err := startCmd.Run(); err != nil {
		if abort != nil {
			abort()
		}
		return fmt.Errorf("starting container: %w", err)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, platform.ContainerSurvivalSignals()...)
	go func() {
		for range sigCh {
		}
	}()

	const (
		maxRapidFailures = 3
		maxAttachAttempts = 20
	)
	rapidFailures := 0
	attachAttempts := 0

	for {
		start := time.Now()

		attachCmd := exec.Command(c.dockerCmd(), "attach", "--sig-proxy=false", containerName)
		attachCmd.Stdin = os.Stdin
		attachCmd.Stdout = os.Stdout
		attachCmd.Stderr = os.Stderr
		_ = attachCmd.Run()

		if !c.isContainerRunning(containerName) {
			break
		}

		// Non-TTY stdin (pipes, redirects, CI) cannot reconnect attach,
		// so exit the loop and let the reaper handle the container.
		if !term.IsTerminal(int(os.Stdin.Fd())) {
			break
		}

		attachAttempts++
		if attachAttempts >= maxAttachAttempts {
			break
		}

		if time.Since(start) < time.Second {
			rapidFailures++
			if rapidFailures >= maxRapidFailures {
				break
			}
		} else {
			rapidFailures = 0
		}
	}

	signal.Stop(sigCh)
	close(sigCh)

	// Get exit code before closing the pipe — once the reaper fires it may
	// remove the container before we can inspect it.
	// Only query when the container has actually stopped; a running container's
	// State.ExitCode is 0 (default) which would mask the abnormal exit.
	exitCode := 1
	if !c.isContainerRunning(containerName) {
		exitCode = c.getContainerExitCode(containerName)
	}

	// Close pipe to signal the reaper. If the container already exited, the
	// reaper proceeds to cleanup immediately. If still running (TTY lost),
	// the reaper's podman-wait blocks until the container eventually stops,
	// then runs cleanup (SSH tunnel, container rm, report).
	_ = writeFd.Close()
	os.Exit(exitCode)
	return nil
}

func (c *ShellClient) inspectField(name, tmpl string) (string, error) {
	out, err := exec.Command(c.dockerCmd(), "inspect", "--format", tmpl, name).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// isContainerRunning returns true if the container is confirmed running,
// false if confirmed stopped or not found. Retries on transient inspect
// failures to avoid misidentifying a running container as stopped.
func (c *ShellClient) isContainerRunning(name string) bool {
	const maxRetries = 3
	for i := range maxRetries {
		out, err := exec.Command(c.dockerCmd(), "inspect", "--format", "{{.State.Running}}", name).CombinedOutput()
		if err == nil {
			return strings.TrimSpace(string(out)) == "true"
		}
		if IsInspectNotRecoverable(out, err) {
			return false
		}
		if i < maxRetries-1 {
			time.Sleep(500 * time.Millisecond)
		}
	}
	// All retries failed — assume still running so the attach loop
	// continues rather than exiting with a wrong exit code.
	return true
}

// IsInspectNotRecoverable returns true when an inspect error is permanent
// (container not found or runtime binary missing) rather than transient.
func IsInspectNotRecoverable(output []byte, err error) bool {
	var execErr *exec.Error
	if errors.As(err, &execErr) {
		return true
	}
	msg := strings.ToLower(strings.TrimSpace(string(output)))
	return strings.Contains(msg, "no such container") ||
		strings.Contains(msg, "no such object")
}

func (c *ShellClient) getContainerExitCode(name string) int {
	const maxRetries = 3
	for i := range maxRetries {
		val, err := c.inspectField(name, "{{.State.ExitCode}}")
		if err == nil {
			if code, perr := strconv.Atoi(val); perr == nil {
				return code
			}
		}
		if i < maxRetries-1 {
			time.Sleep(500 * time.Millisecond)
		}
	}
	return 1
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

// Tag tags a local image with a new name.
func (c *ShellClient) Tag(ctx context.Context, source, target string) error {
	cmd := exec.CommandContext(ctx, c.dockerCmd(), "tag", source, target)
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Push pushes an image to a container registry.
func (c *ShellClient) Push(ctx context.Context, imageName string) error {
	cmd := exec.CommandContext(ctx, c.dockerCmd(), "push", imageName)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// StartDetached starts a container in detached mode without attaching.
func (c *ShellClient) StartDetached(containerName string, config RunConfig) error {
	args := BuildDetachedRunArgs(containerName, config)
	cmd := exec.Command(c.dockerCmd(), args...)
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// StopContainer stops a running container.
func (c *ShellClient) StopContainer(containerName string) error {
	cmd := exec.Command(c.dockerCmd(), "stop", containerName)
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
