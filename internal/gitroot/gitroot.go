package gitroot

import (
	"fmt"
	"os/exec"
	"strings"
)

// FindRoot returns the top-level directory of the current git repository.
// Tests may replace this variable to mock git root detection.
var FindRoot = func() (string, error) {
	return FindRootFrom("")
}

// FindRootFrom returns the git repository root for the given directory.
// An empty dir uses the current working directory.
func FindRootFrom(dir string) (string, error) {
	var cmd *exec.Cmd
	if dir == "" {
		cmd = exec.Command("git", "rev-parse", "--show-toplevel")
	} else {
		cmd = exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel")
	}
	out, err := cmd.Output()
	if err != nil {
		if dir == "" {
			return "", fmt.Errorf("not in a git repository")
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
