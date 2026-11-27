package envfile

import (
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"
)

// WriteFile writes KEY=VALUE pairs to the given file path.
// Keys are sorted for deterministic output.
// If env is empty or nil, no file is created.
//
// The file is owner-only (0600) because values are often secrets. A file
// that already exists keeps its mode under os.WriteFile, so it is chmod'ed
// back to 0600 as well.
func WriteFile(path string, env map[string]string) error {
	if len(env) == 0 {
		return nil
	}

	var b strings.Builder
	for _, k := range slices.Sorted(maps.Keys(env)) {
		fmt.Fprintf(&b, "%s=%s\n", k, env[k])
	}

	if err := os.WriteFile(path, []byte(b.String()), 0600); err != nil {
		return err
	}
	return os.Chmod(path, 0600)
}
