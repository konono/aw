package config

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// syncFiles is the list of individual files to sync from claudeHome.
var syncFiles = []string{"settings.json", "CLAUDE.md"}

// syncDirs is the list of directories to sync from claudeHome.
var syncDirs = []string{"hooks", "plugins", "commands", "agents"}

// Syncer syncs host AI tool settings to the container-side config directory.
type Syncer interface {
	SyncSettings(claudeHome, containerClaudeHome string) error
	SyncCodexSettings(codexHome, containerCodexHome string) error
	EnsureOnboardingState(path string) error
}

// DefaultSyncer is the default implementation of Syncer.
type DefaultSyncer struct{}

// NewSyncer creates a new DefaultSyncer.
func NewSyncer() *DefaultSyncer {
	return &DefaultSyncer{}
}

// SyncSettings copies settings files and directories from claudeHome to containerClaudeHome.
func (s *DefaultSyncer) SyncSettings(claudeHome, containerClaudeHome string) error {
	if err := os.MkdirAll(containerClaudeHome, 0755); err != nil {
		return fmt.Errorf("creating container claude home: %w", err)
	}

	for _, f := range syncFiles {
		src := filepath.Join(claudeHome, f)
		dst := filepath.Join(containerClaudeHome, f)
		if f == "settings.json" {
			if err := syncSettingsJSON(src, dst); err != nil {
				return fmt.Errorf("syncing file %s: %w", f, err)
			}
		} else {
			if err := copyFileIfExists(src, dst); err != nil {
				return fmt.Errorf("syncing file %s: %w", f, err)
			}
		}
	}

	for _, d := range syncDirs {
		src := filepath.Join(claudeHome, d)
		dst := filepath.Join(containerClaudeHome, d)
		if err := syncDirIfExists(src, dst); err != nil {
			return fmt.Errorf("syncing directory %s: %w", d, err)
		}
	}

	return nil
}

// SyncCodexSettings copies the Codex config directory from codexHome to containerCodexHome.
// If codexHome does not exist, it creates an empty containerCodexHome directory.
func (s *DefaultSyncer) SyncCodexSettings(codexHome, containerCodexHome string) error {
	if err := os.MkdirAll(containerCodexHome, 0755); err != nil {
		return fmt.Errorf("creating container codex home: %w", err)
	}

	info, err := os.Stat(codexHome)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return nil
	}

	if err := os.RemoveAll(containerCodexHome); err != nil {
		return fmt.Errorf("removing old container codex home: %w", err)
	}

	return copyDir(codexHome, containerCodexHome)
}

// copyFileIfExists copies src to dst if src exists. Does nothing if src doesn't exist.
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

// syncDirIfExists removes dst and copies src to dst recursively, if src exists.
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

	// Remove old destination
	if err := os.RemoveAll(dst); err != nil {
		return fmt.Errorf("removing old %s: %w", dst, err)
	}

	return copyDir(src, dst)
}

func syncSettingsJSON(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		if os.IsNotExist(err) {
			data = []byte("{}")
		} else {
			return err
		}
	}

	patched, err := patchSettingsForContainer(data)
	if err != nil {
		return fmt.Errorf("patching settings.json: %w", err)
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

// copyDir recursively copies a directory from src to dst.
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
