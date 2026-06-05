package profile

import (
	"maps"
	"slices"
)

// MergeProfile merges override into base.
// Non-zero values in override take precedence over base.
// Sub-structs (Worktree, Zellij) are merged field-by-field rather than replaced wholesale.
func MergeProfile(base, override Profile) Profile {
	merged := base

	if override.Environment != "" {
		merged.Environment = override.Environment
	}
	if override.Launch != "" {
		merged.Launch = override.Launch
	}
	merged.Worktree = mergeWorktree(merged.Worktree, override.Worktree)
	merged.Zellij = mergeZellij(merged.Zellij, override.Zellij)
	merged.Auth = mergeAuth(merged.Auth, override.Auth)
	if override.Env != nil {
		envCopy := make(map[string]string, len(merged.Env)+len(override.Env))
		for k, v := range merged.Env {
			envCopy[k] = v
		}
		for k, v := range override.Env {
			envCopy[k] = v
		}
		merged.Env = envCopy
	}
	if override.Image != "" {
		merged.Image = override.Image
		if override.OS == "" {
			merged.OS = ""
		}
		if override.Dockerfile == "" {
			merged.Dockerfile = ""
		}
	}
	if override.OS != "" {
		merged.OS = override.OS
		if override.Dockerfile == "" {
			merged.Dockerfile = ""
		}
		if override.Image == "" {
			merged.Image = ""
		}
	}
	if override.Dockerfile != "" {
		merged.Dockerfile = override.Dockerfile
		if override.OS == "" {
			merged.OS = ""
		}
		if override.Image == "" {
			merged.Image = ""
		}
	}
	if override.ContainerRuntime != "" {
		merged.ContainerRuntime = override.ContainerRuntime
	}
	if override.MountGH != nil {
		v := *override.MountGH
		merged.MountGH = &v
	}
	if override.MountSSH != nil {
		v := *override.MountSSH
		merged.MountSSH = &v
	}
	if override.SSHAgentForwarding != nil {
		v := *override.SSHAgentForwarding
		merged.SSHAgentForwarding = &v
	}
	if override.MountContainerSock != nil {
		v := *override.MountContainerSock
		merged.MountContainerSock = &v
	}
	if override.Mounts != nil {
		merged.Mounts = override.Mounts
	}

	return merged
}

// MergeProfileDefaults merges top-level defaults using the same field rules as
// MergeProfile.
func MergeProfileDefaults(base, override ProfileDefaults) ProfileDefaults {
	return ProfileDefaultsFromProfile(MergeProfile(base.AsProfile(), override.AsProfile()))
}

func mergeWorktree(base, override *WorktreeConfig) *WorktreeConfig {
	if override == nil {
		return base
	}
	if base == nil {
		v := *override
		return &v
	}
	merged := *base
	if override.Base != "" {
		merged.Base = override.Base
	}
	if override.Dir != "" {
		merged.Dir = override.Dir
	}
	if override.OnCreate != "" {
		merged.OnCreate = override.OnCreate
	}
	if override.OnEnd != "" {
		merged.OnEnd = override.OnEnd
	}
	return &merged
}

func mergeZellij(base, override *ZellijConfig) *ZellijConfig {
	if override == nil {
		return base
	}
	if base == nil {
		v := *override
		return &v
	}
	merged := *base
	if override.Layout != "" {
		merged.Layout = override.Layout
	}
	if override.Tool != "" {
		merged.Tool = override.Tool
	}
	return &merged
}

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

// RelativeProfile returns the minimal profile override needed to reproduce
// effective when merged on top of defaults.
func RelativeProfile(defaults, effective Profile) Profile {
	relative := Profile{}
	if effective.Environment != defaults.Environment {
		relative.Environment = effective.Environment
	}
	if effective.Launch != defaults.Launch {
		relative.Launch = effective.Launch
	}
	relative.Worktree = relativeWorktree(defaults.Worktree, effective.Worktree)
	relative.Zellij = relativeZellij(defaults.Zellij, effective.Zellij)
	relative.Env = relativeEnv(defaults.Env, effective.Env)
	if effective.OS != defaults.OS {
		relative.OS = effective.OS
	}
	if effective.Image != defaults.Image {
		relative.Image = effective.Image
	}
	if effective.Dockerfile != defaults.Dockerfile {
		relative.Dockerfile = effective.Dockerfile
	}
	if effective.ContainerRuntime != defaults.ContainerRuntime {
		relative.ContainerRuntime = effective.ContainerRuntime
	}
	relative.Auth = relativeAuth(defaults.Auth, effective.Auth)
	if !equalBoolPtr(effective.MountGH, defaults.MountGH) && effective.MountGH != nil {
		v := *effective.MountGH
		relative.MountGH = &v
	}
	if !equalBoolPtr(effective.MountSSH, defaults.MountSSH) && effective.MountSSH != nil {
		v := *effective.MountSSH
		relative.MountSSH = &v
	}
	if !equalBoolPtr(effective.SSHAgentForwarding, defaults.SSHAgentForwarding) && effective.SSHAgentForwarding != nil {
		v := *effective.SSHAgentForwarding
		relative.SSHAgentForwarding = &v
	}
	if !equalBoolPtr(effective.MountContainerSock, defaults.MountContainerSock) && effective.MountContainerSock != nil {
		v := *effective.MountContainerSock
		relative.MountContainerSock = &v
	}
	if !equalMounts(effective.Mounts, defaults.Mounts) && effective.Mounts != nil {
		relative.Mounts = cloneMounts(effective.Mounts)
	}
	return relative
}

