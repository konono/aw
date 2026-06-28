package profile

import (
	"maps"
)

// MergeProfile merges override into base.
// Non-zero values in override take precedence over base.
// Sub-structs (Worktree) are merged field-by-field rather than replaced wholesale.
func MergeProfile(base, override Profile) Profile {
	merged := base
	mergeStringFields(&merged, override)
	mergeImageFields(&merged, override)
	mergeBoolPtrFields(&merged, override)
	mergeCollectionFields(&merged, base, override)
	merged.Worktree = mergeWorktree(merged.Worktree, override.Worktree)
	merged.Auth = mergeAuth(merged.Auth, override.Auth)
	merged.Export = mergeExport(merged.Export, override.Export)
	merged.Reaper = mergeReaper(merged.Reaper, override.Reaper)
	return merged
}

func mergeStringFields(merged *Profile, override Profile) {
	if override.Environment != "" {
		merged.Environment = override.Environment
	}
	if override.Launch != "" {
		merged.Launch = override.Launch
	}
	if override.Delivery != "" {
		merged.Delivery = override.Delivery
	}
	if override.ContainerRuntime != "" {
		merged.ContainerRuntime = override.ContainerRuntime
	}
	if override.ContainerUser != "" {
		merged.ContainerUser = override.ContainerUser
	}
	if override.PackageManager != "" {
		merged.PackageManager = override.PackageManager
	}
	if override.CACert != "" {
		merged.CACert = override.CACert
	}
	if override.ImagePullPolicy != "" {
		merged.ImagePullPolicy = override.ImagePullPolicy
	}
}

func mergeImageFields(merged *Profile, override Profile) {
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
}

