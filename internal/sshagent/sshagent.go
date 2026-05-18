package sshagent

import (
	"fmt"
	"os"
	"runtime"
)

const VMSocketPath = "/tmp/aw-ssh-agent.sock"

// ForwardedAgent holds the result of SSH agent forwarding setup.
type ForwardedAgent struct {
	SocketPath string   // path to mount into the container
	Cleanup    func()   // call on shutdown to release resources
}

// Setup configures SSH agent forwarding for the given container runtime.
// On Linux and macOS+Docker, it returns the host's SSH_AUTH_SOCK directly.
// On macOS+Podman, it establishes an SSH tunnel into the Podman VM.
func Setup(containerRuntime string) (*ForwardedAgent, error) {
	hostSock := os.Getenv("SSH_AUTH_SOCK")
	if hostSock == "" {
		return nil, fmt.Errorf("SSH_AUTH_SOCK is not set; ensure ssh-agent is running")
	}

	if runtime.GOOS == "darwin" && containerRuntime == "podman" {
		return setupPodmanDarwin(hostSock)
	}

	if _, err := os.Stat(hostSock); err != nil {
		return nil, fmt.Errorf("SSH_AUTH_SOCK %q does not exist: %w", hostSock, err)
	}

	return &ForwardedAgent{
		SocketPath: hostSock,
		Cleanup:    func() {},
	}, nil
}
