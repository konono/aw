package profile

// ConfigSource describes where the config was loaded from.
type ConfigSource struct {
	IsBuiltin bool   // true if the built-in default config was used
	FilePath  string // non-empty if loaded from a file
}

// Config represents the top-level .agent-workspace.yml file.
//
// Profile fields declared at the top level (via Defaults) act as defaults for
// every profile in Profiles. Each profile overrides those defaults
// field-by-field using the same merge rules as MergeProfile.
type Config struct {
	Default  string             `yaml:"default"`
	Defaults ProfileDefaults    `yaml:",inline"` // top-level defaults shared by all profiles
	Profiles map[string]Profile `yaml:"profiles"`
	Source   ConfigSource       `yaml:"-"`
}

// ContainerRuntime specifies the container CLI to use.
type ContainerRuntime string

const (
	ContainerRuntimeDocker ContainerRuntime = "docker"
	ContainerRuntimePodman ContainerRuntime = "podman"
)

// OSTemplate specifies the base OS for the container image.
type OSTemplate string

const (
	OSDebian12   OSTemplate = "debian12"
	OSUBI9       OSTemplate = "ubi9"
	OSUBI10      OSTemplate = "ubi10"
	OSUbuntu2604 OSTemplate = "ubuntu2604"
)

// Profile describes a single named workspace profile.
type Profile struct {
	Worktree         *WorktreeConfig   `yaml:"worktree,omitempty"`
	Environment      Environment       `yaml:"environment"`
	Launch           LaunchMode        `yaml:"launch"`
	Zellij           *ZellijConfig     `yaml:"zellij,omitempty"`
	Auth             *AuthConfig       `yaml:"auth,omitempty"`
	Env              map[string]string `yaml:"env,omitempty"`
	OS               OSTemplate        `yaml:"os,omitempty"`
	Image            string            `yaml:"image,omitempty"`
	Dockerfile       string            `yaml:"dockerfile,omitempty"`
	ContainerRuntime   ContainerRuntime  `yaml:"container_runtime,omitempty"`
	SkipDevboxInstall *bool             `yaml:"skip_devbox_install,omitempty"`
	SkipMiseInstall   *bool             `yaml:"skip_mise_install,omitempty"`
	MountGH          *bool             `yaml:"mount_gh,omitempty"`
	MountSSH         *bool             `yaml:"mount_ssh,omitempty"`
	SSHAgentForwarding *bool           `yaml:"ssh_agent_forwarding,omitempty"`
	MountContainerSock *bool          `yaml:"mount_container_sock,omitempty"`
	Mounts           []CustomMount     `yaml:"mounts,omitempty"`
}

// ProfileDefaults describes top-level defaults shared by all profiles.
type ProfileDefaults Profile

// AsProfile converts top-level defaults into a Profile for merge operations.
func (d ProfileDefaults) AsProfile() Profile {
	return Profile(d)
}

// ProfileDefaultsFromProfile converts a Profile into top-level defaults.
func ProfileDefaultsFromProfile(p Profile) ProfileDefaults {
	return ProfileDefaults(p)
}

// BuiltinShared returns the subset of starter defaults that should remain
// inheritable across later config layers. Container-only fields are baked into
// the built-in starter profiles so user-defined host profiles do not inherit
// invalid defaults from the embedded template.
func (d ProfileDefaults) BuiltinShared() ProfileDefaults {
	shared := d
	shared.Environment = ""
	shared.OS = ""
	shared.Image = ""
	shared.Dockerfile = ""
	return shared
}

// MountMode specifies the access mode for a custom mount.
type MountMode string

const (
	MountModeRO MountMode = "ro"
	MountModeRW MountMode = "rw"
)

// CustomMount represents a user-defined bind mount for Docker containers.
// Mounts are read-only by default; set mode: rw to allow writes.
type CustomMount struct {
	Source  string    `yaml:"source"`
	Target  string    `yaml:"target"`
	Mode    MountMode `yaml:"mode,omitempty"`    // "ro" (default) or "rw"
	Options string    `yaml:"options,omitempty"` // extra mount options (e.g. "z", "Z,nocopy", "cached")
}

