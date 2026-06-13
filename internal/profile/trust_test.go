package profile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProjectConfig_SafeFieldsLoadWithoutTrustPrompt(t *testing.T) {
	tmpDir := t.TempDir()
	origGlobalConfigDir := globalConfigDir
	globalConfigDir = func() (string, error) { return tmpDir, nil }
	defer func() { globalConfigDir = origGlobalConfigDir }()

	promptCalled := false
	origPrompt := promptTrust
	promptTrust = func(_ string, _ []string) bool {
		promptCalled = true
		return false
	}
	defer func() { promptTrust = origPrompt }()

	cfg := &Config{
		Profiles: map[string]Profile{
			"test": {
				Environment: EnvironmentContainer,
				Launch:      LaunchClaude,
				OS:          OSDebian12,
			},
		},
	}

	result, err := CheckProjectTrust("/fake/project/.aw.yml", []byte("safe config"), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if promptCalled {
		t.Error("expected no trust prompt for config with only safe fields")
	}
	if result != cfg {
		t.Error("expected original config returned unchanged")
	}
}

func TestProjectConfig_MountsRequireTrust(t *testing.T) {
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
				Mounts: []CustomMount{
					{Source: "~/.aws", Target: "/aws"},
				},
			},
		},
	}

	_, err := CheckProjectTrust("/fake/project/.aw.yml", []byte("mounts config"), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !promptCalled {
		t.Error("expected trust prompt when config contains mounts")
	}
}

func TestProjectConfig_EnvRequiresTrust(t *testing.T) {
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
				Env: map[string]string{"SECRET": "value"},
			},
		},
	}

	_, err := CheckProjectTrust("/fake/project/.aw.yml", []byte("env config"), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !promptCalled {
		t.Error("expected trust prompt when config contains env vars")
	}
}

func TestProjectConfig_DockerfileRequiresTrust(t *testing.T) {
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
				Dockerfile: "Dockerfile.custom",
			},
		},
	}

	_, err := CheckProjectTrust("/fake/project/.aw.yml", []byte("dockerfile config"), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !promptCalled {
		t.Error("expected trust prompt when config contains dockerfile")
	}
}

func TestProjectConfig_WorktreeOnCreateRequiresTrust(t *testing.T) {
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
				Worktree: &WorktreeConfig{
					OnCreate: "npm install",
				},
			},
		},
	}

	_, err := CheckProjectTrust("/fake/project/.aw.yml", []byte("worktree on-create config"), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !promptCalled {
		t.Error("expected trust prompt when config contains worktree on-create")
	}
}

func TestProjectConfig_WorktreeOnEndRequiresTrust(t *testing.T) {
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
				Worktree: &WorktreeConfig{
					OnEnd: "cleanup.sh",
				},
			},
		},
	}

	_, err := CheckProjectTrust("/fake/project/.aw.yml", []byte("worktree on-end config"), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !promptCalled {
		t.Error("expected trust prompt when config contains worktree on-end")
	}
}

func TestProjectConfig_DeniedStripsAllSensitiveKeepsSafe(t *testing.T) {
	tmpDir := t.TempDir()
	origGlobalConfigDir := globalConfigDir
	globalConfigDir = func() (string, error) { return tmpDir, nil }
	defer func() { globalConfigDir = origGlobalConfigDir }()

	origPrompt := promptTrust
	promptTrust = func(_ string, _ []string) bool { return false }
	defer func() { promptTrust = origPrompt }()

	cfg := &Config{
		Defaults: ProfileDefaultsFromProfile(Profile{
			Env:        map[string]string{"DEFAULT_KEY": "val"},
			Dockerfile: "Dockerfile.default",
		}),
		Profiles: map[string]Profile{
			"dev": {
				Environment: EnvironmentContainer,
				Launch:      LaunchClaude,
				OS:          OSDebian12,
				Worktree: &WorktreeConfig{
					Base:     "origin/dev",
					OnCreate: "npm install",
					OnEnd:    "cleanup.sh",
				},
				Mounts:     []CustomMount{{Source: "/secret", Target: "/data"}},
				Env:        map[string]string{"API_KEY": "secret"},
				Dockerfile: "Dockerfile.custom",
				Image:      "custom:latest",
			},
		},
	}

	result, err := CheckProjectTrust("/fake/project/.aw.yml", []byte("data"), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// All sensitive fields should be stripped from profile
	p := result.Profiles["dev"]
	if p.Worktree != nil && p.Worktree.OnCreate != "" {
		t.Error("expected worktree on-create stripped")
	}
	if p.Worktree != nil && p.Worktree.OnEnd != "" {
		t.Error("expected worktree on-end stripped")
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
	if p.Image != "" {
		t.Error("expected image stripped")
	}

	// Sensitive fields should be stripped from defaults too
	d := result.Defaults.AsProfile()
	if len(d.Env) != 0 {
		t.Error("expected defaults env stripped")
	}
	if d.Dockerfile != "" {
		t.Error("expected defaults dockerfile stripped")
	}

	// Safe fields should remain
	if p.Environment != EnvironmentContainer {
		t.Error("expected environment preserved")
	}
	if p.Launch != LaunchClaude {
		t.Error("expected launch preserved")
	}
	if p.OS != OSDebian12 {
		t.Error("expected os preserved")
	}
	if p.Worktree == nil || p.Worktree.Base != "origin/dev" {
		t.Error("expected worktree.base preserved (it is not sensitive)")
	}
}

func TestProjectConfig_ApprovedPreservesAllFields(t *testing.T) {
	tmpDir := t.TempDir()
	origGlobalConfigDir := globalConfigDir
	globalConfigDir = func() (string, error) { return tmpDir, nil }
	defer func() { globalConfigDir = origGlobalConfigDir }()

	origPrompt := promptTrust
	promptTrust = func(_ string, _ []string) bool { return true }
	defer func() { promptTrust = origPrompt }()

	cfg := &Config{
		Profiles: map[string]Profile{
			"dev": {
				Environment: EnvironmentContainer,
				Launch:      LaunchClaude,
				Worktree: &WorktreeConfig{
					Base:     "origin/dev",
					OnCreate: "npm install",
					OnEnd:    "cleanup.sh",
				},
				Mounts:     []CustomMount{{Source: "~/.aws", Target: "/aws"}},
				Env:        map[string]string{"API_KEY": "secret"},
				Dockerfile: "Dockerfile.custom",
			},
		},
	}

	result, err := CheckProjectTrust("/fake/project/.aw.yml", []byte("data"), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	p := result.Profiles["dev"]
	if p.Worktree == nil || p.Worktree.OnCreate != "npm install" {
		t.Error("expected worktree on-create preserved when approved")
	}
	if p.Worktree == nil || p.Worktree.OnEnd != "cleanup.sh" {
		t.Error("expected worktree on-end preserved when approved")
	}
	if len(p.Mounts) != 1 {
		t.Error("expected mounts preserved when approved")
	}
	if p.Env["API_KEY"] != "secret" {
		t.Error("expected env preserved when approved")
	}
	if p.Dockerfile != "Dockerfile.custom" {
		t.Error("expected dockerfile preserved when approved")
	}
	if p.Environment != EnvironmentContainer {
		t.Error("expected environment preserved when approved")
	}
	if p.Launch != LaunchClaude {
		t.Error("expected launch preserved when approved")
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

