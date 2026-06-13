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
