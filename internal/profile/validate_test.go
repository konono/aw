package profile

import (
	"strings"
	"testing"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		profile Profile
		wantErr string
	}{
		{
			name: "valid docker + claude",
			profile: Profile{
				Environment: EnvironmentContainer,
				Launch:      LaunchClaude,
			},
		},
		{
			name: "valid host + shell with worktree",
			profile: Profile{
				Worktree:    &WorktreeConfig{Base: "origin/main"},
				Environment: EnvironmentHost,
				Launch:      LaunchShell,
			},
		},
		{
			name: "valid docker + zellij with config",
			profile: Profile{
				Worktree:    &WorktreeConfig{},
				Environment: EnvironmentContainer,
				Launch:      LaunchZellij,
				Zellij:      &ZellijConfig{Layout: "default"},
			},
		},
		{
			name: "valid docker + codex",
			profile: Profile{
				Environment: EnvironmentContainer,
				Launch:      LaunchCodex,
			},
		},
		{
			name: "valid host + codex",
			profile: Profile{
				Environment: EnvironmentHost,
				Launch:      LaunchCodex,
			},
		},
		{
			name: "valid zellij with tool codex",
			profile: Profile{
				Environment: EnvironmentContainer,
				Launch:      LaunchZellij,
				Zellij:      &ZellijConfig{Layout: "default", Tool: "codex"},
			},
		},
		{
			name: "invalid zellij tool",
			profile: Profile{
				Environment: EnvironmentContainer,
				Launch:      LaunchZellij,
				Zellij:      &ZellijConfig{Layout: "default", Tool: "gemini"},
			},
			wantErr: "unknown zellij tool",
		},
		{
			name: "missing environment",
			profile: Profile{
				Launch: LaunchClaude,
			},
			wantErr: "environment is required",
		},
		{
			name: "missing launch",
			profile: Profile{
				Environment: EnvironmentContainer,
			},
			wantErr: "launch is required",
		},
		{
			name: "unknown environment",
			profile: Profile{
				Environment: "kubernetes",
				Launch:      LaunchClaude,
			},
			wantErr: "unknown environment",
		},
		{
			name: "unknown launch mode",
			profile: Profile{
				Environment: EnvironmentHost,
				Launch:      "tmux",
			},
			wantErr: "unknown launch mode",
		},
		{
			name: "zellij config with non-zellij launch",
			profile: Profile{
				Environment: EnvironmentContainer,
				Launch:      LaunchClaude,
				Zellij:      &ZellijConfig{Layout: "default"},
			},
			wantErr: "zellij config is only valid with launch: zellij",
		},
		{
			name: "valid docker with custom dockerfile",
			profile: Profile{
				Environment: EnvironmentContainer,
				Launch:      LaunchClaude,
				Dockerfile:  "docker/Dockerfile.custom",
			},
		},
		{
			name: "dockerfile with non-docker environment",
			profile: Profile{
				Environment: EnvironmentHost,
				Launch:      LaunchShell,
				Dockerfile:  "docker/Dockerfile.custom",
			},
			wantErr: "dockerfile is only valid with environment: container",
		},
		{
			name: "valid container + claude + os debian12",
			profile: Profile{
				Environment: EnvironmentContainer,
				Launch:      LaunchClaude,
				OS:          OSDebian12,
			},
		},
		{
			name: "valid container + claude + os ubi9",
			profile: Profile{
				Environment: EnvironmentContainer,
				Launch:      LaunchClaude,
				OS:          OSUBI9,
			},
		},
		{
			name: "valid container + claude + os ubi10",
			profile: Profile{
				Environment: EnvironmentContainer,
				Launch:      LaunchClaude,
				OS:          OSUBI10,
			},
		},
		{
			name: "valid container + claude + os ubuntu2604",
			profile: Profile{
				Environment: EnvironmentContainer,
				Launch:      LaunchClaude,
				OS:          OSUbuntu2604,
			},
		},
		{
			name: "unknown os value",
			profile: Profile{
				Environment: EnvironmentContainer,
				Launch:      LaunchClaude,
				OS:          "centos7",
			},
			wantErr: "unknown os",
		},
		{
			name: "os with host environment",
			profile: Profile{
				Environment: EnvironmentHost,
				Launch:      LaunchShell,
				OS:          OSUBI9,
			},
			wantErr: "os is only valid with environment: container",
		},
		{
			name: "os and dockerfile mutually exclusive",
			profile: Profile{
				Environment: EnvironmentContainer,
				Launch:      LaunchClaude,
				OS:          OSUBI10,
				Dockerfile:  "custom/Dockerfile",
			},
			wantErr: "os and dockerfile are mutually exclusive",
		},
		{
			name: "dockerfile without os is valid",
			profile: Profile{
				Environment: EnvironmentContainer,
				Launch:      LaunchClaude,
				Dockerfile:  "custom/Dockerfile",
			},
		},
		{
			name: "valid container + image",
			profile: Profile{
				Environment: EnvironmentContainer,
				Launch:      LaunchClaude,
				Image:       "my-image:latest",
			},
		},
		{
			name: "image with host environment",
			profile: Profile{
				Environment: EnvironmentHost,
				Launch:      LaunchShell,
				Image:       "my-image:latest",
			},
			wantErr: "image is only valid with environment: container",
		},
		{
			name: "image and os coexist (image takes priority)",
			profile: Profile{
				Environment: EnvironmentContainer,
				Launch:      LaunchClaude,
				Image:       "my-image:latest",
				OS:          OSDebian12,
			},
		},
		{
			name: "image and dockerfile coexist (image takes priority)",
			profile: Profile{
				Environment: EnvironmentContainer,
				Launch:      LaunchClaude,
				Image:       "my-image:latest",
				Dockerfile:  "custom/Dockerfile",
			},
		},
		{
			name:    "valid container + skip_devbox_install",
			profile: func() Profile { v := true; return Profile{Environment: EnvironmentContainer, Launch: LaunchClaude, SkipDevboxInstall: &v} }(),
		},
		{
			name:    "skip_devbox_install with host environment",
			profile: func() Profile { v := true; return Profile{Environment: EnvironmentHost, Launch: LaunchShell, SkipDevboxInstall: &v} }(),
			wantErr: "skip_devbox_install is only valid with environment: container",
		},
		{
			name:    "valid container + skip_mise_install",
			profile: func() Profile { v := true; return Profile{Environment: EnvironmentContainer, Launch: LaunchClaude, SkipMiseInstall: &v} }(),
		},
		{
			name:    "skip_mise_install with host environment",
			profile: func() Profile { v := true; return Profile{Environment: EnvironmentHost, Launch: LaunchShell, SkipMiseInstall: &v} }(),
			wantErr: "skip_mise_install is only valid with environment: container",
		},
		{
			name:    "valid container + ssh_agent_forwarding",
			profile: func() Profile { v := true; return Profile{Environment: EnvironmentContainer, Launch: LaunchClaude, SSHAgentForwarding: &v} }(),
		},
		{
			name:    "ssh_agent_forwarding with host environment",
			profile: func() Profile { v := true; return Profile{Environment: EnvironmentHost, Launch: LaunchShell, SSHAgentForwarding: &v} }(),
			wantErr: "ssh_agent_forwarding is only valid with environment: container",
		},
		{
			name:    "valid container + mount_container_sock",
			profile: func() Profile { v := true; return Profile{Environment: EnvironmentContainer, Launch: LaunchClaude, MountContainerSock: &v} }(),
		},
		{
			name:    "gh_token with host environment",
			profile: func() Profile { v := true; return Profile{Environment: EnvironmentHost, Launch: LaunchShell, GhToken: &v} }(),
			wantErr: "gh_token is only valid with environment: container",
		},
		{
			name: "gh_token and mount_gh mutually exclusive",
			profile: func() Profile {
				v := true
				return Profile{Environment: EnvironmentContainer, Launch: LaunchClaude, GhToken: &v, MountGH: &v}
			}(),
			wantErr: "mount_gh and gh_token are mutually exclusive",
		},
		{
			name:    "mount_container_sock with host environment",
			profile: func() Profile { v := true; return Profile{Environment: EnvironmentHost, Launch: LaunchShell, MountContainerSock: &v} }(),
			wantErr: "mount_container_sock is only valid with environment: container",
		},
		{
			name: "valid auth config",
			profile: Profile{
				Environment: EnvironmentContainer,
				Launch:      LaunchCodex,
				Auth: &AuthConfig{
					OnLaunch: &OnLaunchAuthConfig{Check: AuthOnLaunchCheckWarn},
					Codex: &CodexAuthConfig{
						LoginMode:        CodexLoginModeDevice,
						CredentialsStore: CodexCredentialsStoreFile,
						SeedFromHost:     AuthSeedFromHostIfMissing,
						PersistAuth:      AuthPersistModeStage,
					},
				},
			},
		},
		{
			name: "invalid auth on_launch check",
			profile: Profile{
				Environment: EnvironmentContainer,
				Launch:      LaunchCodex,
				Auth: &AuthConfig{
					OnLaunch: &OnLaunchAuthConfig{Check: "block"},
				},
			},
			wantErr: "unknown auth.on_launch.check",
		},
		{
			name: "invalid codex login mode",
			profile: Profile{
				Environment: EnvironmentContainer,
				Launch:      LaunchCodex,
				Auth: &AuthConfig{
					Codex: &CodexAuthConfig{LoginMode: "console"},
				},
			},
			wantErr: "unknown auth.codex.login_mode",
		},
		{
			name: "invalid claude login mode",
			profile: Profile{
				Environment: EnvironmentContainer,
				Launch:      LaunchClaude,
				Auth: &AuthConfig{
					Claude: &ClaudeAuthConfig{LoginMode: "device"},
				},
			},
			wantErr: "unknown auth.claude.login_mode",
		},
		{
			name: "valid export config",
			profile: Profile{
				Environment: EnvironmentContainer,
				Launch:      LaunchClaude,
				Export: &ExportConfig{
					Snapshot: true,
					Include: []ExportInclude{
						{Src: "./certs", Dst: "/usr/local/share/ca-certificates"},
					},
					Env: map[string]string{"FOO": "bar"},
				},
			},
		},
		{
			name: "export with host environment",
			profile: Profile{
				Environment: EnvironmentHost,
				Launch:      LaunchShell,
				Export:       &ExportConfig{Snapshot: true},
			},
			wantErr: "export is only valid with environment: container",
		},
		{
			name: "export include missing src",
			profile: Profile{
				Environment: EnvironmentContainer,
				Launch:      LaunchClaude,
				Export: &ExportConfig{
					Include: []ExportInclude{{Dst: "/home/agent/test"}},
				},
			},
			wantErr: "export.include[0]: src is required",
		},
		{
			name: "export include missing dst",
			profile: Profile{
				Environment: EnvironmentContainer,
				Launch:      LaunchClaude,
				Export: &ExportConfig{
					Include: []ExportInclude{{Src: "./test"}},
				},
			},
			wantErr: "export.include[0]: dst is required",
		},
		{
			name: "export include non-absolute dst",
			profile: Profile{
				Environment: EnvironmentContainer,
				Launch:      LaunchClaude,
				Export: &ExportConfig{
					Include: []ExportInclude{{Src: "./test", Dst: "relative/path"}},
				},
			},
			wantErr: "export.include[0]: dst must be an absolute path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.profile)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("Validate() unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Validate() expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Validate() error = %q, want containing %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr string
	}{
		{
			name: "valid config",
			config: Config{
				Default: "test",
				Profiles: map[string]Profile{
					"test": {
						Environment: EnvironmentContainer,
						Launch:      LaunchClaude,
					},
				},
			},
		},
		{
			name: "no profiles",
			config: Config{
				Profiles: map[string]Profile{},
			},
			wantErr: "no profiles defined",
		},
		{
			name: "default profile not found",
			config: Config{
				Default: "nonexistent",
				Profiles: map[string]Profile{
					"test": {
						Environment: EnvironmentContainer,
						Launch:      LaunchClaude,
					},
				},
			},
			wantErr: "default profile \"nonexistent\" not found",
		},
		{
			name: "invalid profile in config",
			config: Config{
				Profiles: map[string]Profile{
					"bad": {
						Environment: "invalid",
						Launch:      LaunchClaude,
					},
				},
			},
			wantErr: "config validation errors",
		},
		{
			name: "no default is ok",
			config: Config{
				Profiles: map[string]Profile{
					"test": {
						Environment: EnvironmentHost,
						Launch:      LaunchShell,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConfig(&tt.config)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("ValidateConfig() unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("ValidateConfig() expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("ValidateConfig() error = %q, want containing %q", err.Error(), tt.wantErr)
			}
		})
	}
}
