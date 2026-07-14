package doctor

import (
	"testing"

	"github.com/konono/aw/v4/internal/profile"
)

func boolPtr(b bool) *bool { return &b }

func TestCollectRuntimes(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *profile.Config
		wantKeys map[string]bool
	}{
		{
			name:     "no profiles",
			cfg:      &profile.Config{Profiles: map[string]profile.Profile{}},
			wantKeys: map[string]bool{},
		},
		{
			name: "docker only",
			cfg: &profile.Config{Profiles: map[string]profile.Profile{
				"a": {Environment: profile.EnvironmentContainer, ContainerRuntime: profile.ContainerRuntimeDocker},
			}},
			wantKeys: map[string]bool{"docker": true},
		},
		{
			name: "podman only",
			cfg: &profile.Config{Profiles: map[string]profile.Profile{
				"a": {Environment: profile.EnvironmentContainer, ContainerRuntime: profile.ContainerRuntimePodman},
			}},
			wantKeys: map[string]bool{"podman": true},
		},
		{
			name: "mixed runtimes",
			cfg: &profile.Config{Profiles: map[string]profile.Profile{
				"a": {Environment: profile.EnvironmentContainer, ContainerRuntime: profile.ContainerRuntimeDocker},
				"b": {Environment: profile.EnvironmentContainer, ContainerRuntime: profile.ContainerRuntimePodman},
			}},
			wantKeys: map[string]bool{"docker": true, "podman": true},
		},
		{
			name: "host environment is ignored",
			cfg: &profile.Config{Profiles: map[string]profile.Profile{
				"a": {Environment: profile.EnvironmentHost, ContainerRuntime: profile.ContainerRuntimeDocker},
			}},
			wantKeys: map[string]bool{},
		},
		{
			name: "empty environment is included",
			cfg: &profile.Config{Profiles: map[string]profile.Profile{
				"a": {ContainerRuntime: profile.ContainerRuntimeDocker},
			}},
			wantKeys: map[string]bool{"docker": true},
		},
		{
			name: "empty runtime defaults to docker",
			cfg: &profile.Config{Profiles: map[string]profile.Profile{
				"a": {Environment: profile.EnvironmentContainer},
			}},
			wantKeys: map[string]bool{"docker": true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := collectRuntimes(tt.cfg)
			if len(got) != len(tt.wantKeys) {
				t.Fatalf("collectRuntimes() returned %d keys, want %d: got=%v", len(got), len(tt.wantKeys), got)
			}
			for k, v := range tt.wantKeys {
				if got[k] != v {
					t.Errorf("collectRuntimes()[%q] = %v, want %v", k, got[k], v)
				}
			}
		})
	}
}

func TestCollectGHNeeds(t *testing.T) {
	tests := []struct {
		name          string
		cfg           *profile.Config
		wantGHToken   bool
		wantMountGH   bool
	}{
		{
			name:        "no profiles",
			cfg:         &profile.Config{Profiles: map[string]profile.Profile{}},
			wantGHToken: false,
			wantMountGH: false,
		},
		{
			name: "gh_token only",
			cfg: &profile.Config{Profiles: map[string]profile.Profile{
				"a": {GhToken: boolPtr(true)},
			}},
			wantGHToken: true,
			wantMountGH: false,
		},
		{
			name: "mount_gh only",
			cfg: &profile.Config{Profiles: map[string]profile.Profile{
				"a": {MountGH: boolPtr(true)},
			}},
			wantGHToken: false,
			wantMountGH: true,
		},
		{
			name: "both true",
			cfg: &profile.Config{Profiles: map[string]profile.Profile{
				"a": {GhToken: boolPtr(true), MountGH: boolPtr(true)},
			}},
			wantGHToken: true,
			wantMountGH: true,
		},
		{
			name: "explicit false",
			cfg: &profile.Config{Profiles: map[string]profile.Profile{
				"a": {GhToken: boolPtr(false), MountGH: boolPtr(false)},
			}},
			wantGHToken: false,
			wantMountGH: false,
		},
		{
			name: "nil pointers",
			cfg: &profile.Config{Profiles: map[string]profile.Profile{
				"a": {},
			}},
			wantGHToken: false,
			wantMountGH: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotToken, gotMount := collectGHNeeds(tt.cfg)
			if gotToken != tt.wantGHToken {
				t.Errorf("collectGHNeeds() ghToken = %v, want %v", gotToken, tt.wantGHToken)
			}
			if gotMount != tt.wantMountGH {
				t.Errorf("collectGHNeeds() mountGH = %v, want %v", gotMount, tt.wantMountGH)
			}
		})
	}
}

