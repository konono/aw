package docker

import "github.com/konono/aw/v4/internal/platform"

// PodmanUserns returns "keep-id" for rootless Podman so that the host UID
// maps to the same UID inside the container.
func PodmanUserns(runtime string) string {
	if runtime == "podman" {
		return "keep-id"
	}
	return ""
}

// HostUserID returns the --user value mapping the current host UID and GID.
// On Windows, returns "" because Docker Desktop handles UID mapping automatically.
func HostUserID() string {
	return platform.HostUserID()
}

// RootGroupAdd returns the supplementary groups to add to the container process.
// GID 0 (root group) is added so the container can write to paths that are
// root-group-writable inside the image (OpenShift GID 0 pattern).
func RootGroupAdd() []string {
	return []string{"0"}
}
