package profile

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestWorktreeConfig_EffectiveBase(t *testing.T) {
	tests := []struct {
		name string
		base string
		want string
	}{
		{"empty defaults to origin/main", "", "origin/main"},
		{"custom base", "origin/develop", "origin/develop"},
		{"specific commit", "abc123", "abc123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &WorktreeConfig{Base: tt.base}
			if got := w.EffectiveBase(); got != tt.want {
				t.Errorf("EffectiveBase() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestProfile_EffectiveOS(t *testing.T) {
	tests := []struct {
		name string
		os   OSTemplate
		want OSTemplate
	}{
		{"empty defaults to debian12", "", OSDebian12},
		{"explicit debian12", OSDebian12, OSDebian12},
		{"ubi9", OSUBI9, OSUBI9},
		{"ubi10", OSUBI10, OSUBI10},
		{"ubuntu2604", OSUbuntu2604, OSUbuntu2604},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Profile{OS: tt.os}
			if got := p.EffectiveOS(); got != tt.want {
				t.Errorf("EffectiveOS() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestProfile_EffectiveTool(t *testing.T) {
	tests := []struct {
		name    string
		profile Profile
		want    string
	}{
		{
			name:    "launch claude returns claude",
			profile: Profile{Launch: LaunchClaude},
			want:    "claude",
		},
		{
			name:    "launch codex returns codex",
			profile: Profile{Launch: LaunchCodex},
			want:    "codex",
		},
		{
			name:    "launch cursor returns cursor",
			profile: Profile{Launch: LaunchCursor},
			want:    "cursor",
		},
		{
			name:    "launch shell returns empty",
			profile: Profile{Launch: LaunchShell},
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.profile.EffectiveTool(); got != tt.want {
				t.Errorf("EffectiveTool() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestProfile_EffectiveMountContainerSock(t *testing.T) {
	tests := []struct {
		name    string
		profile *Profile
		want    bool
	}{
		{
			name:    "nil profile defaults to false",
			profile: nil,
			want:    false,
		},
		{
			name:    "unset defaults to false",
			profile: &Profile{},
			want:    false,
		},
		{
			name:    "explicit false stays false",
			profile: &Profile{MountContainerSock: boolPtr(false)},
			want:    false,
		},
		{
			name:    "explicit true enables container sock mount",
			profile: &Profile{MountContainerSock: boolPtr(true)},
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.profile.EffectiveMountContainerSock(); got != tt.want {
				t.Errorf("EffectiveMountContainerSock() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProfile_EffectiveMountSSH(t *testing.T) {
	tests := []struct {
		name    string
		profile *Profile
		want    bool
	}{
		{
			name:    "nil profile defaults to false",
			profile: nil,
			want:    false,
		},
		{
			name:    "unset defaults to false",
			profile: &Profile{},
			want:    false,
		},
		{
			name:    "explicit false stays false",
			profile: &Profile{MountSSH: boolPtr(false)},
			want:    false,
		},
		{
			name:    "explicit true enables ssh mount",
			profile: &Profile{MountSSH: boolPtr(true)},
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.profile.EffectiveMountSSH(); got != tt.want {
				t.Errorf("EffectiveMountSSH() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProfile_UnmarshalYAML_LegacyExport(t *testing.T) {
	t.Run("export field migrated to build", func(t *testing.T) {
		input := `
environment: container
launch: claude
export:
  snapshot: true
  include:
    - src: ./certs
      dst: /usr/local/share/ca-certificates
  env:
    HTTP_PROXY: http://proxy:8080
`
		var p Profile
		if err := yaml.Unmarshal([]byte(input), &p); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if p.Build == nil {
			t.Fatal("Build should be populated from legacy export field")
		}
		if !p.Build.LegacySnapshot {
			t.Error("LegacySnapshot should be true")
		}
		if len(p.Build.Include) != 1 || p.Build.Include[0].Src != "./certs" {
			t.Errorf("Include = %v, want [{./certs /usr/local/share/ca-certificates}]", p.Build.Include)
		}
		if p.Build.Env["HTTP_PROXY"] != "http://proxy:8080" {
			t.Errorf("Env[HTTP_PROXY] = %q", p.Build.Env["HTTP_PROXY"])
		}
	})

	t.Run("build field takes precedence over export", func(t *testing.T) {
		input := `
environment: container
launch: claude
build:
  include:
    - src: ./from-build
      dst: /build
export:
  snapshot: true
  include:
    - src: ./from-export
      dst: /export
`
		var p Profile
		if err := yaml.Unmarshal([]byte(input), &p); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if p.Build == nil {
			t.Fatal("Build should not be nil")
		}
		if len(p.Build.Include) != 1 || p.Build.Include[0].Src != "./from-build" {
			t.Errorf("Build should use build: field, got Include = %v", p.Build.Include)
		}
		if p.Build.LegacySnapshot {
			t.Error("LegacySnapshot should be false when build: takes precedence")
		}
	})

	t.Run("export snapshot only", func(t *testing.T) {
		input := `
environment: container
launch: claude
export:
  snapshot: true
`
		var p Profile
		if err := yaml.Unmarshal([]byte(input), &p); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if p.Build == nil {
			t.Fatal("Build should be populated from legacy export field")
		}
		if !p.Build.LegacySnapshot {
			t.Error("LegacySnapshot should be true for export: { snapshot: true }")
		}
	})

	t.Run("no export or build field", func(t *testing.T) {
		input := `
environment: container
launch: claude
`
		var p Profile
		if err := yaml.Unmarshal([]byte(input), &p); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if p.Build != nil {
			t.Errorf("Build should be nil when neither export nor build is set, got %+v", p.Build)
		}
	})
}

func TestEffectiveDelivery(t *testing.T) {
	tests := []struct {
		name     string
		delivery string
		tool     string
		want     string
	}{
		{"explicit turn", "turn", "claude", "turn"},
		{"explicit monitor", "monitor", "claude", "monitor"},
		{"explicit off", "off", "claude", "off"},
		{"default claude", "", "claude", "turn"},
		{"default codex", "", "codex", "turn"},
		{"default cursor", "", "cursor", "off"},
		{"default opencode", "", "opencode", "off"},
		{"override cursor default", "turn", "cursor", "turn"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Profile{Delivery: tt.delivery}
			if got := p.EffectiveDelivery(tt.tool); got != tt.want {
				t.Errorf("EffectiveDelivery(%q) = %q, want %q", tt.tool, got, tt.want)
			}
		})
	}
}

func TestProfile_CodexSyncOptions(t *testing.T) {
	tests := []struct {
		name          string
		auth          *AuthConfig
		wantCredStore string
		wantSeedHost  string
	}{
		{"nil auth", nil, "", ""},
		{"nil codex", &AuthConfig{}, "", ""},
		{"empty codex", &AuthConfig{Codex: &CodexAuthConfig{}}, "", ""},
		{
			"keyring + always",
			&AuthConfig{Codex: &CodexAuthConfig{
				CredentialsStore: CodexCredentialsStoreKeyring,
				SeedFromHost:     AuthSeedFromHostAlways,
			}},
			"keyring", "always",
		},
		{
			"file + never",
			&AuthConfig{Codex: &CodexAuthConfig{
				CredentialsStore: CodexCredentialsStoreFile,
				SeedFromHost:     AuthSeedFromHostNever,
			}},
			"file", "never",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := Profile{Auth: tt.auth}
			credStore, seedHost := p.CodexSyncOptions()
			if credStore != tt.wantCredStore {
				t.Errorf("credentialsStore = %q, want %q", credStore, tt.wantCredStore)
			}
			if seedHost != tt.wantSeedHost {
				t.Errorf("seedFromHost = %q, want %q", seedHost, tt.wantSeedHost)
			}
		})
	}
}
