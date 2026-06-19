//go:build unix

package platform

import (
	"runtime"
	"testing"
)

func TestDetectDockerSock_ReturnsPathOnDarwin(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("darwin-only test")
	}
	path, err := DetectDockerSock()
	if err != nil {
		t.Fatalf("DetectDockerSock() on darwin returned error: %v", err)
	}
	if path == "" {
		t.Error("DetectDockerSock() on darwin returned empty path")
	}
}

func TestDetectDockerSock_ReturnsPathOrErrorOnLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux-only test")
	}
	path, err := DetectDockerSock()
	if err != nil {
		return
	}
	if path == "" {
		t.Error("DetectDockerSock() returned empty path with nil error")
	}
}

func TestDetectPodmanSock_ReturnsPathOnDarwin(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("darwin-only test")
	}
	path, err := DetectPodmanSock()
	if err != nil {
		t.Fatalf("DetectPodmanSock() on darwin returned error: %v", err)
	}
	if path == "" {
		t.Error("DetectPodmanSock() on darwin returned empty path")
	}
}
