package profile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHasSensitiveFields_Empty(t *testing.T) {
	cfg := &Config{
		Profiles: map[string]Profile{
			"test": {
				Environment: EnvironmentContainer,
				Launch:      LaunchClaude,
			},
		},
	}

	fields := hasSensitiveFields(cfg)
	if len(fields) != 0 {
		t.Errorf("expected no sensitive fields, got %v", fields)
	}
}

func TestHasSensitiveFields_OnCreate(t *testing.T) {
	cfg := &Config{
		Profiles: map[string]Profile{
			"test": {
				Worktree: &WorktreeConfig{
					OnCreate: "echo hello",
				},
			},
		},
	}

	fields := hasSensitiveFields(cfg)
	if len(fields) != 1 {
		t.Fatalf("expected 1 sensitive field, got %d: %v", len(fields), fields)
	}
}

func TestHasSensitiveFields_Mounts(t *testing.T) {
	cfg := &Config{
		Profiles: map[string]Profile{
			"test": {
				Mounts: []CustomMount{
					{Source: "~/.aws", Target: "/aws"},
				},
			},
		},
	}

	fields := hasSensitiveFields(cfg)
	if len(fields) != 1 {
		t.Fatalf("expected 1 sensitive field, got %d: %v", len(fields), fields)
	}
}

func TestHasSensitiveFields_Env(t *testing.T) {
	cfg := &Config{
		Profiles: map[string]Profile{
			"test": {
				Env: map[string]string{"FOO": "bar"},
			},
		},
	}

	fields := hasSensitiveFields(cfg)
	if len(fields) != 1 {
		t.Fatalf("expected 1 sensitive field, got %d: %v", len(fields), fields)
	}
}

func TestHasSensitiveFields_Dockerfile(t *testing.T) {
	cfg := &Config{
		Profiles: map[string]Profile{
			"test": {
				Dockerfile: "Dockerfile.custom",
			},
		},
	}

	fields := hasSensitiveFields(cfg)
	if len(fields) != 1 {
		t.Fatalf("expected 1 sensitive field, got %d: %v", len(fields), fields)
	}
}

func TestHasSensitiveFields_Defaults(t *testing.T) {
	cfg := &Config{
		Defaults: ProfileDefaultsFromProfile(Profile{
			Env: map[string]string{"KEY": "val"},
		}),
		Profiles: map[string]Profile{
			"test": {},
		},
	}

	fields := hasSensitiveFields(cfg)
	if len(fields) != 1 {
		t.Fatalf("expected 1 sensitive field from defaults, got %d: %v", len(fields), fields)
	}
}

func TestTrustStore(t *testing.T) {
	tmpDir := t.TempDir()
	origGlobalConfigDir := globalConfigDir
	globalConfigDir = func() (string, error) { return tmpDir, nil }
	defer func() { globalConfigDir = origGlobalConfigDir }()

	configPath := "/fake/project/.aw.yml"
	data := []byte("test config content")

	// Initially not trusted
	trusted, err := isTrusted(configPath, data)
	if err != nil {
		t.Fatalf("isTrusted error: %v", err)
	}
	if trusted {
		t.Fatal("expected not trusted initially")
	}

	// Save trust
	if err := saveTrust(configPath, data); err != nil {
		t.Fatalf("saveTrust error: %v", err)
	}

	// Now trusted
	trusted, err = isTrusted(configPath, data)
	if err != nil {
		t.Fatalf("isTrusted error: %v", err)
	}
	if !trusted {
		t.Fatal("expected trusted after save")
	}

	// Different content is not trusted
	trusted, err = isTrusted(configPath, []byte("modified content"))
	if err != nil {
		t.Fatalf("isTrusted error: %v", err)
	}
	if trusted {
		t.Fatal("expected not trusted for different content")
	}

	// Verify trust file was created
	td, _ := trustDir()
	entries, _ := os.ReadDir(filepath.Join(td))
	if len(entries) != 1 {
		t.Fatalf("expected 1 trust file, got %d", len(entries))
	}
}

