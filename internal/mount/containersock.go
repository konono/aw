package mount

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// ContainerSockContainerPath is the fixed path where the container runtime socket
// is mounted inside the container.
const ContainerSockContainerPath = "/run/container.sock"

const defaultDockerSock = "/var/run/docker.sock"
const defaultPodmanSock = "/run/podman/podman.sock"

// DetectContainerSock returns the socket path to mount as the source for the
// container runtime socket. The path depends on the runtime and platform.
//
// For Docker (all platforms): /var/run/docker.sock
// For Podman on macOS: /run/podman/podman.sock (VM-internal path)
// For Podman on Linux: auto-detected via podman info, with fallback
func DetectContainerSock(containerRuntime string) (string, error) {
	switch containerRuntime {
	case "docker":
		return detectDockerSock()
	case "podman":
		return detectPodmanSock()
	default:
		return "", fmt.Errorf("unsupported container runtime: %q", containerRuntime)
	}
}

func detectDockerSock() (string, error) {
	if runtime.GOOS == "darwin" {
		return defaultDockerSock, nil
	}
	if _, err := os.Stat(defaultDockerSock); err != nil {
		return "", fmt.Errorf("docker socket not found at %s: %w", defaultDockerSock, err)
	}
	return defaultDockerSock, nil
}

func detectPodmanSock() (string, error) {
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
