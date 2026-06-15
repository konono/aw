package sshagent

import (
	"fmt"
	"os"
	"runtime"
)

// VMSocketPath returns the session-specific socket path for SSH agent forwarding
// into the Podman VM. Each session gets its own socket to avoid interference
// between concurrent aw instances.
func VMSocketPath(containerName string) string {
	return fmt.Sprintf("/tmp/aw-ssh-agent-%s.sock", containerName)
}

// ForwardedAgent holds the result of SSH agent forwarding setup.
type ForwardedAgent struct {
	SocketPath   string           // path to mount into the container
	Cleanup      func()           // call on shutdown to release resources
	SSHTunnelPID int              // PID of SSH tunnel process (macOS+Podman only)
	SSHConfig    *PodmanSSHConfig // SSH config for Podman VM access (macOS+Podman only)
}

// PodmanSSHConfig holds SSH connection info for the Podman machine.
type PodmanSSHConfig struct {
	IdentityPath   string
	Port           int
	RemoteUsername string
}

// Setup configures SSH agent forwarding for the given container runtime.
// On Linux and macOS+Docker, it returns the host's SSH_AUTH_SOCK directly.
// On macOS+Podman, it establishes an SSH tunnel into the Podman VM.
// containerName is used to create a session-specific socket path.
func Setup(containerRuntime, containerName string) (*ForwardedAgent, error) {
	hostSock := os.Getenv("SSH_AUTH_SOCK")
	if hostSock == "" {
		return nil, fmt.Errorf("SSH_AUTH_SOCK is not set; ensure ssh-agent is running")
	}

	if runtime.GOOS == "darwin" && containerRuntime == "podman" {
		return setupPodmanDarwin(hostSock, containerName)
	}

	if _, err := os.Stat(hostSock); err != nil {
		return nil, fmt.Errorf("SSH_AUTH_SOCK %q does not exist: %w", hostSock, err)
	}

	return &ForwardedAgent{
		SocketPath: hostSock,
		Cleanup:    func() {},
	}, nil
}
