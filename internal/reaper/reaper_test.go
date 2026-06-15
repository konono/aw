package reaper

import (
	"encoding/json"
	"testing"

	"github.com/konono/aw/internal/docker"
)

func TestReaperSpecJSON(t *testing.T) {
	spec := ReaperSpec{
		Timeout:       60,
		ContainerName: "aw-claude-1750000690",
		Runtime:       "podman",
		KeepContainer: false,
		PodmanSSH: &PodmanSSHConfig{
			IdentityPath:   "/Users/test/.ssh/podman-machine-default",
			Port:           54321,
			RemoteUsername: "core",
		},
		Tasks: []ReaperTask{
			{
				Type:   "kill_process",
				Label:  "ssh-tunnel",
				Config: json.RawMessage(`{"pid":5678,"signal":15}`),
			},
			{
				Type:   "remove_file",
				Label:  "vm-socket",
				Config: json.RawMessage(`{"path":"/tmp/aw-ssh-agent.sock","host":"podman-vm"}`),
			},
		},
	}

	data, err := json.Marshal(spec)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded ReaperSpec
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.ContainerName != spec.ContainerName {
		t.Errorf("ContainerName = %q, want %q", decoded.ContainerName, spec.ContainerName)
	}
	if decoded.Runtime != spec.Runtime {
		t.Errorf("Runtime = %q, want %q", decoded.Runtime, spec.Runtime)
	}
	if decoded.PodmanSSH == nil {
		t.Fatal("PodmanSSH should not be nil")
	}
	if decoded.PodmanSSH.Port != spec.PodmanSSH.Port {
		t.Errorf("PodmanSSH.Port = %d, want %d", decoded.PodmanSSH.Port, spec.PodmanSSH.Port)
	}
	if len(decoded.Tasks) != 2 {
		t.Fatalf("len(Tasks) = %d, want 2", len(decoded.Tasks))
	}
	if decoded.Tasks[0].Type != "kill_process" {
		t.Errorf("Tasks[0].Type = %q, want %q", decoded.Tasks[0].Type, "kill_process")
	}
}

func TestBuildExecRunArgs_NoRm(t *testing.T) {
	args := docker.BuildExecRunArgs("aw-test-123", docker.RunConfig{
		ImageName: "test-image",
	})

	for _, arg := range args {
		if arg == "--rm" {
			t.Error("BuildExecRunArgs should not include --rm")
		}
	}
}

func TestBuildExecRunArgs_HasName(t *testing.T) {
	args := docker.BuildExecRunArgs("aw-test-123", docker.RunConfig{
		ImageName: "test-image",
	})

	found := false
	for i, arg := range args {
		if arg == "--name" && i+1 < len(args) && args[i+1] == "aw-test-123" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("BuildExecRunArgs should include --name aw-test-123, got %v", args)
	}
}

func TestBuildExecRunArgs_HasInit(t *testing.T) {
	args := docker.BuildExecRunArgs("aw-test-123", docker.RunConfig{
		ImageName: "test-image",
	})

	found := false
	for _, arg := range args {
		if arg == "--init" {
			found = true
			break
		}
	}
	if !found {
		t.Error("BuildExecRunArgs should include --init")
	}
}

func TestRunReportJSON(t *testing.T) {
	report := RunReport{
		ContainerName: "aw-test-123",
		ExitCode:      0,
		Tasks: []TaskResult{
			{Type: "kill_process", Label: "ssh-tunnel", Status: "ok"},
		},
	}

	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded RunReport
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", decoded.ExitCode)
	}
	if len(decoded.Tasks) != 1 {
		t.Fatalf("len(Tasks) = %d, want 1", len(decoded.Tasks))
	}
	if decoded.Tasks[0].Status != "ok" {
		t.Errorf("Tasks[0].Status = %q, want %q", decoded.Tasks[0].Status, "ok")
	}
}
