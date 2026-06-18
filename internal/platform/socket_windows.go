//go:build windows

package platform

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const windowsDockerPipe = `//./pipe/docker_engine`

// DetectDockerSock returns the Docker socket/pipe path on Windows.
// Docker Desktop for Windows uses a named pipe.
func DetectDockerSock() (string, error) {
	if host := os.Getenv("DOCKER_HOST"); host != "" {
		return host, nil
	}
	return windowsDockerPipe, nil
}

// DetectPodmanSock returns the Podman socket path on Windows.
// Podman for Windows runs via WSL2; the socket is detected via podman info.
func DetectPodmanSock() (string, error) {
	out, err := exec.Command("podman", "info", "--format", "{{.Host.RemoteSocket.Path}}").Output()
	if err == nil {
		path := strings.TrimSpace(string(out))
		if path != "" {
			return path, nil
		}
	}

	return "", fmt.Errorf("podman socket not found; ensure Podman Desktop is running or podman machine is started")
}
