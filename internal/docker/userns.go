package docker

import "github.com/konono/aw/internal/platform"

// PodmanUserns returns "keep-id" for rootless Podman so that the host UID
// maps to the same UID inside the container.
func PodmanUserns(runtime string) string {
	if runtime == "podman" {
		return "keep-id"
	}
	return ""
}

// HostUserID returns the --user value mapping the current host UID with GID 0.
// On Windows, returns "" because Docker Desktop handles UID mapping automatically.
func HostUserID() string {
	return platform.HostUserID()
}
