package auth

import (
	"strings"
	"testing"

	"github.com/konono/aw/internal/profile"
)

func TestResolveActionCommand_DefaultCodexLogin(t *testing.T) {
	spec, err := resolveActionCommand(profile.Profile{
		Environment: profile.EnvironmentContainer,
		Launch:      profile.LaunchCodex,
	}, ActionLogin)
	if err != nil {
		t.Fatalf("resolveActionCommand() error: %v", err)
	}

	want := []string{"codex", "login", "--device-auth"}
	if strings.Join(spec.command, " ") != strings.Join(want, " ") {
		t.Fatalf("command = %v, want %v", spec.command, want)
	}
}

func TestResolveActionCommand_CodexDeviceLogin(t *testing.T) {
	spec, err := resolveActionCommand(profile.Profile{
		Environment: profile.EnvironmentContainer,
		Launch:      profile.LaunchCodex,
		Auth: &profile.AuthConfig{
			Codex: &profile.CodexAuthConfig{
				LoginMode: profile.CodexLoginModeDevice,
			},
		},
	}, ActionLogin)
	if err != nil {
		t.Fatalf("resolveActionCommand() error: %v", err)
	}

	want := []string{"codex", "login", "--device-auth"}
	if strings.Join(spec.command, " ") != strings.Join(want, " ") {
		t.Fatalf("command = %v, want %v", spec.command, want)
	}
}

func TestResolveActionCommand_CodexBrowserLogin(t *testing.T) {
	spec, err := resolveActionCommand(profile.Profile{
		Environment: profile.EnvironmentContainer,
		Launch:      profile.LaunchCodex,
		Auth: &profile.AuthConfig{
			Codex: &profile.CodexAuthConfig{
				LoginMode: profile.CodexLoginModeBrowser,
			},
		},
	}, ActionLogin)
	if err != nil {
		t.Fatalf("resolveActionCommand() error: %v", err)
	}

	want := []string{"codex", "login"}
	if strings.Join(spec.command, " ") != strings.Join(want, " ") {
		t.Fatalf("command = %v, want %v", spec.command, want)
	}
}

func TestResolveActionCommand_DefaultClaudeStatus(t *testing.T) {
	spec, err := resolveActionCommand(profile.Profile{
		Environment: profile.EnvironmentHost,
		Launch:      profile.LaunchClaude,
	}, ActionStatus)
	if err != nil {
		t.Fatalf("resolveActionCommand() error: %v", err)
	}

	want := []string{"claude", "auth", "status", "--text"}
	if strings.Join(spec.command, " ") != strings.Join(want, " ") {
		t.Fatalf("command = %v, want %v", spec.command, want)
	}
}

func TestResolveActionCommand_ClaudeExternalAuthWithoutManagedConfig(t *testing.T) {
	_, err := resolveActionCommand(profile.Profile{
		Environment: profile.EnvironmentContainer,
		Launch:      profile.LaunchClaude,
		Env: map[string]string{
			"CLAUDE_CODE_USE_VERTEX": "1",
		},
	}, ActionLogin)
	if err == nil {
		t.Fatal("expected error for external auth profile")
	}
	if !strings.Contains(err.Error(), "外部認証") {
		t.Fatalf("error = %q, want external auth hint", err.Error())
	}
}

func TestResolveActionCommand_OpenCodeLoginWithProviderAndMethod(t *testing.T) {
	spec, err := resolveActionCommand(profile.Profile{
		Environment: profile.EnvironmentContainer,
		Launch:      profile.LaunchOpenCode,
		Auth: &profile.AuthConfig{
			OpenCode: &profile.OpenCodeAuthConfig{
				Provider: "openai",
				Method:   "api-key",
			},
		},
	}, ActionLogin)
	if err != nil {
		t.Fatalf("resolveActionCommand() error: %v", err)
	}

	want := []string{"opencode", "auth", "login", "--provider", "openai", "--method", "api-key"}
	if strings.Join(spec.command, " ") != strings.Join(want, " ") {
		t.Fatalf("command = %v, want %v", spec.command, want)
	}
}

func TestResolveActionCommand_InvalidAction(t *testing.T) {
	// An invalid action should still return a command (the default branch in
	// buildXxxCommand falls back to a login-style command), so we verify it
	// does not return an error for a known tool.
	spec, err := resolveActionCommand(profile.Profile{
		Environment: profile.EnvironmentContainer,
		Launch:      profile.LaunchClaude,
	}, Action("invalid"))
	if err != nil {
		t.Fatalf("resolveActionCommand() unexpected error for invalid action: %v", err)
	}
	// The default case falls back to the login command.
	if len(spec.command) == 0 {
		t.Fatal("expected non-empty command for invalid action fallback")
	}
}

func TestResolveActionCommand_ShellLaunchReturnsError(t *testing.T) {
	_, err := resolveActionCommand(profile.Profile{
		Environment: profile.EnvironmentContainer,
		Launch:      profile.LaunchShell,
	}, ActionLogin)
	if err == nil {
		t.Fatal("expected error for shell profile (no tool)")
	}
}

func TestResolveActionCommand_LogoutCommands(t *testing.T) {
	tests := []struct {
		name   string
		launch profile.LaunchMode
		want   string
	}{
		{"claude logout", profile.LaunchClaude, "claude auth logout"},
		{"codex logout", profile.LaunchCodex, "codex logout"},
		{"opencode logout", profile.LaunchOpenCode, "opencode auth logout"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec, err := resolveActionCommand(profile.Profile{
				Environment: profile.EnvironmentContainer,
				Launch:      tt.launch,
			}, ActionLogout)
			if err != nil {
				t.Fatalf("resolveActionCommand() error: %v", err)
			}
			got := strings.Join(spec.command, " ")
			if got != tt.want {
				t.Fatalf("command = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveActionCommand_StatusCommands(t *testing.T) {
	tests := []struct {
		name   string
		launch profile.LaunchMode
		want   string
	}{
		{"claude status", profile.LaunchClaude, "claude auth status --text"},
		{"codex status", profile.LaunchCodex, "codex login status"},
		{"opencode status", profile.LaunchOpenCode, "opencode auth list"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec, err := resolveActionCommand(profile.Profile{
				Environment: profile.EnvironmentContainer,
				Launch:      tt.launch,
			}, ActionStatus)
			if err != nil {
				t.Fatalf("resolveActionCommand() error: %v", err)
			}
			got := strings.Join(spec.command, " ")
			if got != tt.want {
				t.Fatalf("command = %q, want %q", got, tt.want)
			}
		})
	}
}