func mergeBoolPtrFields(merged *Profile, override Profile) {
	if override.SkipDevboxInstall != nil {
		v := *override.SkipDevboxInstall
		merged.SkipDevboxInstall = &v
	}
	if override.SkipMiseInstall != nil {
		v := *override.SkipMiseInstall
		merged.SkipMiseInstall = &v
	}
	if override.GhToken != nil {
		v := *override.GhToken
		merged.GhToken = &v
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
}

func mergeCollectionFields(merged *Profile, base, override Profile) {
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
	if override.Mounts != nil {
		merged.Mounts = override.Mounts
	}
	if override.Packages != nil {
		merged.Packages = override.Packages
	}
	if override.BuildEnv != nil {
		envCopy := make(map[string]string, len(merged.BuildEnv)+len(override.BuildEnv))
		for k, v := range merged.BuildEnv {
			envCopy[k] = v
		}
		for k, v := range override.BuildEnv {
			envCopy[k] = v
		}
		merged.BuildEnv = envCopy
	}
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

// RelativeProfile returns the minimal profile override needed to reproduce
// effective when merged on top of defaults.
func RelativeProfile(defaults, effective Profile) Profile {
	relative := Profile{}
	relativeStringFields(&relative, defaults, effective)
	relativeImageFields(&relative, defaults, effective)
	relativeBoolPtrFields(&relative, defaults, effective)
	relativeCollectionFields(&relative, defaults, effective)
	relative.Worktree = relativeWorktree(defaults.Worktree, effective.Worktree)
	relative.Auth = relativeAuth(defaults.Auth, effective.Auth)
	if !equalExport(effective.Export, defaults.Export) && effective.Export != nil {
		relative.Export = cloneExport(effective.Export)
	}
	if !equalReaper(effective.Reaper, defaults.Reaper) && effective.Reaper != nil {
		c := *effective.Reaper
		relative.Reaper = &c
	}
	return relative
}

func relativeStringFields(relative *Profile, defaults, effective Profile) {
	if effective.Environment != defaults.Environment {
		relative.Environment = effective.Environment
	}
	if effective.Launch != defaults.Launch {
		relative.Launch = effective.Launch
	}
	if effective.Delivery != defaults.Delivery {
		relative.Delivery = effective.Delivery
	}
	if effective.ContainerRuntime != defaults.ContainerRuntime {
		relative.ContainerRuntime = effective.ContainerRuntime
	}
	if effective.ContainerUser != defaults.ContainerUser {
		relative.ContainerUser = effective.ContainerUser
	}
	if effective.PackageManager != defaults.PackageManager && effective.PackageManager != "" {
		relative.PackageManager = effective.PackageManager
	}
	if effective.CACert != defaults.CACert {
		relative.CACert = effective.CACert
	}
	if effective.ImagePullPolicy != defaults.ImagePullPolicy {
		relative.ImagePullPolicy = effective.ImagePullPolicy
	}
}

func relativeImageFields(relative *Profile, defaults, effective Profile) {
	if effective.OS != defaults.OS {
		relative.OS = effective.OS
	}
	if effective.Image != defaults.Image {
		relative.Image = effective.Image
	}
	if effective.Dockerfile != defaults.Dockerfile {
		relative.Dockerfile = effective.Dockerfile
	}
}

func relativeBoolPtrFields(relative *Profile, defaults, effective Profile) {
	if !equalBoolPtr(effective.SkipDevboxInstall, defaults.SkipDevboxInstall) && effective.SkipDevboxInstall != nil {
		v := *effective.SkipDevboxInstall
		relative.SkipDevboxInstall = &v
	}
	if !equalBoolPtr(effective.SkipMiseInstall, defaults.SkipMiseInstall) && effective.SkipMiseInstall != nil {
		v := *effective.SkipMiseInstall
		relative.SkipMiseInstall = &v
	}
	if !equalBoolPtr(effective.GhToken, defaults.GhToken) && effective.GhToken != nil {
		v := *effective.GhToken
		relative.GhToken = &v
	}
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
}

func relativeCollectionFields(relative *Profile, defaults, effective Profile) {
	relative.Env = relativeEnv(defaults.Env, effective.Env)
	if !equalMounts(effective.Mounts, defaults.Mounts) && effective.Mounts != nil {
		relative.Mounts = cloneMounts(effective.Mounts)
	}
	if !equalStrings(effective.Packages, defaults.Packages) && effective.Packages != nil {
		relative.Packages = append([]string{}, effective.Packages...)
	}
	relative.BuildEnv = relativeEnv(defaults.BuildEnv, effective.BuildEnv)
}

func equalReaper(a, b *ReaperProfileConfig) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
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

	// Merge teams: user teams override builtin teams by name
	if len(builtin.Teams) > 0 || len(user.Teams) > 0 {
		merged.Teams = make(map[string]Team, len(builtin.Teams)+len(user.Teams))
		for name, t := range builtin.Teams {
			merged.Teams[name] = t
		}
		for name, t := range user.Teams {
			merged.Teams[name] = t
		}
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
		Teams:    cfg.Teams,
		Source:   cfg.Source,
	}
	defaults := cfg.Defaults.AsProfile()
	for name, p := range cfg.Profiles {
		out.Profiles[name] = MergeProfile(defaults, p)
	}
	return out
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

func cloneMounts(mounts []CustomMount) []CustomMount {
	if mounts == nil {
		return nil
	}
	cloned := make([]CustomMount, len(mounts))
	copy(cloned, mounts)
	return cloned
}

func mergeExport(base, override *ExportConfig) *ExportConfig {
	if override == nil {
		return cloneExport(base)
	}
	if base == nil {
		return cloneExport(override)
	}
	merged := *base
	if override.Snapshot {
		merged.Snapshot = true
	}
	if override.Include != nil {
		merged.Include = cloneExportIncludes(override.Include)
	}
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
	return &merged
}

func cloneExport(cfg *ExportConfig) *ExportConfig {
	if cfg == nil {
		return nil
	}
	clone := *cfg
	clone.Include = cloneExportIncludes(cfg.Include)
	clone.Env = maps.Clone(cfg.Env)
	return &clone
}

func cloneExportIncludes(includes []ExportInclude) []ExportInclude {
	if includes == nil {
		return nil
	}
	cloned := make([]ExportInclude, len(includes))
	copy(cloned, includes)
	return cloned
}

func equalStrings(a, b []string) bool {
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

func equalExport(a, b *ExportConfig) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	if a.Snapshot != b.Snapshot {
		return false
	}
	if len(a.Include) != len(b.Include) {
		return false
	}
	for i := range a.Include {
		if a.Include[i] != b.Include[i] {
			return false
		}
	}
	if len(a.Env) != len(b.Env) {
		return false
	}
	for k, v := range a.Env {
		if bv, ok := b.Env[k]; !ok || v != bv {
			return false
		}
	}
	return true
}

func mergeReaper(base, override *ReaperProfileConfig) *ReaperProfileConfig {
	if override == nil {
		return base
	}
	if base == nil {
		c := *override
		return &c
	}
	merged := *base
	if override.Timeout > 0 {
		merged.Timeout = override.Timeout
	}
	if override.KeepContainer {
		merged.KeepContainer = true
	}
	if override.ReportRetention > 0 {
		merged.ReportRetention = override.ReportRetention
	}
	if override.CollectLogs != "" {
		merged.CollectLogs = override.CollectLogs
	}
	return &merged
}
