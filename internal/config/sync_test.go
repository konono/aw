package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSyncSettings_CopiesFiles(t *testing.T) {
	claudeHome := t.TempDir()
	containerHome := t.TempDir()

	// Create source files
	if err := os.WriteFile(filepath.Join(claudeHome, "settings.json"), []byte(`{"key":"value"}`), 0644); err != nil {
		t.Fatalf("writing settings.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(claudeHome, "CLAUDE.md"), []byte("# Instructions"), 0644); err != nil {
		t.Fatalf("writing CLAUDE.md: %v", err)
	}

	syncer := NewSyncer()
	if err := syncer.SyncToolSettings(claudeHome, containerHome, ClaudeSyncSpec); err != nil {
		t.Fatalf("SyncToolSettings() error: %v", err)
	}

	// Verify settings.json was patched
	content, err := os.ReadFile(filepath.Join(containerHome, "settings.json"))
	if err != nil {
		t.Fatalf("reading settings.json: %v", err)
	}
	var settings map[string]interface{}
	if err := json.Unmarshal(content, &settings); err != nil {
		t.Fatalf("parsing settings.json: %v", err)
	}
	if settings["key"] != "value" {
		t.Errorf("settings.json key = %v, want %q", settings["key"], "value")
	}
	if settings["skipDangerousModePermissionPrompt"] != true {
		t.Error("settings.json should have skipDangerousModePermissionPrompt: true")
	}

	content, err = os.ReadFile(filepath.Join(containerHome, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("reading CLAUDE.md: %v", err)
	}
	if string(content) != "# Instructions" {
		t.Errorf("CLAUDE.md = %q, want %q", string(content), "# Instructions")
	}
}

func TestSyncSettings_SkipsMissingFiles(t *testing.T) {
	claudeHome := t.TempDir()
	containerHome := t.TempDir()

	// Don't create any source files
	syncer := NewSyncer()
	if err := syncer.SyncToolSettings(claudeHome, containerHome, ClaudeSyncSpec); err != nil {
		t.Fatalf("SyncToolSettings() error: %v", err)
	}

	// settings.json should still be created with minimal content
	content, err := os.ReadFile(filepath.Join(containerHome, "settings.json"))
	if err != nil {
		t.Fatalf("settings.json should exist even without source: %v", err)
	}
	var settings map[string]interface{}
	if err := json.Unmarshal(content, &settings); err != nil {
		t.Fatalf("parsing settings.json: %v", err)
	}
	if settings["skipDangerousModePermissionPrompt"] != true {
		t.Error("settings.json should have skipDangerousModePermissionPrompt: true")
	}
}

func TestSyncSettings_CopiesDirectories(t *testing.T) {
	claudeHome := t.TempDir()
	containerHome := t.TempDir()

	// Create source directory with files
	hooksDir := filepath.Join(claudeHome, "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		t.Fatalf("creating hooks dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(hooksDir, "pre-commit.sh"), []byte("#!/bin/bash\necho hi"), 0755); err != nil {
		t.Fatalf("writing pre-commit.sh: %v", err)
	}

	syncer := NewSyncer()
	if err := syncer.SyncToolSettings(claudeHome, containerHome, ClaudeSyncSpec); err != nil {
		t.Fatalf("SyncToolSettings() error: %v", err)
	}

	// Verify directory and file were copied
	content, err := os.ReadFile(filepath.Join(containerHome, "hooks", "pre-commit.sh"))
	if err != nil {
		t.Fatalf("reading hooks/pre-commit.sh: %v", err)
	}
	if string(content) != "#!/bin/bash\necho hi" {
		t.Errorf("hooks/pre-commit.sh = %q, want %q", string(content), "#!/bin/bash\necho hi")
	}
}

func TestSyncSettings_ReplacesExistingDirectories(t *testing.T) {
	claudeHome := t.TempDir()
	containerHome := t.TempDir()

	// Create old content in container
	oldHooksDir := filepath.Join(containerHome, "hooks")
	if err := os.MkdirAll(oldHooksDir, 0755); err != nil {
		t.Fatalf("creating old hooks dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(oldHooksDir, "old-hook.sh"), []byte("old"), 0644); err != nil {
		t.Fatalf("writing old-hook.sh: %v", err)
	}

	// Create new content in source
	newHooksDir := filepath.Join(claudeHome, "hooks")
	if err := os.MkdirAll(newHooksDir, 0755); err != nil {
		t.Fatalf("creating new hooks dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(newHooksDir, "new-hook.sh"), []byte("new"), 0644); err != nil {
		t.Fatalf("writing new-hook.sh: %v", err)
	}

	syncer := NewSyncer()
	if err := syncer.SyncToolSettings(claudeHome, containerHome, ClaudeSyncSpec); err != nil {
		t.Fatalf("SyncToolSettings() error: %v", err)
	}

	// Old file should be gone
	if _, err := os.Stat(filepath.Join(containerHome, "hooks", "old-hook.sh")); !os.IsNotExist(err) {
		t.Error("old-hook.sh should have been removed")
	}

	// New file should exist
	content, err := os.ReadFile(filepath.Join(containerHome, "hooks", "new-hook.sh"))
	if err != nil {
		t.Fatalf("reading new-hook.sh: %v", err)
	}
	if string(content) != "new" {
		t.Errorf("new-hook.sh = %q, want %q", string(content), "new")
	}
}

func TestSyncSettings_SkipsMissingDirectories(t *testing.T) {
	claudeHome := t.TempDir()
	containerHome := t.TempDir()

	syncer := NewSyncer()
	if err := syncer.SyncToolSettings(claudeHome, containerHome, ClaudeSyncSpec); err != nil {
		t.Fatalf("SyncToolSettings() error: %v", err)
	}

	// Verify no directories were created
	for _, d := range ClaudeSyncSpec.Dirs {
		if _, err := os.Stat(filepath.Join(containerHome, d)); !os.IsNotExist(err) {
			t.Errorf("%s should not exist when source is missing", d)
		}
	}
}

func TestSyncSettings_CreatesContainerHome(t *testing.T) {
	claudeHome := t.TempDir()
	containerHome := filepath.Join(t.TempDir(), "nonexistent", "agent-workspace")

	syncer := NewSyncer()
	if err := syncer.SyncToolSettings(claudeHome, containerHome, ClaudeSyncSpec); err != nil {
		t.Fatalf("SyncToolSettings() error: %v", err)
	}

	info, err := os.Stat(containerHome)
	if err != nil {
		t.Fatalf("container home should exist: %v", err)
	}
	if !info.IsDir() {
		t.Error("container home should be a directory")
	}
}

func TestSyncSettings_NestedDirectories(t *testing.T) {
	claudeHome := t.TempDir()
	containerHome := t.TempDir()

	// Create nested directory structure
	nestedDir := filepath.Join(claudeHome, "plugins", "subdir")
	if err := os.MkdirAll(nestedDir, 0755); err != nil {
		t.Fatalf("creating nested dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nestedDir, "plugin.json"), []byte(`{}`), 0644); err != nil {
		t.Fatalf("writing plugin.json: %v", err)
	}

	syncer := NewSyncer()
	if err := syncer.SyncToolSettings(claudeHome, containerHome, ClaudeSyncSpec); err != nil {
		t.Fatalf("SyncToolSettings() error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(containerHome, "plugins", "subdir", "plugin.json"))
	if err != nil {
		t.Fatalf("reading nested file: %v", err)
	}
	if string(content) != `{}` {
		t.Errorf("plugin.json = %q, want %q", string(content), `{}`)
	}
}

func TestSyncCodexSettings_CopiesDirectory(t *testing.T) {
	codexHome := t.TempDir()
	containerHome := filepath.Join(t.TempDir(), "agent-workspace-codex")

	// Create source files
	if err := os.WriteFile(filepath.Join(codexHome, "config.toml"), []byte("[auth]\napi_key = \"test\""), 0644); err != nil {
		t.Fatalf("writing config.toml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(codexHome, "auth.json"), []byte(`{"token":"abc"}`), 0644); err != nil {
		t.Fatalf("writing auth.json: %v", err)
	}

	syncer := NewSyncer()
	if err := syncer.SyncToolSettings(codexHome, containerHome, CodexSyncSpec); err != nil {
		t.Fatalf("SyncToolSettings(CodexSyncSpec) error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(containerHome, "config.toml"))
	if err != nil {
		t.Fatalf("reading config.toml: %v", err)
	}
	if string(content) != "[auth]\napi_key = \"test\"\ncli_auth_credentials_store = \"file\"\n" {
		t.Errorf("config.toml = %q, want credentials_store patch", string(content))
	}

	content, err = os.ReadFile(filepath.Join(containerHome, "auth.json"))
	if err != nil {
		t.Fatalf("reading auth.json: %v", err)
	}
	if string(content) != `{"token":"abc"}` {
		t.Errorf("auth.json = %q, want %q", string(content), `{"token":"abc"}`)
	}
}

func TestSyncCodexSettings_NoSourceDir(t *testing.T) {
	containerHome := filepath.Join(t.TempDir(), "agent-workspace-codex")

	syncer := NewSyncer()
	if err := syncer.SyncToolSettings("/nonexistent/codex", containerHome, CodexSyncSpec); err != nil {
		t.Fatalf("SyncToolSettings(CodexSyncSpec) should not error for missing source: %v", err)
	}

	// Container dir should be created even if source doesn't exist
	info, err := os.Stat(containerHome)
	if err != nil {
		t.Fatalf("container codex home should exist: %v", err)
	}
	if !info.IsDir() {
		t.Error("container codex home should be a directory")
	}
}

func TestSyncCodexSettings_SeedAuthOnlyWhenMissing(t *testing.T) {
	codexHome := t.TempDir()
	containerHome := filepath.Join(t.TempDir(), "agent-workspace-codex")

	if err := os.WriteFile(filepath.Join(codexHome, "auth.json"), []byte(`{"token":"host"}`), 0644); err != nil {
		t.Fatalf("writing source auth.json: %v", err)
	}
	if err := os.MkdirAll(containerHome, 0755); err != nil {
		t.Fatalf("creating container home: %v", err)
	}
	if err := os.WriteFile(filepath.Join(containerHome, "auth.json"), []byte(`{"token":"stage"}`), 0644); err != nil {
		t.Fatalf("writing existing container auth.json: %v", err)
	}

	syncer := NewSyncer()
	if err := syncer.SyncToolSettings(codexHome, containerHome, CodexSyncSpecWithOptions("file", "if_missing")); err != nil {
		t.Fatalf("SyncToolSettings() error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(containerHome, "auth.json"))
	if err != nil {
		t.Fatalf("reading auth.json: %v", err)
	}
	if string(content) != `{"token":"stage"}` {
		t.Errorf("auth.json = %q, want existing stage copy to be preserved", string(content))
	}
}

func TestSyncCodexSettings_AlwaysCopiesAuthWhenConfigured(t *testing.T) {
	codexHome := t.TempDir()
	containerHome := filepath.Join(t.TempDir(), "agent-workspace-codex")

	if err := os.WriteFile(filepath.Join(codexHome, "auth.json"), []byte(`{"token":"host"}`), 0644); err != nil {
		t.Fatalf("writing source auth.json: %v", err)
	}
	if err := os.MkdirAll(containerHome, 0755); err != nil {
		t.Fatalf("creating container home: %v", err)
	}
	if err := os.WriteFile(filepath.Join(containerHome, "auth.json"), []byte(`{"token":"stage"}`), 0644); err != nil {
		t.Fatalf("writing existing container auth.json: %v", err)
	}

	syncer := NewSyncer()
	if err := syncer.SyncToolSettings(codexHome, containerHome, CodexSyncSpecWithOptions("file", "always")); err != nil {
		t.Fatalf("SyncToolSettings() error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(containerHome, "auth.json"))
	if err != nil {
		t.Fatalf("reading auth.json: %v", err)
	}
	if string(content) != `{"token":"host"}` {
		t.Errorf("auth.json = %q, want host copy to replace stage copy", string(content))
	}
}

func TestPatchSettingsForContainer(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		check   func(t *testing.T, settings map[string]interface{})
	}{
		{
			name:  "adds skip permission to existing settings",
			input: `{"model":"claude-opus-4-6"}`,
			check: func(t *testing.T, s map[string]interface{}) {
				if s["skipDangerousModePermissionPrompt"] != true {
					t.Error("missing skipDangerousModePermissionPrompt")
				}
				if s["model"] != "claude-opus-4-6" {
					t.Errorf("model = %v, want claude-opus-4-6", s["model"])
				}
			},
		},
		{
			name:  "adds skip permission to empty object",
			input: `{}`,
			check: func(t *testing.T, s map[string]interface{}) {
				if s["skipDangerousModePermissionPrompt"] != true {
					t.Error("missing skipDangerousModePermissionPrompt")
				}
				if len(s) != 1 {
					t.Errorf("expected 1 key, got %d", len(s))
				}
			},
		},
		{
			name:  "preserves all existing fields",
			input: `{"hooks":{"Stop":[]},"statusLine":{"type":"command"},"model":"opus"}`,
			check: func(t *testing.T, s map[string]interface{}) {
				if s["skipDangerousModePermissionPrompt"] != true {
					t.Error("missing skipDangerousModePermissionPrompt")
				}
				if s["hooks"] == nil {
					t.Error("hooks should be preserved")
				}
				if s["statusLine"] == nil {
					t.Error("statusLine should be preserved")
				}
			},
		},
		{
			name:    "returns error for invalid JSON",
			input:   `not json`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := patchSettingsForContainer([]byte(tt.input))
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			var settings map[string]interface{}
			if err := json.Unmarshal(out, &settings); err != nil {
				t.Fatalf("output is not valid JSON: %v", err)
			}
			tt.check(t, settings)
		})
	}
}
