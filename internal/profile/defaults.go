package profile

import (
	_ "embed"
	"fmt"
)

var (
	//go:embed embed/config.yml
	defaultConfigYAML []byte

	// builtinConfig is the embedded starter config prepared for runtime merging.
	// We bake environment/os into the starter profiles, but preserve only the
	// top-level container runtime as inheritable so users can still override it
	// globally without inheriting container-only fields like os into host profiles.
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

	defaultRuntime := cfg.ContainerRuntime
	applied := ApplyTopLevel(*cfg)
	if defaultRuntime != "" {
		for name, p := range applied.Profiles {
			if p.ContainerRuntime == defaultRuntime {
				p.ContainerRuntime = ""
				applied.Profiles[name] = p
			}
		}
	}
	applied.Profile = Profile{ContainerRuntime: defaultRuntime}

	final := ApplyTopLevel(applied)
	if err := ValidateConfig(&final); err != nil {
		panic(fmt.Sprintf("validate embedded default config: %v", err))
	}

	return applied
}