// MergeConfig merges a higher-precedence config on top of a lower-precedence config.
//   - Builtin-only profiles are preserved as-is.
//   - User-only profiles are added as-is.
//   - Profiles in both are merged (builtin base + user overlay).
//   - User's Default takes precedence if non-empty.
//   - User's top-level defaults override the builtin top-level defaults.
//
// This function does NOT apply the top-level defaults to each profile; that is
// done by ApplyTopLevel so that callers can inspect the raw per-profile data
// if they need to.
func MergeConfig(builtin, user Config) Config {
	merged := Config{
		Default:  builtin.Default,
		Defaults: MergeProfileDefaults(builtin.Defaults, user.Defaults),
		Profiles: make(map[string]Profile, len(builtin.Profiles)+len(user.Profiles)),
	}

	for name, p := range builtin.Profiles {
		merged.Profiles[name] = p
	}

	for name, userProfile := range user.Profiles {
		if base, ok := merged.Profiles[name]; ok {
			merged.Profiles[name] = MergeProfile(base, userProfile)
		} else {
			merged.Profiles[name] = userProfile
		}
	}

	if user.Default != "" {
		merged.Default = user.Default
	}

	return merged
}

// ApplyDefaults returns a new Config in which each profile is the result of
// merging the top-level defaults with the per-profile overrides.
// The returned Config's top-level defaults are left as-is (they are redundant once
// applied, but harmless and useful for round-tripping).
func ApplyDefaults(cfg Config) Config {
	out := Config{
		Default:  cfg.Default,
		Defaults: cfg.Defaults,
		Profiles: make(map[string]Profile, len(cfg.Profiles)),
		Source:   cfg.Source,
	}
	defaults := cfg.Defaults.AsProfile()
	for name, p := range cfg.Profiles {
		out.Profiles[name] = MergeProfile(defaults, p)
	}
	return out
}

// ApplyTopLevel keeps the old name for callers/tests that still refer to it.
func ApplyTopLevel(cfg Config) Config {
	return ApplyDefaults(cfg)
}

func relativeWorktree(defaults, effective *WorktreeConfig) *WorktreeConfig {
	if effective == nil {
		return nil
	}
	if defaults == nil {
		v := *effective
		return &v
	}
	relative := WorktreeConfig{}
	changed := false
	if effective.Base != defaults.Base {
		relative.Base = effective.Base
		changed = true
	}
	if effective.Dir != defaults.Dir {
		relative.Dir = effective.Dir
		changed = true
	}
	if effective.OnCreate != defaults.OnCreate {
		relative.OnCreate = effective.OnCreate
		changed = true
	}
	if effective.OnEnd != defaults.OnEnd {
		relative.OnEnd = effective.OnEnd
		changed = true
	}
	if !changed {
		return nil
	}
	return &relative
}

func relativeZellij(defaults, effective *ZellijConfig) *ZellijConfig {
	if effective == nil {
		return nil
	}
	if defaults == nil {
		v := *effective
		return &v
	}
	relative := ZellijConfig{}
	changed := false
	if effective.Layout != defaults.Layout {
		relative.Layout = effective.Layout
		changed = true
	}
	if effective.Tool != defaults.Tool {
		relative.Tool = effective.Tool
		changed = true
	}
	if !changed {
		return nil
	}
	return &relative
}

func relativeEnv(defaults, effective map[string]string) map[string]string {
	if effective == nil {
		return nil
	}
	if defaults == nil {
		return maps.Clone(effective)
	}
	relative := make(map[string]string)
	for k, v := range effective {
		if dv, ok := defaults[k]; !ok || dv != v {
			relative[k] = v
		}
	}
	if len(relative) == 0 {
		return nil
	}
	return relative
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

func equalBoolPtr(a, b *bool) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func equalMounts(a, b []CustomMount) bool {
	if (a == nil) != (b == nil) {
		return false
	}
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
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

func cloneMounts(mounts []CustomMount) []CustomMount {
	if mounts == nil {
		return nil
	}
	cloned := make([]CustomMount, len(mounts))
	copy(cloned, mounts)
	return cloned
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
