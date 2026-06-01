package profile

import "testing"

func TestWorktreeConfig_EffectiveBase(t *testing.T) {
	tests := []struct {
		name string
		base string
		want string
	}{
		{"empty defaults to origin/main", "", "origin/main"},
		{"custom base", "origin/develop", "origin/develop"},
		{"specific commit", "abc123", "abc123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &WorktreeConfig{Base: tt.base}
			if got := w.EffectiveBase(); got != tt.want {
				t.Errorf("EffectiveBase() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestProfile_EffectiveOS(t *testing.T) {
	tests := []struct {
		name string
		os   OSTemplate
		want OSTemplate
	}{
		{"empty defaults to debian12", "", OSDebian12},
		{"explicit debian12", OSDebian12, OSDebian12},
		{"ubi9", OSUBI9, OSUBI9},
		{"ubi10", OSUBI10, OSUBI10},
		{"ubuntu2604", OSUbuntu2604, OSUbuntu2604},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Profile{OS: tt.os}
			if got := p.EffectiveOS(); got != tt.want {
				t.Errorf("EffectiveOS() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestProfile_EffectiveTool(t *testing.T) {
	tests := []struct {
		name    string
		profile Profile
		want    string
	}{
		{
			name:    "launch claude returns claude",
			profile: Profile{Launch: LaunchClaude},
			want:    "claude",
		},
		{
			name:    "launch codex returns codex",
			profile: Profile{Launch: LaunchCodex},
			want:    "codex",
		},
		{
			name:    "launch shell returns empty",
			profile: Profile{Launch: LaunchShell},
			want:    "",
		},
		{
			name:    "launch zellij defaults to claude",
			profile: Profile{Launch: LaunchZellij, Zellij: &ZellijConfig{Layout: "default"}},
			want:    "claude",
		},
		{
			name:    "launch zellij with tool codex",
			profile: Profile{Launch: LaunchZellij, Zellij: &ZellijConfig{Layout: "default", Tool: "codex"}},
			want:    "codex",
		},
		{
			name:    "launch zellij with tool claude",
			profile: Profile{Launch: LaunchZellij, Zellij: &ZellijConfig{Layout: "default", Tool: "claude"}},
			want:    "claude",
		},
		{
			name:    "launch zellij nil config defaults to claude",
			profile: Profile{Launch: LaunchZellij},
			want:    "claude",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.profile.EffectiveTool(); got != tt.want {
				t.Errorf("EffectiveTool() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestProfile_EffectiveMountContainerSock(t *testing.T) {
	tests := []struct {
		name    string
		profile *Profile
		want    bool
	}{
		{
			name:    "nil profile defaults to false",
			profile: nil,
			want:    false,
		},
		{
			name:    "unset defaults to false",
			profile: &Profile{},
			want:    false,
		},
		{
			name:    "explicit false stays false",
			profile: &Profile{MountContainerSock: boolPtr(false)},
			want:    false,
		},
		{
			name:    "explicit true enables container sock mount",
			profile: &Profile{MountContainerSock: boolPtr(true)},
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.profile.EffectiveMountContainerSock(); got != tt.want {
				t.Errorf("EffectiveMountContainerSock() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProfile_EffectiveMountSSH(t *testing.T) {
	tests := []struct {
		name    string
		profile *Profile
		want    bool
	}{
		{
			name:    "nil profile defaults to false",
			profile: nil,
			want:    false,
		},
		{
			name:    "unset defaults to false",
			profile: &Profile{},
			want:    false,
		},
		{
			name:    "explicit false stays false",
			profile: &Profile{MountSSH: boolPtr(false)},
			want:    false,
		},
		{
			name:    "explicit true enables ssh mount",
			profile: &Profile{MountSSH: boolPtr(true)},
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.profile.EffectiveMountSSH(); got != tt.want {
				t.Errorf("EffectiveMountSSH() = %v, want %v", got, tt.want)
			}
		})
	}
}