func TestCollectSSHNeeds(t *testing.T) {
	tests := []struct {
		name      string
		cfg       *profile.Config
		wantSSH   bool
		wantAgent bool
	}{
		{
			name:      "no profiles",
			cfg:       &profile.Config{Profiles: map[string]profile.Profile{}},
			wantSSH:   false,
			wantAgent: false,
		},
		{
			name: "mount_ssh only",
			cfg: &profile.Config{Profiles: map[string]profile.Profile{
				"a": {MountSSH: boolPtr(true)},
			}},
			wantSSH:   true,
			wantAgent: false,
		},
		{
			name: "ssh_agent_forwarding only",
			cfg: &profile.Config{Profiles: map[string]profile.Profile{
				"a": {SSHAgentForwarding: boolPtr(true)},
			}},
			wantSSH:   false,
			wantAgent: true,
		},
		{
			name: "both true",
			cfg: &profile.Config{Profiles: map[string]profile.Profile{
				"a": {MountSSH: boolPtr(true), SSHAgentForwarding: boolPtr(true)},
			}},
			wantSSH:   true,
			wantAgent: true,
		},
		{
			name: "explicit false",
			cfg: &profile.Config{Profiles: map[string]profile.Profile{
				"a": {MountSSH: boolPtr(false), SSHAgentForwarding: boolPtr(false)},
			}},
			wantSSH:   false,
			wantAgent: false,
		},
		{
			name: "mixed across profiles",
			cfg: &profile.Config{Profiles: map[string]profile.Profile{
				"a": {MountSSH: boolPtr(true)},
				"b": {SSHAgentForwarding: boolPtr(true)},
			}},
			wantSSH:   true,
			wantAgent: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSSH, gotAgent := collectSSHNeeds(tt.cfg)
			if gotSSH != tt.wantSSH {
				t.Errorf("collectSSHNeeds() ssh = %v, want %v", gotSSH, tt.wantSSH)
			}
			if gotAgent != tt.wantAgent {
				t.Errorf("collectSSHNeeds() agent = %v, want %v", gotAgent, tt.wantAgent)
			}
		})
	}
}

func TestCollectContainerSockNeeds(t *testing.T) {
	tests := []struct {
		name string
		cfg  *profile.Config
		want bool
	}{
		{
			name: "no profiles",
			cfg:  &profile.Config{Profiles: map[string]profile.Profile{}},
			want: false,
		},
		{
			name: "mount_container_sock true",
			cfg: &profile.Config{Profiles: map[string]profile.Profile{
				"a": {MountContainerSock: boolPtr(true)},
			}},
			want: true,
		},
		{
			name: "mount_container_sock false",
			cfg: &profile.Config{Profiles: map[string]profile.Profile{
				"a": {MountContainerSock: boolPtr(false)},
			}},
			want: false,
		},
		{
			name: "nil pointer",
			cfg: &profile.Config{Profiles: map[string]profile.Profile{
				"a": {},
			}},
			want: false,
		},
		{
			name: "one true among many",
			cfg: &profile.Config{Profiles: map[string]profile.Profile{
				"a": {MountContainerSock: boolPtr(false)},
				"b": {MountContainerSock: boolPtr(true)},
			}},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := collectContainerSockNeeds(tt.cfg)
			if got != tt.want {
				t.Errorf("collectContainerSockNeeds() = %v, want %v", got, tt.want)
			}
		})
	}
}
