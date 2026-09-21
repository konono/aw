//go:build darwin

package zellij

import (
	"fmt"

	"github.com/konono/aw/v4/internal/sshagent"
)

func setupPodmanDarwin(hostSocketPath, sessionName, containerName string) (*ForwardedZellij, error) {
	agent, err := sshagent.SetupSocketTunnel(hostSocketPath, containerName+"-zellij")
	if err != nil {
		return nil, fmt.Errorf("setting up zellij socket tunnel: %w", err)
	}

	return &ForwardedZellij{
		SocketPath:  agent.SocketPath,
		SessionName: sessionName,
		Cleanup:     agent.Cleanup,
	}, nil
}
