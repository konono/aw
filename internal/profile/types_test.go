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
