//go:build !darwin && !windows

package sshagent

import "fmt"

func setupPodmanDarwin(hostAuthSock, containerName string) (*ForwardedAgent, error) {
	return nil, fmt.Errorf("podman SSH agent forwarding is only supported on macOS")
}

func setupPodmanWindows(hostAuthSock, containerName string) (*ForwardedAgent, error) {
	return nil, fmt.Errorf("podman SSH agent forwarding is not supported on this platform")
}
