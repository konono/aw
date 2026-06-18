//go:build windows

package platform

import (
	"testing"
)

func TestDetectDockerSock_DefaultPipe(t *testing.T) {
	t.Setenv("DOCKER_HOST", "")
	path, err := DetectDockerSock()
	if err != nil {
		t.Fatalf("DetectDockerSock() returned error: %v", err)
	}
	if path == "" {
		t.Error("DetectDockerSock() returned empty path")
	}
}

func TestDetectDockerSock_RespectsDockerHost(t *testing.T) {
	custom := "tcp://localhost:2375"
	t.Setenv("DOCKER_HOST", custom)
	path, err := DetectDockerSock()
	if err != nil {
		t.Fatalf("DetectDockerSock() returned error: %v", err)
	}
	if path != custom {
		t.Errorf("DetectDockerSock() = %q, want %q", path, custom)
	}
}
