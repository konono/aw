package profile

import (
	"os"
	"path/filepath"
	"testing"
)

// TestProjectConfig_SensitiveFieldsRequireTrust verifies that configs with
// sensitive fields trigger a trust prompt, while safe-only configs do not.
func TestProjectConfig_SensitiveFieldsRequireTrust(t *testing.T) {
	tests := []struct {
		name          string
		cfg           *Config
		expectPrompt  bool
	}{
		{
			name: "safe fields only — no prompt",
			cfg: &Config{
				Profiles: map[string]Profile{
					"test": {Environment: EnvironmentContainer, Launch: LaunchClaude},
				},
			},
		},
		{
			name: "mounts trigger prompt",
			cfg: &Config{
				Profiles: map[string]Profile{
					"test": {Mounts: []CustomMount{{Source: "~/.aws", Target: "/aws"}}},
				},
			},
			expectPrompt: true,
		},
		{
			name: "env vars trigger prompt",
			cfg: &Config{
				Profiles: map[string]Profile{
					"test": {Env: map[string]string{"SECRET": "val"}},
				},
			},
			expectPrompt: true,
		},
		{
			name: "dockerfile triggers prompt",
			cfg: &Config{
				Profiles: map[string]Profile{
					"test": {Dockerfile: "Dockerfile.custom"},
				},
			},
			expectPrompt: true,
		},
		{
			name: "image triggers prompt",
			cfg: &Config{
				Profiles: map[string]Profile{
					"test": {Image: "malicious:latest"},
				},
			},
			expectPrompt: true,
		},
		{
			name: "worktree on-create triggers prompt",
			cfg: &Config{
				Profiles: map[string]Profile{
					"test": {Worktree: &WorktreeConfig{OnCreate: "echo hello"}},
				},
			},
			expectPrompt: true,
		},
		{
			name: "worktree on-end triggers prompt",
			cfg: &Config{
				Profiles: map[string]Profile{
					"test": {Worktree: &WorktreeConfig{OnEnd: "echo bye"}},
				},
			},
			expectPrompt: true,
		},
		{
			name: "packages trigger prompt",
			cfg: &Config{
				Profiles: map[string]Profile{
					"test": {Packages: []string{"jq", "tree"}},
				},
			},
			expectPrompt: true,
		},
		{
			name: "sensitive fields in defaults trigger prompt",
			cfg: &Config{
				Defaults: ProfileDefaultsFromProfile(Profile{
					Env: map[string]string{"KEY": "val"},
				}),
				Profiles: map[string]Profile{"test": {}},
			},
			expectPrompt: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("AW_TRUST_PROJECT", "")

			tmpDir := t.TempDir()
			origDir := globalConfigDir
			globalConfigDir = func() (string, error) { return tmpDir, nil }
			defer func() { globalConfigDir = origDir }()

			prompted := false
			origPrompt := promptTrust
			promptTrust = func(_ string, _ []string) bool {
				prompted = true
				return true
			}
			defer func() { promptTrust = origPrompt }()

			_, err := CheckProjectTrust("/fake/path", []byte("data"), tt.cfg)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if prompted != tt.expectPrompt {
				t.Errorf("prompt triggered = %v, want %v", prompted, tt.expectPrompt)
			}
		})
	}
}

