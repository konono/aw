package completion

import (
	"sort"

	"github.com/konono/aw/v4/internal/profile"
	"github.com/konono/aw/v4/internal/toolinfo"
	"github.com/posener/complete"
)

// ProfilePredictor provides tab completion for profile names.
type ProfilePredictor struct{}

func (p ProfilePredictor) Predict(args complete.Args) []string {
	cfg, err := profile.LoadQuiet()
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(cfg.Profiles))
	for name := range cfg.Profiles {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ToolPredictor provides tab completion for tool names and profile names.
type ToolPredictor struct{}

func (t ToolPredictor) Predict(args complete.Args) []string {
	seen := make(map[string]bool)
	for _, name := range toolinfo.Names() {
		if name == "base" {
			continue
		}
		seen[name] = true
	}
	cfg, err := profile.LoadQuiet()
	if err == nil {
		for name := range cfg.Profiles {
			seen[name] = true
		}
	}
	results := make([]string, 0, len(seen))
	for name := range seen {
		results = append(results, name)
	}
	sort.Strings(results)
	return results
}

// TeamPredictor provides tab completion for team names.
type TeamPredictor struct{}

func (t TeamPredictor) Predict(args complete.Args) []string {
	cfg, err := profile.LoadQuiet()
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(cfg.Teams))
	for name := range cfg.Teams {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
