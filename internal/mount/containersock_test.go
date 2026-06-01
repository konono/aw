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
		if path != defaultDockerSock {
			t.Errorf("path = %q, want %q", path, defaultDockerSock)
		}
	} else {
		// On Linux, the socket may or may not exist depending on the environment.
		// Just verify the path is correct when it succeeds.
		if err == nil && path != defaultDockerSock {
			t.Errorf("path = %q, want %q", path, defaultDockerSock)
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
	if path != defaultPodmanSock {
		t.Errorf("path = %q, want %q", path, defaultPodmanSock)
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
