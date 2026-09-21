//go:build !darwin && !windows

package sshagent

import "fmt"

func setupPodmanDarwin(hostAuthSock, containerName string) (*ForwardedAgent, error) {
	return nil, fmt.Errorf("podman SSH agent forwarding is only supported on macOS")
}

// SetupSocketTunnel is only available on macOS where Podman runs in a VM.
func SetupSocketTunnel(hostSocketPath, tunnelName string) (*ForwardedAgent, error) {
	return nil, fmt.Errorf("podman socket tunneling is only supported on macOS")
}

func setupPodmanWindows(hostAuthSock, containerName string) (*ForwardedAgent, error) {
	return nil, fmt.Errorf("podman SSH agent forwarding is not supported on this platform")
}
