//go:build !darwin

package sshagent

import "fmt"

func setupPodmanDarwin(hostAuthSock string) (*ForwardedAgent, error) {
	return nil, fmt.Errorf("podman SSH agent forwarding is only supported on macOS")
}
