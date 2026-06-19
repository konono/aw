package mount

import (
	"fmt"

	"github.com/konono/aw/internal/platform"
)

// ContainerSockContainerPath is the fixed path where the container runtime socket
// is mounted inside the container.
const ContainerSockContainerPath = "/run/container.sock"

// DetectContainerSock returns the socket path to mount as the source for the
// container runtime socket. The path depends on the runtime and platform.
func DetectContainerSock(containerRuntime string) (string, error) {
	switch containerRuntime {
	case "docker":
		return platform.DetectDockerSock()
	case "podman":
		return platform.DetectPodmanSock()
	default:
		return "", fmt.Errorf("unsupported container runtime: %q", containerRuntime)
	}
}
