package profile

import (
	"testing"
)

func boolPtr(v bool) *bool {
	return &v
}

// ---------------------------------------------------------------------------
// Group 1: TestProfileOverride_FieldsTakeEffect
// ---------------------------------------------------------------------------

func TestProfileOverride_FieldsTakeEffect(t *testing.T) {
	tests := []struct {
		name     string
		base     Profile
		override Profile
		check    func(t *testing.T, m Profile)
	}{
		{
			name: "OverrideEnvironment",
			base: Profile{
				Environment: EnvironmentContainer,
				Launch:      LaunchClaude,
			},
			override: Profile{
				Environment: EnvironmentHost,
			},
			check: func(t *testing.T, m Profile) {
				if m.Environment != EnvironmentHost {
					t.Errorf("Environment = %q, want %q", m.Environment, EnvironmentHost)
				}
				if m.Launch != LaunchClaude {
					t.Errorf("Launch = %q, want %q (should be preserved from base)", m.Launch, LaunchClaude)
				}
			},
		},
		{
			name: "OverrideLaunch",
			base: Profile{
				Environment: EnvironmentContainer,
				Launch:      LaunchClaude,
			},
			override: Profile{
				Launch: LaunchShell,
			},
			check: func(t *testing.T, m Profile) {
				if m.Environment != EnvironmentContainer {
					t.Errorf("Environment = %q, want %q (should be preserved from base)", m.Environment, EnvironmentContainer)
				}
				if m.Launch != LaunchShell {
					t.Errorf("Launch = %q, want %q", m.Launch, LaunchShell)
				}
			},
		},
		{
			name: "OverrideOS",
			base: Profile{
				Environment: EnvironmentContainer,
				Launch:      LaunchClaude,
			},
			override: Profile{
				OS: OSUBI9,
			},
			check: func(t *testing.T, m Profile) {
				if m.OS != OSUBI9 {
					t.Errorf("OS = %q, want %q", m.OS, OSUBI9)
				}
			},
		},
		{
			name: "OverrideDockerfile",
			base: Profile{
				Environment: EnvironmentContainer,
				Launch:      LaunchClaude,
			},
			override: Profile{
				Dockerfile: "custom/Dockerfile",
			},
			check: func(t *testing.T, m Profile) {
				if m.Dockerfile != "custom/Dockerfile" {
					t.Errorf("Dockerfile = %q, want %q", m.Dockerfile, "custom/Dockerfile")
				}
			},
		},
		{
			name: "EmptyOverridePreservesAll",
			base: Profile{
				Worktree:    &WorktreeConfig{},
				Environment: EnvironmentContainer,
				Launch:      LaunchClaude,
			},
			override: Profile{},
			check: func(t *testing.T, m Profile) {
				if m.Environment != EnvironmentContainer {
					t.Errorf("Environment = %q, want %q", m.Environment, EnvironmentContainer)
				}
				if m.Launch != LaunchClaude {
					t.Errorf("Launch = %q, want %q", m.Launch, LaunchClaude)
				}
				if m.Worktree == nil {
					t.Fatal("Worktree should be preserved from base")
				}
			},
		},
		{
			name: "EmptyOSOverridePreservesBase",
			base: Profile{
				Environment: EnvironmentContainer,
				Launch:      LaunchClaude,
				OS:          OSUBI10,
			},
			override: Profile{},
			check: func(t *testing.T, m Profile) {
				if m.OS != OSUBI10 {
					t.Errorf("OS = %q, want %q (should be preserved from base)", m.OS, OSUBI10)
				}
			},
		},
		{
			name: "EmptyDockerfileOverridePreservesBase",
			base: Profile{
				Environment: EnvironmentContainer,
				Launch:      LaunchClaude,
				Dockerfile:  "base/Dockerfile",
			},
			override: Profile{},
			check: func(t *testing.T, m Profile) {
				if m.Dockerfile != "base/Dockerfile" {
					t.Errorf("Dockerfile = %q, want %q (should be preserved from base)", m.Dockerfile, "base/Dockerfile")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			merged := MergeProfile(tt.base, tt.override)
			tt.check(t, merged)
		})
	}
}

// ---------------------------------------------------------------------------
// Group 2: TestProfileOverride_MutualExclusivity
// ---------------------------------------------------------------------------

func TestProfileOverride_MutualExclusivity(t *testing.T) {
	tests := []struct {
		name     string
		base     Profile
		override Profile
		check    func(t *testing.T, m Profile)
	}{
		{
			name: "OSOverrideClearsInheritedDockerfile",
			base: Profile{
				Environment: EnvironmentContainer,
				Launch:      LaunchClaude,
				Dockerfile:  "base/Dockerfile",
			},
			override: Profile{
				OS: OSUBI9,
			},
			check: func(t *testing.T, m Profile) {
				if m.OS != OSUBI9 {
					t.Errorf("OS = %q, want %q", m.OS, OSUBI9)
				}
				if m.Dockerfile != "" {
					t.Errorf("Dockerfile = %q, want empty (cleared by OS override)", m.Dockerfile)
				}
			},
		},
		{
			name: "DockerfileOverrideClearsInheritedOS",
			base: Profile{
				Environment: EnvironmentContainer,
				Launch:      LaunchClaude,
				OS:          OSDebian12,
			},
			override: Profile{
				Dockerfile: "custom/Dockerfile",
			},
			check: func(t *testing.T, m Profile) {
				if m.Dockerfile != "custom/Dockerfile" {
					t.Errorf("Dockerfile = %q, want %q", m.Dockerfile, "custom/Dockerfile")
				}
				if m.OS != "" {
					t.Errorf("OS = %q, want empty (cleared by dockerfile override)", m.OS)
				}
			},
		},
		{
			name: "ImageOverrideClearsInheritedOS",
			base: Profile{
				Environment: EnvironmentContainer,
				Launch:      LaunchClaude,
				OS:          OSDebian12,
			},
			override: Profile{
				Image: "my-image:latest",
			},
			check: func(t *testing.T, m Profile) {
				if m.Image != "my-image:latest" {
					t.Errorf("Image = %q, want %q", m.Image, "my-image:latest")
				}
				if m.OS != "" {
					t.Errorf("OS = %q, want empty (cleared by image override)", m.OS)
				}
			},
		},
		{
			name: "ImageOverrideClearsInheritedDockerfile",
			base: Profile{
				Environment: EnvironmentContainer,
				Launch:      LaunchClaude,
				Dockerfile:  "base/Dockerfile",
			},
			override: Profile{
				Image: "my-image:latest",
			},
			check: func(t *testing.T, m Profile) {
				if m.Image != "my-image:latest" {
					t.Errorf("Image = %q, want %q", m.Image, "my-image:latest")
				}
				if m.Dockerfile != "" {
					t.Errorf("Dockerfile = %q, want empty (cleared by image override)", m.Dockerfile)
				}
			},
		},
		{
			name: "OSOverrideClearsInheritedImage",
			base: Profile{
				Environment: EnvironmentContainer,
				Launch:      LaunchClaude,
				Image:       "my-image:latest",
			},
			override: Profile{
				OS: OSUBI9,
			},
			check: func(t *testing.T, m Profile) {
				if m.OS != OSUBI9 {
					t.Errorf("OS = %q, want %q", m.OS, OSUBI9)
				}
				if m.Image != "" {
					t.Errorf("Image = %q, want empty (cleared by OS override)", m.Image)
				}
			},
		},
		{
			name: "DockerfileOverrideClearsInheritedImage",
			base: Profile{
				Environment: EnvironmentContainer,
				Launch:      LaunchClaude,
				Image:       "my-image:latest",
			},
			override: Profile{
				Dockerfile: "custom/Dockerfile",
			},
			check: func(t *testing.T, m Profile) {
				if m.Dockerfile != "custom/Dockerfile" {
					t.Errorf("Dockerfile = %q, want %q", m.Dockerfile, "custom/Dockerfile")
				}
				if m.Image != "" {
					t.Errorf("Image = %q, want empty (cleared by dockerfile override)", m.Image)
				}
			},
		},
		{
			name: "InvalidOverrideKeepsOSAndDockerfileForValidation",
			base: Profile{
				Environment: EnvironmentContainer,
				Launch:      LaunchClaude,
			},
			override: Profile{
				OS:         OSUBI10,
				Dockerfile: "custom/Dockerfile",
			},
			check: func(t *testing.T, m Profile) {
				if m.OS != OSUBI10 {
					t.Errorf("OS = %q, want %q", m.OS, OSUBI10)
				}
				if m.Dockerfile != "custom/Dockerfile" {
					t.Errorf("Dockerfile = %q, want %q", m.Dockerfile, "custom/Dockerfile")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			merged := MergeProfile(tt.base, tt.override)
			tt.check(t, merged)
		})
	}
}

// ---------------------------------------------------------------------------
// Group 3: TestProfileOverride_SubStructDeepMerge
// ---------------------------------------------------------------------------

func TestProfileOverride_SubStructDeepMerge(t *testing.T) {
	tests := []struct {
		name     string
		base     Profile
		override Profile
		check    func(t *testing.T, m Profile)
	}{
		{
			name: "AddWorktree",
			base: Profile{
				Environment: EnvironmentContainer,
				Launch:      LaunchClaude,
			},
			override: Profile{
				Worktree: &WorktreeConfig{Base: "origin/develop"},
			},
			check: func(t *testing.T, m Profile) {
				if m.Worktree == nil {
					t.Fatal("Worktree should not be nil")
				}
				if m.Worktree.Base != "origin/develop" {
					t.Errorf("Worktree.Base = %q, want %q", m.Worktree.Base, "origin/develop")
				}
			},
		},
		{
			name: "OverrideWorktree",
			base: Profile{
				Worktree:    &WorktreeConfig{Base: "origin/main"},
				Environment: EnvironmentContainer,
				Launch:      LaunchClaude,
			},
			override: Profile{
				Worktree: &WorktreeConfig{Base: "origin/develop", OnCreate: "./setup.sh"},
			},
			check: func(t *testing.T, m Profile) {
				if m.Worktree == nil {
					t.Fatal("Worktree should not be nil")
				}
				if m.Worktree.Base != "origin/develop" {
					t.Errorf("Worktree.Base = %q, want %q", m.Worktree.Base, "origin/develop")
				}
				if m.Worktree.OnCreate != "./setup.sh" {
					t.Errorf("Worktree.OnCreate = %q, want %q", m.Worktree.OnCreate, "./setup.sh")
				}
			},
		},
		{
			name: "PreserveWorktreeFromBase",
			base: Profile{
				Worktree:    &WorktreeConfig{Base: "origin/main"},
				Environment: EnvironmentContainer,
				Launch:      LaunchClaude,
			},
			override: Profile{
				Environment: EnvironmentHost,
			},
			check: func(t *testing.T, m Profile) {
				if m.Worktree == nil {
					t.Fatal("Worktree should be preserved from base")
				}
				if m.Worktree.Base != "origin/main" {
					t.Errorf("Worktree.Base = %q, want %q", m.Worktree.Base, "origin/main")
				}
			},
		},
		{
			name: "WorktreeDeepMerge",
			base: Profile{
				Worktree: &WorktreeConfig{Base: "origin/main", Dir: "/base/wt"},
			},
			override: Profile{
				Worktree: &WorktreeConfig{Dir: "/override/wt"},
			},
			check: func(t *testing.T, m Profile) {
				if m.Worktree.Base != "origin/main" {
					t.Errorf("Base = %q, want %q (preserved from base)", m.Worktree.Base, "origin/main")
				}
				if m.Worktree.Dir != "/override/wt" {
					t.Errorf("Dir = %q, want %q (override)", m.Worktree.Dir, "/override/wt")
				}
			},
		},
		{
			name: "AuthDeepMerge",
			base: Profile{
				Auth: &AuthConfig{
					OnLaunch: &OnLaunchAuthConfig{Check: AuthOnLaunchCheckWarn},
					Codex: &CodexAuthConfig{
						CredentialsStore: CodexCredentialsStoreFile,
						SeedFromHost:     AuthSeedFromHostIfMissing,
					},
				},
			},
			override: Profile{
				Auth: &AuthConfig{
					Codex: &CodexAuthConfig{
						LoginMode: CodexLoginModeDevice,
					},
					Claude: &ClaudeAuthConfig{
						LoginMode: ClaudeLoginModeConsole,
					},
				},
			},
			check: func(t *testing.T, m Profile) {
				if m.Auth == nil || m.Auth.Codex == nil {
					t.Fatal("merged auth.codex should not be nil")
				}
				if m.Auth.OnLaunch == nil || m.Auth.OnLaunch.Check != AuthOnLaunchCheckWarn {
					t.Fatalf("auth.on_launch = %+v, want warn", m.Auth.OnLaunch)
				}
				if m.Auth.Codex.LoginMode != CodexLoginModeDevice {
					t.Errorf("auth.codex.login_mode = %q, want %q", m.Auth.Codex.LoginMode, CodexLoginModeDevice)
				}
				if m.Auth.Codex.CredentialsStore != CodexCredentialsStoreFile {
					t.Errorf("auth.codex.credentials_store = %q, want %q", m.Auth.Codex.CredentialsStore, CodexCredentialsStoreFile)
				}
				if m.Auth.Claude == nil || m.Auth.Claude.LoginMode != ClaudeLoginModeConsole {
					t.Fatalf("auth.claude = %+v, want console", m.Auth.Claude)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			merged := MergeProfile(tt.base, tt.override)
			tt.check(t, merged)
		})
	}
}

// ---------------------------------------------------------------------------
// Group 4: TestProfileOverride_EnvVars
// ---------------------------------------------------------------------------

func TestProfileOverride_EnvVars(t *testing.T) {
	tests := []struct {
		name     string
		base     Profile
		override Profile
		check    func(t *testing.T, base Profile, m Profile)
	}{
		{
			name: "EnvMerged",
			base: Profile{
				Environment: EnvironmentContainer,
				Launch:      LaunchClaude,
				Env:         map[string]string{"A": "1", "B": "2"},
			},
			override: Profile{
				Env: map[string]string{"B": "override", "C": "3"},
			},
			check: func(t *testing.T, _ Profile, m Profile) {
				if m.Env["A"] != "1" {
					t.Errorf("Env[A] = %q, want %q", m.Env["A"], "1")
				}
				if m.Env["B"] != "override" {
					t.Errorf("Env[B] = %q, want %q (override should win)", m.Env["B"], "override")
				}
				if m.Env["C"] != "3" {
					t.Errorf("Env[C] = %q, want %q", m.Env["C"], "3")
				}
			},
		},
		{
			name: "NilEnvOverridePreservesBase",
			base: Profile{
				Environment: EnvironmentContainer,
				Launch:      LaunchClaude,
				Env:         map[string]string{"A": "1"},
			},
			override: Profile{},
			check: func(t *testing.T, _ Profile, m Profile) {
				if m.Env["A"] != "1" {
					t.Errorf("Env[A] = %q, want %q (should be preserved from base)", m.Env["A"], "1")
				}
			},
		},
		{
			name: "EnvDoesNotMutateBase",
			base: Profile{
				Environment: EnvironmentContainer,
				Launch:      LaunchClaude,
				Env:         map[string]string{"A": "1"},
			},
			override: Profile{
				Env: map[string]string{"B": "2"},
			},
			check: func(t *testing.T, base Profile, _ Profile) {
				if _, ok := base.Env["B"]; ok {
					t.Error("base.Env should not have been mutated")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			merged := MergeProfile(tt.base, tt.override)
			tt.check(t, tt.base, merged)
		})
	}
}

// ---------------------------------------------------------------------------
// Group 5: TestProfileOverride_BooleanFlags
// ---------------------------------------------------------------------------

func TestProfileOverride_BooleanFlags(t *testing.T) {
	tests := []struct {
		name          string
		base          Profile
		override      Profile
		effectiveFunc func(Profile) bool
		wantEffective bool
	}{
		{
			name:          "MountSSH_TrueOverriddenByFalse",
			base:          Profile{MountSSH: boolPtr(true)},
			override:      Profile{MountSSH: boolPtr(false)},
			effectiveFunc: func(p Profile) bool { return p.EffectiveMountSSH() },
			wantEffective: false,
		},
		{
			name:          "GhToken_TrueOverriddenByFalse",
			base:          Profile{GhToken: boolPtr(true)},
			override:      Profile{GhToken: boolPtr(false)},
			effectiveFunc: func(p Profile) bool { return p.EffectiveGhToken() },
			wantEffective: false,
		},
		{
			name:          "MountContainerSock_TrueOverriddenByFalse",
			base:          Profile{MountContainerSock: boolPtr(true)},
			override:      Profile{MountContainerSock: boolPtr(false)},
			effectiveFunc: func(p Profile) bool { return p.EffectiveMountContainerSock() },
			wantEffective: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			merged := MergeProfile(tt.base, tt.override)
			if got := tt.effectiveFunc(merged); got != tt.wantEffective {
				t.Errorf("effective = %v, want %v", got, tt.wantEffective)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Group 6: TestProfileOverride_Packages
// ---------------------------------------------------------------------------

func TestProfileOverride_Packages(t *testing.T) {
	tests := []struct {
		name     string
		base     Profile
		override Profile
		check    func(t *testing.T, m Profile)
	}{
		{
			name: "PackagesReplaced",
			base: Profile{
				Environment: EnvironmentContainer,
				Launch:      LaunchClaude,
				Packages:    []string{"jq", "tree"},
			},
			override: Profile{
				Packages: []string{"curl", "wget"},
			},
			check: func(t *testing.T, m Profile) {
				if len(m.Packages) != 2 || m.Packages[0] != "curl" || m.Packages[1] != "wget" {
					t.Errorf("Packages = %v, want [curl wget]", m.Packages)
				}
			},
		},
		{
			name: "PackagesPreservedWhenNilOverride",
			base: Profile{
				Packages: []string{"jq"},
			},
			override: Profile{},
			check: func(t *testing.T, m Profile) {
				if len(m.Packages) != 1 || m.Packages[0] != "jq" {
					t.Errorf("Packages = %v, want [jq]", m.Packages)
				}
			},
		},
		{
			name: "PackagesEmptySliceOverridesBase",
			base: Profile{
				Packages: []string{"jq"},
			},
			override: Profile{
				Packages: []string{},
			},
			check: func(t *testing.T, m Profile) {
				if len(m.Packages) != 0 {
					t.Errorf("Packages = %v, want empty", m.Packages)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			merged := MergeProfile(tt.base, tt.override)
			tt.check(t, merged)
		})
	}
}

// ---------------------------------------------------------------------------
// Group 7: TestProfileOverride_Export
// ---------------------------------------------------------------------------

func TestProfileOverride_Export(t *testing.T) {
	tests := []struct {
		name     string
		base     Profile
		override Profile
		check    func(t *testing.T, m Profile)
	}{
		{
			name: "ExportMerge",
			base: Profile{
				Environment: EnvironmentContainer,
				Launch:      LaunchClaude,
				Export: &ExportConfig{
					Snapshot: false,
					Include:  []ExportInclude{{Src: "./old", Dst: "/old"}},
					Env:      map[string]string{"A": "1", "B": "2"},
				},
			},
			override: Profile{
				Export: &ExportConfig{
					Snapshot: true,
					Include:  []ExportInclude{{Src: "./new", Dst: "/new"}},
					Env:      map[string]string{"B": "override", "C": "3"},
				},
			},
			check: func(t *testing.T, m Profile) {
				if m.Export == nil {
					t.Fatal("Export should not be nil")
				}
				if !m.Export.Snapshot {
					t.Error("Snapshot should be true after override")
				}
				if len(m.Export.Include) != 1 || m.Export.Include[0].Src != "./new" {
					t.Errorf("Include should be replaced by override, got %v", m.Export.Include)
				}
				if m.Export.Env["A"] != "1" {
					t.Errorf("Env[A] = %q, want %q (preserved from base)", m.Export.Env["A"], "1")
				}
				if m.Export.Env["B"] != "override" {
					t.Errorf("Env[B] = %q, want %q (overridden)", m.Export.Env["B"], "override")
				}
				if m.Export.Env["C"] != "3" {
					t.Errorf("Env[C] = %q, want %q (added from override)", m.Export.Env["C"], "3")
				}
			},
		},
		{
			name: "ExportNilOverridePreservesBase",
			base: Profile{
				Environment: EnvironmentContainer,
				Launch:      LaunchClaude,
				Export: &ExportConfig{
					Snapshot: true,
					Env:      map[string]string{"X": "1"},
				},
			},
			override: Profile{},
			check: func(t *testing.T, m Profile) {
				if m.Export == nil {
					t.Fatal("Export should be preserved from base")
				}
				if !m.Export.Snapshot {
					t.Error("Snapshot should be preserved from base")
				}
				if m.Export.Env["X"] != "1" {
					t.Errorf("Env[X] = %q, want %q", m.Export.Env["X"], "1")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			merged := MergeProfile(tt.base, tt.override)
			tt.check(t, merged)
		})
	}
}

// ---------------------------------------------------------------------------
// Kept unchanged: TestMergeConfig_* tests
// ---------------------------------------------------------------------------

func TestMergeConfig_BuiltinOnlyProfilesPreserved(t *testing.T) {
	builtin := Config{
		Default: "a",
		Profiles: map[string]Profile{
			"a": {Environment: EnvironmentContainer, Launch: LaunchClaude},
			"b": {Environment: EnvironmentHost, Launch: LaunchShell},
		},
	}
	user := Config{
		Profiles: map[string]Profile{
			"c": {Environment: EnvironmentHost, Launch: LaunchClaude},
		},
	}

	merged := MergeConfig(builtin, user)

	if _, ok := merged.Profiles["a"]; !ok {
		t.Error("builtin profile 'a' should be preserved")
	}
	if _, ok := merged.Profiles["b"]; !ok {
		t.Error("builtin profile 'b' should be preserved")
	}
	if _, ok := merged.Profiles["c"]; !ok {
		t.Error("user profile 'c' should be added")
	}
}

func TestMergeConfig_UserOnlyProfileAdded(t *testing.T) {
	builtin := Config{
		Profiles: map[string]Profile{
			"a": {Environment: EnvironmentContainer, Launch: LaunchClaude},
		},
	}
	user := Config{
		Profiles: map[string]Profile{
			"custom": {Environment: EnvironmentHost, Launch: LaunchShell},
		},
	}

	merged := MergeConfig(builtin, user)

	p, ok := merged.Profiles["custom"]
	if !ok {
		t.Fatal("user-only profile 'custom' should be present")
	}
	if p.Environment != EnvironmentHost {
		t.Errorf("custom.Environment = %q, want %q", p.Environment, EnvironmentHost)
	}
	if p.Launch != LaunchShell {
		t.Errorf("custom.Launch = %q, want %q", p.Launch, LaunchShell)
	}
}

func TestMergeConfig_SameNameProfileMerged(t *testing.T) {
	builtin := Config{
		Profiles: map[string]Profile{
			"claude": {Environment: EnvironmentContainer, Launch: LaunchClaude},
		},
	}
	user := Config{
		Profiles: map[string]Profile{
			"claude": {
				Worktree: &WorktreeConfig{Base: "origin/develop"},
			},
		},
	}

	merged := MergeConfig(builtin, user)

	p := merged.Profiles["claude"]
	if p.Environment != EnvironmentContainer {
		t.Errorf("Environment = %q, want %q (should be preserved from builtin)", p.Environment, EnvironmentContainer)
	}
	if p.Launch != LaunchClaude {
		t.Errorf("Launch = %q, want %q (should be preserved from builtin)", p.Launch, LaunchClaude)
	}
	if p.Worktree == nil {
		t.Fatal("Worktree should be added from user config")
	}
	if p.Worktree.Base != "origin/develop" {
		t.Errorf("Worktree.Base = %q, want %q", p.Worktree.Base, "origin/develop")
	}
}

func TestMergeConfig_DefaultOverride(t *testing.T) {
	builtin := Config{
		Default: "builtin-default",
		Profiles: map[string]Profile{
			"builtin-default": {Environment: EnvironmentContainer, Launch: LaunchClaude},
		},
	}
	user := Config{
		Default: "user-default",
		Profiles: map[string]Profile{
			"user-default": {Environment: EnvironmentHost, Launch: LaunchShell},
		},
	}

	merged := MergeConfig(builtin, user)

	if merged.Default != "user-default" {
		t.Errorf("Default = %q, want %q", merged.Default, "user-default")
	}
}

func TestMergeConfig_DefaultPreservedWhenUserEmpty(t *testing.T) {
	builtin := Config{
		Default: "builtin-default",
		Profiles: map[string]Profile{
			"builtin-default": {Environment: EnvironmentContainer, Launch: LaunchClaude},
		},
	}
	user := Config{
		Profiles: map[string]Profile{
			"custom": {Environment: EnvironmentHost, Launch: LaunchShell},
		},
	}

	merged := MergeConfig(builtin, user)

	if merged.Default != "builtin-default" {
		t.Errorf("Default = %q, want %q (should be preserved from builtin)", merged.Default, "builtin-default")
	}
}

func TestMergeConfig_TopLevelMerged(t *testing.T) {
	builtin := Config{
		Defaults: ProfileDefaults{
			Environment: EnvironmentContainer,
		},
		Profiles: map[string]Profile{},
	}
	user := Config{
		Defaults: ProfileDefaults{
			Worktree: &WorktreeConfig{Dir: "/custom"},
		},
		Profiles: map[string]Profile{},
	}

	merged := MergeConfig(builtin, user)

	if merged.Defaults.Environment != EnvironmentContainer {
		t.Errorf("top-level Environment = %q, want %q (from builtin)", merged.Defaults.Environment, EnvironmentContainer)
	}
	if merged.Defaults.Worktree == nil || merged.Defaults.Worktree.Dir != "/custom" {
		t.Errorf("top-level Worktree.Dir should be %q, got %+v", "/custom", merged.Defaults.Worktree)
	}
}

func TestMergeConfig_WorktreeEmptyObjectEnablesWorktree(t *testing.T) {
	builtin := Config{
		Profiles: map[string]Profile{
			"claude": {Environment: EnvironmentContainer, Launch: LaunchClaude},
		},
	}
	user := Config{
		Profiles: map[string]Profile{
			"claude": {
				Worktree: &WorktreeConfig{},
			},
		},
	}

	merged := MergeConfig(builtin, user)

	p := merged.Profiles["claude"]
	if p.Worktree == nil {
		t.Fatal("Worktree should be enabled via empty object from user config")
	}
	if p.Environment != EnvironmentContainer {
		t.Errorf("Environment = %q, want %q (should be preserved)", p.Environment, EnvironmentContainer)
	}
}

// ---------------------------------------------------------------------------
// Kept unchanged: TestApplyTopLevel_* tests
// ---------------------------------------------------------------------------

func TestApplyTopLevel_PropagatesToProfiles(t *testing.T) {
	cfg := Config{
		Defaults: ProfileDefaults{
			Environment: EnvironmentHost,
			Worktree:    &WorktreeConfig{Base: "origin/main", Dir: "~/.aw/wt"},
			Env:         map[string]string{"A": "1"},
			MountSSH:    boolPtr(true),
		},
		Profiles: map[string]Profile{
			"shell": {
				Launch: LaunchShell,
			},
			"docker-override": {
				Environment: EnvironmentContainer,
				Worktree:    &WorktreeConfig{Dir: "/tmp/wt"},
				Env:         map[string]string{"B": "2"},
				MountSSH:    boolPtr(false),
			},
		},
	}

	out := ApplyDefaults(cfg)

	shell := out.Profiles["shell"]
	if shell.Environment != EnvironmentHost {
		t.Errorf("shell.Environment = %q, want %q (from top-level)", shell.Environment, EnvironmentHost)
	}
	if shell.Launch != LaunchShell {
		t.Errorf("shell.Launch = %q, want %q", shell.Launch, LaunchShell)
	}
	if shell.Worktree == nil || shell.Worktree.Base != "origin/main" || shell.Worktree.Dir != "~/.aw/wt" {
		t.Errorf("shell.Worktree = %+v, want top-level values propagated", shell.Worktree)
	}
	if shell.Env["A"] != "1" {
		t.Errorf("shell.Env[A] = %q, want %q", shell.Env["A"], "1")
	}
	if !shell.EffectiveMountSSH() {
		t.Error("shell should inherit mount_ssh: true from top-level")
	}

	d := out.Profiles["docker-override"]
	if d.Environment != EnvironmentContainer {
		t.Errorf("docker-override.Environment = %q, want %q (override)", d.Environment, EnvironmentContainer)
	}
	if d.Worktree == nil || d.Worktree.Base != "origin/main" {
		t.Errorf("docker-override.Worktree.Base should inherit from top-level, got %+v", d.Worktree)
	}
	if d.Worktree.Dir != "/tmp/wt" {
		t.Errorf("docker-override.Worktree.Dir = %q, want %q (override)", d.Worktree.Dir, "/tmp/wt")
	}
	if d.Env["A"] != "1" || d.Env["B"] != "2" {
		t.Errorf("docker-override.Env = %+v, want both A and B", d.Env)
	}
	if d.EffectiveMountSSH() {
		t.Error("docker-override should override inherited mount_ssh to false")
	}
}

func TestApplyTopLevel_ProfileDockerfileOverridesTopLevelOS(t *testing.T) {
	cfg := Config{
		Defaults: ProfileDefaults{
			Environment: EnvironmentContainer,
			OS:          OSDebian12,
		},
		Profiles: map[string]Profile{
			"custom": {
				Launch:     LaunchClaude,
				Dockerfile: "Dockerfile.custom",
			},
		},
	}

	out := ApplyDefaults(cfg)
	p := out.Profiles["custom"]

	if p.Dockerfile != "Dockerfile.custom" {
		t.Errorf("Dockerfile = %q, want %q", p.Dockerfile, "Dockerfile.custom")
	}
	if p.OS != "" {
		t.Errorf("OS = %q, want empty (top-level OS should be cleared)", p.OS)
	}
}

func TestApplyTopLevel_ProfileOSOverridesTopLevelDockerfile(t *testing.T) {
	cfg := Config{
		Defaults: ProfileDefaults{
			Environment: EnvironmentContainer,
			Dockerfile:  "Dockerfile.base",
		},
		Profiles: map[string]Profile{
			"ubi9": {
				Launch: LaunchClaude,
				OS:     OSUBI9,
			},
		},
	}

	out := ApplyDefaults(cfg)
	p := out.Profiles["ubi9"]

	if p.OS != OSUBI9 {
		t.Errorf("OS = %q, want %q", p.OS, OSUBI9)
	}
	if p.Dockerfile != "" {
		t.Errorf("Dockerfile = %q, want empty (top-level dockerfile should be cleared)", p.Dockerfile)
	}
}

func TestApplyTopLevel_ProfileImageOverridesTopLevelOS(t *testing.T) {
	cfg := Config{
		Defaults: ProfileDefaults{
			Environment: EnvironmentContainer,
			OS:          OSDebian12,
		},
		Profiles: map[string]Profile{
			"airgap": {
				Launch: LaunchClaude,
				Image:  "my-image:latest",
			},
		},
	}

	out := ApplyDefaults(cfg)
	p := out.Profiles["airgap"]

	if p.Image != "my-image:latest" {
		t.Errorf("Image = %q, want %q", p.Image, "my-image:latest")
	}
	if p.OS != "" {
		t.Errorf("OS = %q, want empty (top-level OS should be cleared)", p.OS)
	}
}

// ---------------------------------------------------------------------------
// Kept unchanged: TestRelativeProfile_* tests
// ---------------------------------------------------------------------------

func TestRelativeProfile_RoundTripsThroughDefaults(t *testing.T) {
	defaults := Profile{
		Environment:        EnvironmentContainer,
		Launch:             LaunchClaude,
		Worktree:           &WorktreeConfig{Base: "origin/main", Dir: "/base"},
		Env:                map[string]string{"A": "1"},
		OS:                 OSDebian12,
		ContainerRuntime:   ContainerRuntimePodman,
		MountSSH:           boolPtr(true),
		MountContainerSock: boolPtr(false),
		Mounts:             []CustomMount{{Source: "/src", Target: "/dst"}},
		Packages:           []string{"curl"},
	}
	effective := MergeProfile(defaults, Profile{
		Launch:             LaunchCodex,
		Worktree:           &WorktreeConfig{Dir: "/override"},
		Env:                map[string]string{"B": "2"},
		Dockerfile:         "Dockerfile.custom",
		MountSSH:           boolPtr(false),
		MountContainerSock: boolPtr(true),
		Mounts:             []CustomMount{},
		Packages:           []string{"curl", "jq"},
	})

	relative := RelativeProfile(defaults, effective)
	roundTrip := MergeProfile(defaults, relative)

	if roundTrip.Environment != effective.Environment {
		t.Errorf("Environment = %q, want %q", roundTrip.Environment, effective.Environment)
	}
	if roundTrip.Launch != effective.Launch {
		t.Errorf("Launch = %q, want %q", roundTrip.Launch, effective.Launch)
	}
	if roundTrip.Worktree == nil || effective.Worktree == nil || *roundTrip.Worktree != *effective.Worktree {
		t.Errorf("Worktree = %+v, want %+v", roundTrip.Worktree, effective.Worktree)
	}
	if len(roundTrip.Env) != len(effective.Env) || roundTrip.Env["A"] != effective.Env["A"] || roundTrip.Env["B"] != effective.Env["B"] {
		t.Errorf("Env = %+v, want %+v", roundTrip.Env, effective.Env)
	}
	if roundTrip.OS != effective.OS {
		t.Errorf("OS = %q, want %q", roundTrip.OS, effective.OS)
	}
	if roundTrip.Dockerfile != effective.Dockerfile {
		t.Errorf("Dockerfile = %q, want %q", roundTrip.Dockerfile, effective.Dockerfile)
	}
	if roundTrip.ContainerRuntime != effective.ContainerRuntime {
		t.Errorf("ContainerRuntime = %q, want %q", roundTrip.ContainerRuntime, effective.ContainerRuntime)
	}
	if roundTrip.EffectiveMountSSH() != effective.EffectiveMountSSH() {
		t.Errorf("EffectiveMountSSH() = %v, want %v", roundTrip.EffectiveMountSSH(), effective.EffectiveMountSSH())
	}
	if roundTrip.EffectiveMountContainerSock() != effective.EffectiveMountContainerSock() {
		t.Errorf("EffectiveMountContainerSock() = %v, want %v", roundTrip.EffectiveMountContainerSock(), effective.EffectiveMountContainerSock())
	}
	if len(roundTrip.Mounts) != len(effective.Mounts) {
		t.Fatalf("Mounts = %+v, want %+v", roundTrip.Mounts, effective.Mounts)
	}
	for i := range roundTrip.Mounts {
		if roundTrip.Mounts[i] != effective.Mounts[i] {
			t.Errorf("Mounts[%d] = %+v, want %+v", i, roundTrip.Mounts[i], effective.Mounts[i])
		}
	}
	if len(roundTrip.Packages) != len(effective.Packages) {
		t.Fatalf("Packages len = %d, want %d", len(roundTrip.Packages), len(effective.Packages))
	}
	for i := range roundTrip.Packages {
		if roundTrip.Packages[i] != effective.Packages[i] {
			t.Errorf("Packages[%d] = %q, want %q", i, roundTrip.Packages[i], effective.Packages[i])
		}
	}
}

func TestRelativeProfile_Export(t *testing.T) {
	defaults := Profile{
		Environment: EnvironmentContainer,
		Launch:      LaunchClaude,
	}
	effective := Profile{
		Environment: EnvironmentContainer,
		Launch:      LaunchClaude,
		Export: &ExportConfig{
			Snapshot: true,
			Include:  []ExportInclude{{Src: "./certs", Dst: "/certs"}},
		},
	}

	relative := RelativeProfile(defaults, effective)

	if relative.Export == nil {
		t.Fatal("Export should appear in relative since defaults has none")
	}
	if !relative.Export.Snapshot {
		t.Error("Snapshot should be true in relative")
	}

	// When defaults and effective are the same, Export should be nil in relative
	sameDefaults := Profile{
		Environment: EnvironmentContainer,
		Launch:      LaunchClaude,
		Export:      &ExportConfig{Snapshot: true},
	}
	sameEffective := Profile{
		Environment: EnvironmentContainer,
		Launch:      LaunchClaude,
		Export:      &ExportConfig{Snapshot: true},
	}
	relative2 := RelativeProfile(sameDefaults, sameEffective)
	if relative2.Export != nil {
		t.Error("Export should be nil when defaults and effective match")
	}
}

func TestRelativeProfile_Packages(t *testing.T) {
	defaults := Profile{
		Environment: EnvironmentContainer,
		Launch:      LaunchClaude,
		Packages:    []string{"curl"},
	}

	t.Run("different packages appear in relative", func(t *testing.T) {
		effective := Profile{
			Environment: EnvironmentContainer,
			Launch:      LaunchClaude,
			Packages:    []string{"curl", "jq"},
		}
		relative := RelativeProfile(defaults, effective)
		if len(relative.Packages) != 2 || relative.Packages[0] != "curl" || relative.Packages[1] != "jq" {
			t.Errorf("Packages = %v, want [curl jq]", relative.Packages)
		}
	})

	t.Run("same packages omitted from relative", func(t *testing.T) {
		effective := Profile{
			Environment: EnvironmentContainer,
			Launch:      LaunchClaude,
			Packages:    []string{"curl"},
		}
		relative := RelativeProfile(defaults, effective)
		if relative.Packages != nil {
			t.Errorf("Packages = %v, want nil", relative.Packages)
		}
	})

	t.Run("empty slice overrides default", func(t *testing.T) {
		effective := Profile{
			Environment: EnvironmentContainer,
			Launch:      LaunchClaude,
			Packages:    []string{},
		}
		relative := RelativeProfile(defaults, effective)
		if relative.Packages == nil || len(relative.Packages) != 0 {
			t.Errorf("Packages = %v, want []", relative.Packages)
		}
	})

	t.Run("nil effective preserves default", func(t *testing.T) {
		effective := Profile{
			Environment: EnvironmentContainer,
			Launch:      LaunchClaude,
		}
		relative := RelativeProfile(defaults, effective)
		if relative.Packages != nil {
			t.Errorf("Packages = %v, want nil", relative.Packages)
		}
	})
}
