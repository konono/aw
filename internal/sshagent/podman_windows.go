//go:build windows

package sshagent

import "fmt"

func setupPodmanDarwin(hostAuthSock, containerName string) (*ForwardedAgent, error) {
	return nil, fmt.Errorf("podman SSH agent forwarding is only supported on macOS")
}

func setupPodmanWindows(hostAuthSock, containerName string) (*ForwardedAgent, error) {
	return nil, fmt.Errorf("podman SSH agent forwarding is not yet supported on Windows; use Docker Desktop or set SSH_AUTH_SOCK in a Git Bash environment")
}
