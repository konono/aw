package cmd

import (
	"fmt"
	"os"

	"github.com/konono/aw/internal/profile"
)

// Run handles the deprecated export command by delegating to BuildCmd.
func (e *ExportCmd) Run() error {
	fmt.Fprintln(os.Stderr, "Warning: 'aw export' is deprecated, use 'aw build' instead.")

	var save *string
	snapshot := e.Snapshot || len(e.Include) > 0 || len(e.Env) > 0

	if !snapshot {
		if profileCfg := loadProfileBuildConfig(e.ProfileName); profileCfg != nil {
			if len(profileCfg.Include) > 0 || len(profileCfg.Env) > 0 {
				snapshot = true
			}
		}
	}

	saveTar := !e.Apply || e.Output != ""
	if saveTar {
		save = &e.Output
	}

	b := BuildCmd{
		ProfileName:  e.ProfileName,
		Save:         save,
		FromTemplate: true,
		Apply:        e.Apply,
		NoCache:      e.NoCache,
		Include:      e.Include,
		Env:          e.Env,
		skipSnapshot: !snapshot,
	}
	return b.Run()
}

func loadProfileBuildConfig(profileName string) *profile.BuildConfig {
	cfg, err := profile.Load()
	if err != nil {
		return nil
	}
	p, ok := cfg.Profiles[profileName]
	if !ok {
		return nil
	}
	return p.Build
}
