//go:build unix

package platform

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

const (
	defaultDockerSock = "/var/run/docker.sock"
	defaultPodmanSock = "/run/podman/podman.sock"
)

// DetectDockerSock returns the host path of the Docker socket.
func DetectDockerSock() (string, error) {
	if runtime.GOOS == "darwin" {
		return defaultDockerSock, nil
	}
	if _, err := os.Stat(defaultDockerSock); err != nil {
		return "", fmt.Errorf("docker socket not found at %s: %w", defaultDockerSock, err)
	}
	return defaultDockerSock, nil
}

// DetectPodmanSock returns the host path of the Podman socket.
func DetectPodmanSock() (string, error) {
	if runtime.GOOS == "darwin" {
		return defaultPodmanSock, nil
	}
	return detectPodmanSockLinux()
}

func detectPodmanSockLinux() (string, error) {
	out, err := exec.Command("podman", "info", "--format", "{{.Host.RemoteSocket.Path}}").Output()
	if err == nil {
		path := strings.TrimSpace(string(out))
		if path != "" {
			if _, err := os.Stat(path); err == nil {
				return path, nil
			}
		}
	}

	if _, err := os.Stat(defaultPodmanSock); err == nil {
		return defaultPodmanSock, nil
	}

	return "", fmt.Errorf("podman socket not found; ensure podman is running (try: systemctl --user start podman.socket)")
}
