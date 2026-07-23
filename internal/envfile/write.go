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
func WriteFile(path string, env map[string]string) error {
	if len(env) == 0 {
		return nil
	}

	var b strings.Builder
	for _, k := range slices.Sorted(maps.Keys(env)) {
		fmt.Fprintf(&b, "%s=%s\n", k, env[k])
	}

	return os.WriteFile(path, []byte(b.String()), 0644)
}
