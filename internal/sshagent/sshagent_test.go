package sshagent

import (
	"strings"
	"testing"

	"github.com/konono/aw/internal/pipeline"
)

func TestVMSocketPath(t *testing.T) {
	tests := []struct {
		name          string
		containerName string
		wantContains  string
	}{
		{
			name:          "normal name",
			containerName: "my-container",
			wantContains:  "aw-ssh-agent-my-container.sock",
		},
		{
			name:          "empty name",
			containerName: "",
			wantContains:  "aw-ssh-agent-.sock",
		},
		{
			name:          "name with dots",
			containerName: "aw.project.claude",
			wantContains:  "aw-ssh-agent-aw.project.claude.sock",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := VMSocketPath(tt.containerName)
			if !strings.Contains(got, tt.wantContains) {
				t.Errorf("VMSocketPath(%q) = %q, want path containing %q", tt.containerName, got, tt.wantContains)
			}
		})
	}
}

func TestReaperTunnelPID(t *testing.T) {
	t.Run("nil receiver", func(t *testing.T) {
		var a *ForwardedAgent
		if got := a.ReaperTunnelPID(); got != 0 {
			t.Errorf("ReaperTunnelPID() = %d, want 0", got)
		}
	})

	t.Run("with value", func(t *testing.T) {
		a := &ForwardedAgent{SSHTunnelPID: 12345}
		if got := a.ReaperTunnelPID(); got != 12345 {
			t.Errorf("ReaperTunnelPID() = %d, want 12345", got)
		}
	})
}

func TestReaperSocketPath(t *testing.T) {
	t.Run("nil receiver", func(t *testing.T) {
		var a *ForwardedAgent
		if got := a.ReaperSocketPath(); got != "" {
			t.Errorf("ReaperSocketPath() = %q, want empty", got)
		}
	})

	t.Run("with value", func(t *testing.T) {
		a := &ForwardedAgent{SocketPath: "/tmp/test.sock"}
		if got := a.ReaperSocketPath(); got != "/tmp/test.sock" {
			t.Errorf("ReaperSocketPath() = %q, want %q", got, "/tmp/test.sock")
		}
	})
}

func TestReaperPodmanSSH(t *testing.T) {
	t.Run("nil receiver", func(t *testing.T) {
		var a *ForwardedAgent
		if got := a.ReaperPodmanSSH(); got != nil {
			t.Errorf("ReaperPodmanSSH() = %v, want nil", got)
		}
	})

	t.Run("nil SSHConfig", func(t *testing.T) {
		a := &ForwardedAgent{}
		if got := a.ReaperPodmanSSH(); got != nil {
			t.Errorf("ReaperPodmanSSH() = %v, want nil", got)
		}
	})

	t.Run("with SSHConfig", func(t *testing.T) {
		a := &ForwardedAgent{
			SSHConfig: &PodmanSSHConfig{
				IdentityPath:   "/home/user/.ssh/id_ed25519",
				Port:           2222,
				RemoteUsername: "core",
			},
		}
		got := a.ReaperPodmanSSH()
		if got == nil {
			t.Fatal("ReaperPodmanSSH() = nil, want non-nil")
			return
		}
		want := &pipeline.PodmanSSHConfig{
			IdentityPath:   "/home/user/.ssh/id_ed25519",
			Port:           2222,
			RemoteUsername: "core",
		}
		if got.IdentityPath != want.IdentityPath {
			t.Errorf("IdentityPath = %q, want %q", got.IdentityPath, want.IdentityPath)
		}
		if got.Port != want.Port {
			t.Errorf("Port = %d, want %d", got.Port, want.Port)
		}
		if got.RemoteUsername != want.RemoteUsername {
			t.Errorf("RemoteUsername = %q, want %q", got.RemoteUsername, want.RemoteUsername)
		}
	})
}
