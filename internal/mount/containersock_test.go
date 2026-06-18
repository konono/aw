package mount

import (
	"runtime"
	"testing"
)

func TestDetectContainerSock_Docker(t *testing.T) {
	path, err := DetectContainerSock("docker")
	if runtime.GOOS == "darwin" {
		if err != nil {
			t.Fatalf("DetectContainerSock(docker) on darwin: unexpected error: %v", err)
		}
		if path != "/var/run/docker.sock" {
			t.Errorf("path = %q, want %q", path, "/var/run/docker.sock")
		}
	} else if runtime.GOOS != "windows" {
		if err == nil && path != "/var/run/docker.sock" {
			t.Errorf("path = %q, want %q", path, "/var/run/docker.sock")
		}
	}
}

func TestDetectContainerSock_PodmanDarwin(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("darwin-only test")
	}
	path, err := DetectContainerSock("podman")
	if err != nil {
		t.Fatalf("DetectContainerSock(podman) on darwin: unexpected error: %v", err)
	}
	if path != "/run/podman/podman.sock" {
		t.Errorf("path = %q, want %q", path, "/run/podman/podman.sock")
	}
}

func TestDetectContainerSock_UnsupportedRuntime(t *testing.T) {
	_, err := DetectContainerSock("containerd")
	if err == nil {
		t.Error("DetectContainerSock(containerd) should return an error")
	}
}

func TestContainerSockContainerPath(t *testing.T) {
	if ContainerSockContainerPath != "/run/container.sock" {
		t.Errorf("ContainerSockContainerPath = %q, want %q", ContainerSockContainerPath, "/run/container.sock")
	}
}
