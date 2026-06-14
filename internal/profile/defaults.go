package profile

import (
	_ "embed"
	"fmt"
)

var (
	//go:embed embed/config.yml
	defaultConfigYAML []byte

	// builtinConfig is the embedded starter config prepared for runtime merging.
	// Top-level defaults (environment, os, etc.) are preserved so that project
	// .aw.yml profiles inherit them through the normal merge chain. Only image
	// and dockerfile are stripped because empty is their valid default state.
	builtinConfig = mustParseBuiltinConfig()
)

// DefaultConfigYAML returns the embedded starter config used by `aw init`.
func DefaultConfigYAML() []byte {
	return append([]byte(nil), defaultConfigYAML...)
}

func mustParseBuiltinConfig() Config {
	cfg, err := Parse(defaultConfigYAML)
	if err != nil {
		panic(fmt.Sprintf("parse embedded default config: %v", err))
	}

	applied := ApplyDefaults(*cfg)
	sharedDefaults := cfg.Defaults.BuiltinShared()
	baked := Config{
		Default:  applied.Default,
		Defaults: sharedDefaults,
		Profiles: make(map[string]Profile, len(applied.Profiles)),
	}
	sharedProfile := sharedDefaults.AsProfile()
	for name, p := range applied.Profiles {
		baked.Profiles[name] = RelativeProfile(sharedProfile, p)
	}

	final := ApplyDefaults(baked)
	if err := ValidateConfig(&final); err != nil {
		panic(fmt.Sprintf("validate embedded default config: %v", err))
	}

	return baked
}