// TestProjectConfig_DeniedStripsUnsafePreservesSafe verifies that when a user
// denies trust, all sensitive fields are stripped but safe fields remain.
func TestProjectConfig_DeniedStripsUnsafePreservesSafe(t *testing.T) {
	t.Setenv("AW_TRUST_PROJECT", "")

	tmpDir := t.TempDir()
	origDir := globalConfigDir
	globalConfigDir = func() (string, error) { return tmpDir, nil }
	defer func() { globalConfigDir = origDir }()

	origPrompt := promptTrust
	promptTrust = func(_ string, _ []string) bool { return false }
	defer func() { promptTrust = origPrompt }()

	cfg := &Config{
		Defaults: ProfileDefaultsFromProfile(Profile{
			Env:        map[string]string{"DEFAULT_KEY": "val"},
			Dockerfile: "Dockerfile.default",
		}),
		Profiles: map[string]Profile{
			"test": {
				Environment: EnvironmentContainer,
				Launch:      LaunchClaude,
				Worktree:    &WorktreeConfig{Base: "origin/main", OnCreate: "malicious", OnEnd: "cleanup"},
				Mounts:      []CustomMount{{Source: "/secret", Target: "/data"}},
				Packages:    []string{"evil-pkg"},
				Env:         map[string]string{"EVIL": "true"},
				Dockerfile:  "Dockerfile.evil",
			},
		},
	}

	result, err := CheckProjectTrust("/fake/path", []byte("data"), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	p := result.Profiles["test"]

	// Sensitive fields stripped
	if p.Worktree != nil && p.Worktree.OnCreate != "" {
		t.Error("on-create should be stripped")
	}
	if p.Worktree != nil && p.Worktree.OnEnd != "" {
		t.Error("on-end should be stripped")
	}
	if len(p.Mounts) != 0 {
		t.Error("mounts should be stripped")
	}
	if len(p.Env) != 0 {
		t.Error("env should be stripped")
	}
	if p.Dockerfile != "" {
		t.Error("dockerfile should be stripped")
	}
	if len(p.Packages) != 0 {
		t.Error("packages should be stripped")
	}

	// Defaults sensitive fields stripped
	d := result.Defaults.AsProfile()
	if len(d.Env) != 0 {
		t.Error("defaults env should be stripped")
	}
	if d.Dockerfile != "" {
		t.Error("defaults dockerfile should be stripped")
	}

	// Safe fields preserved
	if p.Environment != EnvironmentContainer {
		t.Error("environment should be preserved")
	}
	if p.Launch != LaunchClaude {
		t.Error("launch should be preserved")
	}
	if p.Worktree == nil || p.Worktree.Base != "origin/main" {
		t.Error("worktree.base should be preserved")
	}

	// Original not mutated
	if cfg.Profiles["test"].Worktree.OnCreate != "malicious" {
		t.Error("original config should not be mutated")
	}
}

func TestTrustStore(t *testing.T) {
	tmpDir := t.TempDir()
	origGlobalConfigDir := globalConfigDir
	globalConfigDir = func() (string, error) { return tmpDir, nil }
	defer func() { globalConfigDir = origGlobalConfigDir }()

	configPath := "/fake/project/.aw.yml"
	data := []byte("test config content")

	trusted, err := isTrusted(configPath, data)
	if err != nil {
		t.Fatalf("isTrusted error: %v", err)
	}
	if trusted {
		t.Fatal("expected not trusted initially")
	}

	if err := saveTrust(configPath, data); err != nil {
		t.Fatalf("saveTrust error: %v", err)
	}

	trusted, err = isTrusted(configPath, data)
	if err != nil {
		t.Fatalf("isTrusted error: %v", err)
	}
	if !trusted {
		t.Fatal("expected trusted after save")
	}

	trusted, err = isTrusted(configPath, []byte("modified content"))
	if err != nil {
		t.Fatalf("isTrusted error: %v", err)
	}
	if trusted {
		t.Fatal("expected not trusted for different content")
	}

	td, _ := trustDir()
	entries, _ := os.ReadDir(filepath.Join(td))
	if len(entries) != 1 {
		t.Fatalf("expected 1 trust file, got %d", len(entries))
	}
}

func TestCheckProjectTrust_ApprovedSavesTrust(t *testing.T) {
	tmpDir := t.TempDir()
	origDir := globalConfigDir
	globalConfigDir = func() (string, error) { return tmpDir, nil }
	defer func() { globalConfigDir = origDir }()

	origPrompt := promptTrust
	promptTrust = func(_ string, _ []string) bool { return true }
	defer func() { promptTrust = origPrompt }()

	cfg := &Config{
		Profiles: map[string]Profile{
			"test": {Worktree: &WorktreeConfig{OnCreate: "echo hi"}},
		},
	}

	data := []byte("config data")
	result, err := CheckProjectTrust("/fake/path", data, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Profiles["test"].Worktree.OnCreate != "echo hi" {
		t.Error("approved config should preserve all fields")
	}

	trusted, _ := isTrusted("/fake/path", data)
	if !trusted {
		t.Error("config should be trusted after approval")
	}
}

func TestCheckProjectTrust_EnvVarAutoTrusts(t *testing.T) {
	tmpDir := t.TempDir()
	origDir := globalConfigDir
	globalConfigDir = func() (string, error) { return tmpDir, nil }
	defer func() { globalConfigDir = origDir }()

	promptCalled := false
	origPrompt := promptTrust
	promptTrust = func(_ string, _ []string) bool {
		promptCalled = true
		return false
	}
	defer func() { promptTrust = origPrompt }()

	t.Setenv("AW_TRUST_PROJECT", "1")

	cfg := &Config{
		Profiles: map[string]Profile{
			"test": {Worktree: &WorktreeConfig{OnCreate: "echo hi"}},
		},
	}

	data := []byte("env trust data")
	result, err := CheckProjectTrust("/fake/env-path", data, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if promptCalled {
		t.Error("should not prompt when AW_TRUST_PROJECT is set")
	}
	if result.Profiles["test"].Worktree.OnCreate != "echo hi" {
		t.Error("auto-trusted config should preserve all fields")
	}

	trusted, _ := isTrusted("/fake/env-path", data)
	if !trusted {
		t.Error("config should be persisted as trusted after AW_TRUST_PROJECT")
	}
}

func TestCheckProjectTrust_EnvVarFalseValuesDoNotTrust(t *testing.T) {
	for _, val := range []string{"0", "false", "no", ""} {
		t.Run("AW_TRUST_PROJECT="+val, func(t *testing.T) {
			tmpDir := t.TempDir()
			origDir := globalConfigDir
			globalConfigDir = func() (string, error) { return tmpDir, nil }
			defer func() { globalConfigDir = origDir }()

			promptCalled := false
			origPrompt := promptTrust
			promptTrust = func(_ string, _ []string) bool {
				promptCalled = true
				return false
			}
			defer func() { promptTrust = origPrompt }()

			t.Setenv("AW_TRUST_PROJECT", val)

			cfg := &Config{
				Profiles: map[string]Profile{
					"test": {Worktree: &WorktreeConfig{OnCreate: "echo hi"}},
				},
			}

			_, _ = CheckProjectTrust("/fake/false-path", []byte("false test"), cfg)
			if !promptCalled {
				t.Errorf("AW_TRUST_PROJECT=%q should still trigger prompt", val)
			}
		})
	}
}

func TestCheckProjectTrust_AlreadyTrustedSkipsPrompt(t *testing.T) {
	tmpDir := t.TempDir()
	origDir := globalConfigDir
	globalConfigDir = func() (string, error) { return tmpDir, nil }
	defer func() { globalConfigDir = origDir }()

	promptCalled := false
	origPrompt := promptTrust
	promptTrust = func(_ string, _ []string) bool {
		promptCalled = true
		return true
	}
	defer func() { promptTrust = origPrompt }()

	cfg := &Config{
		Profiles: map[string]Profile{
			"test": {Worktree: &WorktreeConfig{OnCreate: "echo hi"}},
		},
	}

	data := []byte("config data")
	if err := saveTrust("/fake/path", data); err != nil {
		t.Fatalf("saveTrust error: %v", err)
	}

	result, err := CheckProjectTrust("/fake/path", data, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if promptCalled {
		t.Error("should not prompt when already trusted")
	}
	if result.Profiles["test"].Worktree.OnCreate != "echo hi" {
		t.Error("trusted config should preserve all fields")
	}
}