// IsReadOnly returns whether this mount should be read-only.
func (m CustomMount) IsReadOnly() bool {
	return m.Mode != MountModeRW
}

// EffectiveOS returns the OS template, defaulting to "debian12" if empty.
func (p *Profile) EffectiveOS() OSTemplate {
	if p.OS != "" {
		return p.OS
	}
	return OSDebian12
}

// EffectiveContainerRuntime returns the container runtime, defaulting to "docker".
func (p *Profile) EffectiveContainerRuntime() string {
	if p.ContainerRuntime == ContainerRuntimePodman {
		return "podman"
	}
	return "docker"
}

// EffectiveMountGH returns whether the host ~/.config/gh directory should be mounted.
// Defaults to false; the gh token is readable by the AI agent and can be
// exfiltrated via prompt injection attacks.
func (p *Profile) EffectiveMountGH() bool {
	return p != nil && p.MountGH != nil && *p.MountGH
}

// EffectiveMountSSH returns whether the host ~/.ssh directory should be mounted.
func (p *Profile) EffectiveMountSSH() bool {
	return p != nil && p.MountSSH != nil && *p.MountSSH
}

// EffectiveSSHAgentForwarding returns whether SSH agent forwarding should be enabled.
func (p *Profile) EffectiveSSHAgentForwarding() bool {
	return p != nil && p.SSHAgentForwarding != nil && *p.SSHAgentForwarding
}

// EffectiveMountContainerSock returns whether the container runtime socket should be mounted.
func (p *Profile) EffectiveMountContainerSock() bool {
	return p != nil && p.MountContainerSock != nil && *p.MountContainerSock
}

// EffectiveSkipDevboxInstall returns whether devbox install should be skipped in the entrypoint.
func (p *Profile) EffectiveSkipDevboxInstall() bool {
	return p != nil && p.SkipDevboxInstall != nil && *p.SkipDevboxInstall
}

// EffectiveSkipMiseInstall returns whether mise install should be skipped in the entrypoint.
func (p *Profile) EffectiveSkipMiseInstall() bool {
	return p != nil && p.SkipMiseInstall != nil && *p.SkipMiseInstall
}

// EffectiveAuthOnLaunchCheck returns the configured launch-time auth check mode.
// Empty means disabled.
func (p *Profile) EffectiveAuthOnLaunchCheck() AuthOnLaunchCheck {
	if p == nil || p.Auth == nil || p.Auth.OnLaunch == nil {
		return ""
	}
	return p.Auth.OnLaunch.Check
}

// WorktreeConfig controls git worktree creation.
type WorktreeConfig struct {
	Base     string `yaml:"base,omitempty"`      // default: "origin/main"
	Dir      string `yaml:"dir,omitempty"`       // directory to host worktrees in; default: <repoRoot>/worktrees. Supports ~ expansion and paths relative to repoRoot.
	OnCreate string `yaml:"on-create,omitempty"` // shell command to run after worktree creation
	OnEnd    string `yaml:"on-end,omitempty"`    // shell command to run after launched process exits
}

// EffectiveBase returns the base ref, defaulting to "origin/main" if empty.
func (w *WorktreeConfig) EffectiveBase() string {
	if w.Base != "" {
		return w.Base
	}
	return "origin/main"
}

// ZellijConfig controls zellij session settings.
type ZellijConfig struct {
	Layout string `yaml:"layout,omitempty"` // "default" or custom path (future)
	Tool   string `yaml:"tool,omitempty"`   // AI tool to use: "claude" (default) or "codex"
}

// AuthConfig ties authentication behavior to a profile. It does not store
// tokens or API keys; those remain in each tool's own credential store.
type AuthConfig struct {
	OnLaunch *OnLaunchAuthConfig `yaml:"on_launch,omitempty"`
	Codex    *CodexAuthConfig    `yaml:"codex,omitempty"`
	Claude   *ClaudeAuthConfig   `yaml:"claude,omitempty"`
	OpenCode *OpenCodeAuthConfig `yaml:"opencode,omitempty"`
}