func TestCheckProjectTrust_NoSensitive(t *testing.T) {
	cfg := &Config{
		Profiles: map[string]Profile{
			"test": {
				Environment: EnvironmentContainer,
				Launch:      LaunchClaude,
			},
		},
	}

	result, err := CheckProjectTrust("/fake/path", []byte("data"), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != cfg {
		t.Fatal("expected same config returned when no sensitive fields")
	}
}

func TestCheckProjectTrust_Approved(t *testing.T) {
	tmpDir := t.TempDir()
	origGlobalConfigDir := globalConfigDir
	globalConfigDir = func() (string, error) { return tmpDir, nil }
	defer func() { globalConfigDir = origGlobalConfigDir }()

	origPrompt := promptTrust
	promptTrust = func(_ string, _ []string) bool { return true }
	defer func() { promptTrust = origPrompt }()

	cfg := &Config{
		Profiles: map[string]Profile{
			"test": {
				Worktree: &WorktreeConfig{OnCreate: "echo hi"},
			},
		},
	}

	data := []byte("config data")
	result, err := CheckProjectTrust("/fake/path", data, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should return original config
	if result.Profiles["test"].Worktree == nil || result.Profiles["test"].Worktree.OnCreate != "echo hi" {
		t.Fatal("expected on-create to be preserved when approved")
	}

	// Should be trusted now
	trusted, _ := isTrusted("/fake/path", data)
	if !trusted {
		t.Fatal("expected config to be trusted after approval")
	}
}

func TestCheckProjectTrust_Denied(t *testing.T) {
	tmpDir := t.TempDir()
	origGlobalConfigDir := globalConfigDir
	globalConfigDir = func() (string, error) { return tmpDir, nil }
	defer func() { globalConfigDir = origGlobalConfigDir }()

	origPrompt := promptTrust
	promptTrust = func(_ string, _ []string) bool { return false }
	defer func() { promptTrust = origPrompt }()

	cfg := &Config{
		Profiles: map[string]Profile{
			"test": {
				Environment: EnvironmentContainer,
				Launch:      LaunchClaude,
				Worktree:    &WorktreeConfig{OnCreate: "malicious command", Base: "origin/main"},
				Mounts:      []CustomMount{{Source: "~/.aws", Target: "/exfil"}},
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
	if p.Worktree != nil && p.Worktree.OnCreate != "" {
		t.Error("expected on-create to be stripped")
	}
	if len(p.Mounts) != 0 {
		t.Error("expected mounts to be stripped")
	}
	if len(p.Env) != 0 {
		t.Error("expected env to be stripped")
	}
	if p.Dockerfile != "" {
		t.Error("expected dockerfile to be stripped")
	}

	// Non-sensitive fields should be preserved
	if p.Environment != EnvironmentContainer {
		t.Error("expected environment to be preserved")
	}
	if p.Launch != LaunchClaude {
		t.Error("expected launch to be preserved")
	}
	if p.Worktree == nil || p.Worktree.Base != "origin/main" {
		t.Error("expected worktree.base to be preserved")
	}
}

func TestCheckProjectTrust_AlreadyTrusted(t *testing.T) {
	tmpDir := t.TempDir()
	origGlobalConfigDir := globalConfigDir
	globalConfigDir = func() (string, error) { return tmpDir, nil }
	defer func() { globalConfigDir = origGlobalConfigDir }()

	promptCalled := false
	origPrompt := promptTrust
	promptTrust = func(_ string, _ []string) bool {
		promptCalled = true
		return true
	}
	defer func() { promptTrust = origPrompt }()

	cfg := &Config{
		Profiles: map[string]Profile{
			"test": {
				Worktree: &WorktreeConfig{OnCreate: "echo hi"},
			},
		},
	}

	data := []byte("config data")
	// Pre-trust
	if err := saveTrust("/fake/path", data); err != nil {
		t.Fatalf("saveTrust error: %v", err)
	}

	result, err := CheckProjectTrust("/fake/path", data, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if promptCalled {
		t.Fatal("expected no prompt when already trusted")
	}
	if result.Profiles["test"].Worktree.OnCreate != "echo hi" {
		t.Fatal("expected config unchanged when already trusted")
	}
}

func TestStripSensitiveFields(t *testing.T) {
	cfg := &Config{
		Defaults: ProfileDefaultsFromProfile(Profile{
			Env:        map[string]string{"DEFAULT_KEY": "val"},
			Dockerfile: "Dockerfile.default",
		}),
		Profiles: map[string]Profile{
			"a": {
				Environment: EnvironmentContainer,
				Launch:      LaunchClaude,
				Worktree: &WorktreeConfig{
					Base:     "origin/dev",
					OnCreate: "echo attack",
					OnEnd:    "echo cleanup",
				},
				Mounts:     []CustomMount{{Source: "/secret", Target: "/data"}},
				Env:        map[string]string{"FOO": "bar"},
				Dockerfile: "Dockerfile.custom",
			},
		},
	}

	stripped := stripSensitiveFields(cfg)

	// Defaults sensitive fields stripped
	d := stripped.Defaults.AsProfile()
	if len(d.Env) != 0 {
		t.Error("expected defaults env stripped")
	}
	if d.Dockerfile != "" {
		t.Error("expected defaults dockerfile stripped")
	}

	// Profile sensitive fields stripped
	p := stripped.Profiles["a"]
	if p.Worktree.OnCreate != "" || p.Worktree.OnEnd != "" {
		t.Error("expected on-create/on-end stripped")
	}
	if len(p.Mounts) != 0 {
		t.Error("expected mounts stripped")
	}
	if len(p.Env) != 0 {
		t.Error("expected env stripped")
	}
	if p.Dockerfile != "" {
		t.Error("expected dockerfile stripped")
	}

	// Non-sensitive preserved
	if p.Environment != EnvironmentContainer {
		t.Error("expected environment preserved")
	}
	if p.Launch != LaunchClaude {
		t.Error("expected launch preserved")
	}
	if p.Worktree.Base != "origin/dev" {
		t.Error("expected worktree.base preserved")
	}

	// Original not mutated
	if cfg.Profiles["a"].Worktree.OnCreate != "echo attack" {
		t.Error("original config should not be mutated")
	}
}
