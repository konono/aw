//go:build !darwin

package zellij

func setupPodmanDarwin(hostSocketPath, sessionName, containerName string) (*ForwardedZellij, error) {
	// On non-Darwin platforms, Podman runs natively — no tunnel needed.
	// This path is not reached because Setup() only calls this on darwin+podman.
	return &ForwardedZellij{
		SocketPath:  hostSocketPath,
		SessionName: sessionName,
		Cleanup:     func() {},
	}, nil
}