// OnLaunchAuthConfig controls optional auth checks before a normal `aw <profile>`
// launch. This does not run login automatically; it only checks status.
type OnLaunchAuthConfig struct {
	Check AuthOnLaunchCheck `yaml:"check,omitempty"`
}

type AuthOnLaunchCheck string

const (
	AuthOnLaunchCheckNone    AuthOnLaunchCheck = "none"
	AuthOnLaunchCheckWarn    AuthOnLaunchCheck = "warn"
	AuthOnLaunchCheckRequire AuthOnLaunchCheck = "require"
)

type CodexAuthConfig struct {
	LoginMode        CodexLoginMode        `yaml:"login_mode,omitempty"`
	CredentialsStore CodexCredentialsStore `yaml:"credentials_store,omitempty"`
	SeedFromHost     AuthSeedFromHostMode  `yaml:"seed_from_host,omitempty"`
	PersistAuth      AuthPersistMode       `yaml:"persist_auth,omitempty"`
	LoginArgs        []string              `yaml:"login_args,omitempty"`
}

type CodexLoginMode string

const (
	CodexLoginModeBrowser     CodexLoginMode = "browser"
	CodexLoginModeDevice      CodexLoginMode = "device"
	CodexLoginModeAPIKey      CodexLoginMode = "api-key"
	CodexLoginModeAccessToken CodexLoginMode = "access-token"
)

type CodexCredentialsStore string

const (
	CodexCredentialsStoreFile    CodexCredentialsStore = "file"
	CodexCredentialsStoreKeyring CodexCredentialsStore = "keyring"
	CodexCredentialsStoreAuto    CodexCredentialsStore = "auto"
)

type AuthSeedFromHostMode string

const (
	AuthSeedFromHostIfMissing AuthSeedFromHostMode = "if_missing"
	AuthSeedFromHostAlways    AuthSeedFromHostMode = "always"
	AuthSeedFromHostNever     AuthSeedFromHostMode = "never"
)

type AuthPersistMode string

const (
	AuthPersistModeStage AuthPersistMode = "stage"
)

type ClaudeAuthConfig struct {
	LoginMode ClaudeLoginMode `yaml:"login_mode,omitempty"`
	LoginArgs []string        `yaml:"login_args,omitempty"`
}

type ClaudeLoginMode string

const (
	ClaudeLoginModeBrowser ClaudeLoginMode = "browser"
	ClaudeLoginModeConsole ClaudeLoginMode = "console"
	ClaudeLoginModeEmail   ClaudeLoginMode = "email"
	ClaudeLoginModeSSO     ClaudeLoginMode = "sso"
)

type OpenCodeAuthConfig struct {
	Provider  string   `yaml:"provider,omitempty"`
	Method    string   `yaml:"method,omitempty"`
	LoginArgs []string `yaml:"login_args,omitempty"`
}

// Environment specifies where the main process runs.
type Environment string

const (
	EnvironmentHost      Environment = "host"
	EnvironmentContainer Environment = "container"
)

// LaunchMode specifies what to launch.
type LaunchMode string

const (
	LaunchShell    LaunchMode = "shell"
	LaunchClaude   LaunchMode = "claude"
	LaunchCodex    LaunchMode = "codex"
	LaunchOpenCode LaunchMode = "opencode"
	LaunchZellij   LaunchMode = "zellij"
)

// EffectiveTool returns the AI tool name ("claude" or "codex") based on
// the launch mode and, for zellij, the ZellijConfig.Tool override.
func (p *Profile) EffectiveTool() string {
	switch p.Launch {
	case LaunchClaude:
		return "claude"
	case LaunchCodex:
		return "codex"
	case LaunchOpenCode:
		return "opencode"
	case LaunchZellij:
		if p.Zellij != nil && p.Zellij.Tool != "" {
			return p.Zellij.Tool
		}
		return "claude"
	default:
		return ""
	}
}
