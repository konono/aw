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
			wantErr: "dockerfile is only valid with environment: docker",
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
