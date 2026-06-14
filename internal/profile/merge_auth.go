package profile

import "slices"

func mergeAuth(base, override *AuthConfig) *AuthConfig {
	if override == nil {
		return cloneAuth(base)
	}
	if base == nil {
		return cloneAuth(override)
	}
	return &AuthConfig{
		OnLaunch: mergeOnLaunchAuth(base.OnLaunch, override.OnLaunch),
		Codex:    mergeCodexAuth(base.Codex, override.Codex),
		Claude:   mergeClaudeAuth(base.Claude, override.Claude),
		OpenCode: mergeOpenCodeAuth(base.OpenCode, override.OpenCode),
	}
}

func mergeOnLaunchAuth(base, override *OnLaunchAuthConfig) *OnLaunchAuthConfig {
	if override == nil {
		return cloneOnLaunchAuth(base)
	}
	if base == nil {
		return cloneOnLaunchAuth(override)
	}
	merged := *base
	if override.Check != "" {
		merged.Check = override.Check
	}
	return &merged
}

func mergeCodexAuth(base, override *CodexAuthConfig) *CodexAuthConfig {
	if override == nil {
		return cloneCodexAuth(base)
	}
	if base == nil {
		return cloneCodexAuth(override)
	}
	merged := *base
	if override.LoginMode != "" {
		merged.LoginMode = override.LoginMode
	}
	if override.CredentialsStore != "" {
		merged.CredentialsStore = override.CredentialsStore
	}
	if override.SeedFromHost != "" {
		merged.SeedFromHost = override.SeedFromHost
	}
	if override.PersistAuth != "" {
		merged.PersistAuth = override.PersistAuth
	}
	if override.LoginArgs != nil {
		merged.LoginArgs = slices.Clone(override.LoginArgs)
	}
	return &merged
}

func mergeClaudeAuth(base, override *ClaudeAuthConfig) *ClaudeAuthConfig {
	if override == nil {
		return cloneClaudeAuth(base)
	}
	if base == nil {
		return cloneClaudeAuth(override)
	}
	merged := *base
	if override.LoginMode != "" {
		merged.LoginMode = override.LoginMode
	}
	if override.LoginArgs != nil {
		merged.LoginArgs = slices.Clone(override.LoginArgs)
	}
	return &merged
}

func mergeOpenCodeAuth(base, override *OpenCodeAuthConfig) *OpenCodeAuthConfig {
	if override == nil {
		return cloneOpenCodeAuth(base)
	}
	if base == nil {
		return cloneOpenCodeAuth(override)
	}
	merged := *base
	if override.Provider != "" {
		merged.Provider = override.Provider
	}
	if override.Method != "" {
		merged.Method = override.Method
	}
	if override.LoginArgs != nil {
		merged.LoginArgs = slices.Clone(override.LoginArgs)
	}
	return &merged
}

func relativeAuth(defaults, effective *AuthConfig) *AuthConfig {
	if effective == nil {
		return nil
	}
	if equalAuth(defaults, effective) {
		return nil
	}
	return cloneAuth(effective)
}

func equalAuth(a, b *AuthConfig) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return equalOnLaunchAuth(a.OnLaunch, b.OnLaunch) &&
		equalCodexAuth(a.Codex, b.Codex) &&
		equalClaudeAuth(a.Claude, b.Claude) &&
		equalOpenCodeAuth(a.OpenCode, b.OpenCode)
}

func equalOnLaunchAuth(a, b *OnLaunchAuthConfig) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Check == b.Check
}

func equalCodexAuth(a, b *CodexAuthConfig) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.LoginMode == b.LoginMode &&
		a.CredentialsStore == b.CredentialsStore &&
		a.SeedFromHost == b.SeedFromHost &&
		a.PersistAuth == b.PersistAuth &&
		slices.Equal(a.LoginArgs, b.LoginArgs)
}

func equalClaudeAuth(a, b *ClaudeAuthConfig) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.LoginMode == b.LoginMode &&
		slices.Equal(a.LoginArgs, b.LoginArgs)
}

func equalOpenCodeAuth(a, b *OpenCodeAuthConfig) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Provider == b.Provider &&
		a.Method == b.Method &&
		slices.Equal(a.LoginArgs, b.LoginArgs)
}

func cloneAuth(cfg *AuthConfig) *AuthConfig {
	if cfg == nil {
		return nil
	}
	return &AuthConfig{
		OnLaunch: cloneOnLaunchAuth(cfg.OnLaunch),
		Codex:    cloneCodexAuth(cfg.Codex),
		Claude:   cloneClaudeAuth(cfg.Claude),
		OpenCode: cloneOpenCodeAuth(cfg.OpenCode),
	}
}

func cloneOnLaunchAuth(cfg *OnLaunchAuthConfig) *OnLaunchAuthConfig {
	if cfg == nil {
		return nil
	}
	clone := *cfg
	return &clone
}

func cloneCodexAuth(cfg *CodexAuthConfig) *CodexAuthConfig {
	if cfg == nil {
		return nil
	}
	clone := *cfg
	clone.LoginArgs = slices.Clone(cfg.LoginArgs)
	return &clone
}

func cloneClaudeAuth(cfg *ClaudeAuthConfig) *ClaudeAuthConfig {
	if cfg == nil {
		return nil
	}
	clone := *cfg
	clone.LoginArgs = slices.Clone(cfg.LoginArgs)
	return &clone
}

func cloneOpenCodeAuth(cfg *OpenCodeAuthConfig) *OpenCodeAuthConfig {
	if cfg == nil {
		return nil
	}
	clone := *cfg
	clone.LoginArgs = slices.Clone(cfg.LoginArgs)
	return &clone
}
