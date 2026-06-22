package config

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Patcher transforms file content during sync (e.g. patching settings.json).
type Patcher func([]byte) ([]byte, error)

// ToolSyncSpec defines which files/dirs to copy from the host tool config.
// Files/dirs NOT listed here are preserved in the staging dir (session data etc.)
type ToolSyncSpec struct {
	Files     []string           // individual files to sync on every run
	SeedFiles []string           // files copied only when the destination is missing
	Dirs      []string           // directories to sync (remove & re-copy)
	Patch     map[string]Patcher // file-specific patches
}

// ClaudeSyncSpec syncs Claude Code settings while preserving session data.
var ClaudeSyncSpec = ToolSyncSpec{
	Files: []string{"settings.json", "CLAUDE.md"},
	Dirs:  []string{"hooks", "plugins", "commands", "agents", "skills"},
	Patch: map[string]Patcher{
		"settings.json": patchSettingsForContainer,
	},
}

// CodexSyncSpec syncs Codex config while preserving session history.
var CodexSyncSpec = ToolSyncSpec{
	Files:     []string{"config.toml", "AGENTS.md", "AGENTS.override.md"},
	SeedFiles: []string{"auth.json"},
	Dirs:      []string{"rules", "themes"},
	Patch: map[string]Patcher{
		"config.toml": patchCodexConfigForContainer("file"),
	},
}

// OpenCodeSyncSpec syncs OpenCode config while preserving session data.
var OpenCodeSyncSpec = ToolSyncSpec{
	Files: []string{"opencode.json", "opencode.jsonc", "tui.json", "AGENTS.md"},
	Dirs:  []string{"agents", "commands", "plugins", "skills", "tools", "themes", "modes"},
}

// CursorSyncSpec syncs Cursor CLI config while preserving session data.
// auth.json is seeded (not overwritten) so in-container login persists.
var CursorSyncSpec = ToolSyncSpec{
	Files:     []string{"cli-config.json"},
	SeedFiles: []string{"auth.json"},
}

// Syncer syncs host AI tool settings to the container-side staging directory.
type Syncer interface {
	SyncToolSettings(srcDir, dstDir string, spec ToolSyncSpec) error
	EnsureOnboardingState(path string) error
}

// DefaultSyncer is the default implementation of Syncer.
type DefaultSyncer struct{}

// NewSyncer creates a new DefaultSyncer.
func NewSyncer() *DefaultSyncer {
	return &DefaultSyncer{}
}

// SyncToolSettings copies specified files and directories from srcDir to dstDir.
// Files/dirs not in the spec are left untouched in dstDir (preserving session data).
func (s *DefaultSyncer) SyncToolSettings(srcDir, dstDir string, spec ToolSyncSpec) error {
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return fmt.Errorf("creating staging dir: %w", err)
	}

	for _, f := range spec.Files {
		src := filepath.Join(srcDir, f)
		dst := filepath.Join(dstDir, f)
		if patcher, ok := spec.Patch[f]; ok {
			if err := syncPatchedFile(src, dst, patcher); err != nil {
				return fmt.Errorf("syncing file %s: %w", f, err)
			}
		} else {
			if err := copyFileIfExists(src, dst); err != nil {
				return fmt.Errorf("syncing file %s: %w", f, err)
			}
		}
	}

	for _, f := range spec.SeedFiles {
		src := filepath.Join(srcDir, f)
		dst := filepath.Join(dstDir, f)
		if err := copyFileIfMissing(src, dst); err != nil {
			return fmt.Errorf("seeding file %s: %w", f, err)
		}
	}

	for _, d := range spec.Dirs {
		src := filepath.Join(srcDir, d)
		dst := filepath.Join(dstDir, d)
		if err := syncDirOrRemove(src, dst); err != nil {
			return fmt.Errorf("syncing directory %s: %w", d, err)
		}
	}

	return nil
}

func copyFileIfExists(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer func() { _ = srcFile.Close() }()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer func() { _ = dstFile.Close() }()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

func copyFileIfMissing(src, dst string) error {
	if _, err := os.Stat(dst); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	return copyFileIfExists(src, dst)
}

// syncDirOrRemove syncs a directory from src to dst. If src does not exist,
// dst is removed to prevent stale content (e.g. hooks injected by a
// compromised container) from persisting across runs.
func syncDirOrRemove(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		if os.IsNotExist(err) {
			_ = os.RemoveAll(dst)
			return nil
		}
		return err
	}
	if !info.IsDir() {
		_ = os.RemoveAll(dst)
		return nil
	}

	if err := os.RemoveAll(dst); err != nil {
		return fmt.Errorf("removing old %s: %w", dst, err)
	}

	return copyDir(src, dst)
}

func syncPatchedFile(src, dst string, patcher Patcher) error {
	data, err := os.ReadFile(src)
	if err != nil {
		if os.IsNotExist(err) {
			data = nil
		} else {
			return err
		}
	}

	patched, err := patcher(data)
	if err != nil {
		return fmt.Errorf("patching file: %w", err)
	}

	return os.WriteFile(dst, patched, 0644)
}

func patchSettingsForContainer(data []byte) ([]byte, error) {
	if len(strings.TrimSpace(string(data))) == 0 {
		data = []byte("{}")
	}
	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, err
	}

	settings["skipDangerousModePermissionPrompt"] = true

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}

// CodexSyncSpecWithOptions returns a Codex sync spec tuned for container auth.
func CodexSyncSpecWithOptions(credentialsStore, seedFromHost string) ToolSyncSpec {
	if credentialsStore == "" {
		credentialsStore = "file"
	}
	if seedFromHost == "" {
		seedFromHost = "if_missing"
	}

	spec := ToolSyncSpec{
		Files: []string{"config.toml", "AGENTS.md", "AGENTS.override.md"},
		Dirs:  []string{"rules", "themes"},
		Patch: map[string]Patcher{
			"config.toml": patchCodexConfigForContainer(credentialsStore),
		},
	}

	switch seedFromHost {
	case "always":
		spec.Files = append(spec.Files, "auth.json")
	case "never":
		// Keep stage-only auth state.
	default:
		spec.SeedFiles = []string{"auth.json"}
	}

	return spec
}

func patchCodexConfigForContainer(credentialsStore string) Patcher {
	return func(data []byte) ([]byte, error) {
		if credentialsStore == "" {
			credentialsStore = "file"
		}
		content := strings.ReplaceAll(string(data), "\r\n", "\n")
		lines := strings.Split(content, "\n")

		updated := false
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			if !strings.HasPrefix(trimmed, "cli_auth_credentials_store") {
				continue
			}
			indentLen := len(line) - len(strings.TrimLeft(line, " \t"))
			lines[i] = line[:indentLen] + fmt.Sprintf(`cli_auth_credentials_store = %q`, credentialsStore)
			updated = true
		}

		out := strings.Join(lines, "\n")
		out = strings.TrimRight(out, "\n")
		if !updated {
			if out != "" {
				out += "\n"
			}
			out += fmt.Sprintf(`cli_auth_credentials_store = %q`, credentialsStore)
		}
		return []byte(out + "\n"), nil
	}
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}

		return copyFileIfExists(path, target)
	})
}
