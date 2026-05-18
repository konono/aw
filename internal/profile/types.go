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
	Env              map[string]string `yaml:"env,omitempty"`
	OS               OSTemplate        `yaml:"os,omitempty"`
	Dockerfile       string            `yaml:"dockerfile,omitempty"`
	ContainerRuntime ContainerRuntime  `yaml:"container_runtime,omitempty"`
	MountSSH         *bool             `yaml:"mount_ssh,omitempty"`
	SSHAgentForwarding *bool           `yaml:"ssh_agent_forwarding,omitempty"`
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
	shared.Dockerfile = ""
	return shared
}

// CustomMount represents a user-defined bind mount for Docker containers.
type CustomMount struct {
	Source   string `yaml:"source"`
	Target   string `yaml:"target"`
	ReadOnly bool   `yaml:"readonly,omitempty"`
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

// EffectiveMountSSH returns whether the host ~/.ssh directory should be mounted.
func (p *Profile) EffectiveMountSSH() bool {
	return p != nil && p.MountSSH != nil && *p.MountSSH
}

// EffectiveSSHAgentForwarding returns whether SSH agent forwarding should be enabled.
func (p *Profile) EffectiveSSHAgentForwarding() bool {
	return p != nil && p.SSHAgentForwarding != nil && *p.SSHAgentForwarding
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
