package config

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// Patcher transforms file content during sync (e.g. patching settings.json).
type Patcher func([]byte) ([]byte, error)

// ToolSyncSpec defines which files/dirs to copy from the host tool config.
// Files/dirs NOT listed here are preserved in the staging dir (session data etc.)
type ToolSyncSpec struct {
	Files []string            // individual files to sync
	Dirs  []string            // directories to sync (remove & re-copy)
	Patch map[string]Patcher  // file-specific patches
}

// ClaudeSyncSpec syncs Claude Code settings while preserving session data.
var ClaudeSyncSpec = ToolSyncSpec{
	Files: []string{"settings.json", "CLAUDE.md"},
	Dirs:  []string{"hooks", "plugins", "commands", "agents"},
	Patch: map[string]Patcher{
		"settings.json": patchSettingsForContainer,
	},
}

// CodexSyncSpec syncs Codex config while preserving session history.
var CodexSyncSpec = ToolSyncSpec{
	Files: []string{"config.toml", "auth.json", "AGENTS.md", "AGENTS.override.md"},
	Dirs:  []string{"rules", "themes"},
}

// OpenCodeSyncSpec syncs OpenCode config while preserving session data.
var OpenCodeSyncSpec = ToolSyncSpec{
	Files: []string{"opencode.json", "opencode.jsonc", "tui.json", "AGENTS.md"},
	Dirs:  []string{"agents", "commands", "plugins", "skills", "tools", "themes", "modes"},
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

	for _, d := range spec.Dirs {
		src := filepath.Join(srcDir, d)
		dst := filepath.Join(dstDir, d)
		if err := syncDirIfExists(src, dst); err != nil {
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

func syncDirIfExists(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
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
			data = []byte("{}")
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
