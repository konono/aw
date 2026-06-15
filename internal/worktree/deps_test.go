package worktree

import (
	"testing"
)

func TestCheckRequiredDeps(t *testing.T) {
	err := CheckRequiredDeps()
	if err != nil {
		t.Logf("CheckRequiredDeps returned error: %v", err)
	}
}

func TestCheckOptionalDeps(t *testing.T) {
	warnings := CheckOptionalDeps()
	// Just verify it returns a slice (may or may not have warnings)
	t.Logf("Optional dep warnings: %v", warnings)
}
