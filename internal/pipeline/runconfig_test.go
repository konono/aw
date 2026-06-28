package pipeline

import (
	"testing"

	"github.com/konono/aw/internal/docker"
)

func TestBuildRunConfig_IncludesGroupAdd(t *testing.T) {
	ec := &ExecutionContext{
		DockerImage: "test-image",
		WorkDir:     "/workspace",
	}

	rc := BuildRunConfig(ec, "docker", []string{"id"}, map[string]string{})

	if len(rc.GroupAdd) == 0 {
		t.Fatal("BuildRunConfig should set GroupAdd")
	}
	if rc.GroupAdd[0] != "0" {
		t.Errorf("GroupAdd[0] = %q, want %q", rc.GroupAdd[0], "0")
	}
}

func TestAuthRunConfig_IncludesGroupAdd(t *testing.T) {
	ec := &ExecutionContext{
		DockerImage: "test-image",
		WorkDir:     "/workspace",
	}

	rc := AuthRunConfig(ec, "podman", "claude", []string{"auth"})

	if len(rc.GroupAdd) == 0 {
		t.Fatal("AuthRunConfig should set GroupAdd")
	}
	if rc.GroupAdd[0] != "0" {
		t.Errorf("GroupAdd[0] = %q, want %q", rc.GroupAdd[0], "0")
	}
}

func TestBuildRunConfig_UserMatchesHostUserID(t *testing.T) {
	ec := &ExecutionContext{
		DockerImage: "test-image",
		WorkDir:     "/workspace",
	}

	rc := BuildRunConfig(ec, "docker", []string{"id"}, map[string]string{})

	expected := docker.HostUserID()
	if rc.User != expected {
		t.Errorf("User = %q, want %q", rc.User, expected)
	}
}

func TestBuildRunConfig_PodmanUserns(t *testing.T) {
	ec := &ExecutionContext{
		DockerImage: "test-image",
		WorkDir:     "/workspace",
	}

	rc := BuildRunConfig(ec, "podman", []string{"id"}, map[string]string{})
	if rc.Userns != "keep-id" {
		t.Errorf("Userns = %q, want %q", rc.Userns, "keep-id")
	}

	rc2 := BuildRunConfig(ec, "docker", []string{"id"}, map[string]string{})
	if rc2.Userns != "" {
		t.Errorf("Userns = %q, want empty for docker", rc2.Userns)
	}
}
