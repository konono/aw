package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/konono/aw/v4/internal/dirhistory"
	"github.com/konono/aw/v4/internal/pathutil"
	"github.com/konono/aw/v4/internal/picker"
)

func expandTilde(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return pathutil.ExpandTilde(path, home)
}

func selectRecentDir(query string) (selected string, cancelled bool, err error) {
	store, err := dirhistory.Open()
	if err != nil {
		return "", false, fmt.Errorf("opening directory history: %w", err)
	}

	candidates := store.Candidates()
	if len(candidates) == 0 {
		return "", false, fmt.Errorf("no directory history found (run aw from a project directory first)")
	}

	paths := make([]string, len(candidates))
	for i, c := range candidates {
		paths[i] = c.Path
	}

	result, err := picker.Pick(paths, picker.Options{Query: query})
	if err != nil {
		if errors.Is(err, picker.ErrCancelled) {
			return "", true, nil
		}
		return "", false, err
	}

	return result, false, nil
}

func recordDirHistory(dir, profileName, origInvocationDir string) {
	if isWorktreePath(dir, origInvocationDir) {
		dir = origInvocationDir
	}

	store, err := dirhistory.Open()
	if err != nil {
		return
	}

	store.Record(dir, profileName)
	_ = store.Save()
}

func isWorktreePath(dir, origDir string) bool {
	if dir == origDir {
		return false
	}
	return strings.Contains(dir, "/worktrees/") || strings.Contains(dir, "\\worktrees\\")
}
