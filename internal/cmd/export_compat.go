package cmd

import (
	"fmt"
	"os"

	"github.com/konono/aw/internal/profile"
)

// Run handles the deprecated export command by delegating to BuildCmd.
func (e *ExportCmd) Run() error {
	fmt.Fprintln(os.Stderr, "Warning: 'aw export' is deprecated, use 'aw build' instead.")

	cfg, err := profile.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	var buildCfg *profile.BuildConfig
	if p, ok := cfg.Profiles[e.ProfileName]; ok {
		buildCfg = p.Build
	}

	snapshot := exportNeedsSnapshot(e.Snapshot, e.Include, e.Env, buildCfg)

	var save *string
	saveTar := !e.Apply || e.Output != ""
	if saveTar {
		save = &e.Output
	}

	b := BuildCmd{
		ProfileName:     e.ProfileName,
		Save:            save,
		FromTemplate:    true,
		Apply:           e.Apply,
		NoCache:         e.NoCache,
		Include:         e.Include,
		Env:             e.Env,
		skipSnapshot:    !snapshot,
		preloadedConfig: cfg,
	}
	return b.Run()
}

func exportNeedsSnapshot(flagSnapshot bool, flagIncludes []string, flagEnv map[string]string, profileCfg *profile.BuildConfig) bool {
	if flagSnapshot || len(flagIncludes) > 0 || len(flagEnv) > 0 {
		return true
	}
	if profileCfg != nil {
		if profileCfg.LegacySnapshot || len(profileCfg.Include) > 0 || len(profileCfg.Env) > 0 {
			return true
		}
	}
	return false
}
