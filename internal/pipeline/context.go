package pipeline

import (
	"github.com/konono/aw/internal/containerenv"
	"github.com/konono/aw/internal/docker"
	"github.com/konono/aw/internal/profile"
)

// ExecutionContext carries mutable state through pipeline stages.
type ExecutionContext struct {
	// Input (set before pipeline runs)
	Profile     profile.Profile
	ProfileName string
	HomeDir     string
	OrigWorkDir string // directory where `aw` was invoked

	// Set by WorktreeStage (if applicable)
	WorkDir        string // effective working directory (may be worktree path)
	WorktreePath   string // empty if no worktree was created
	WorktreeBranch string // branch name of the created worktree
	WorktreeBase   string // base ref the worktree was created from (e.g. "origin/main")
	RepoRoot       string // git repository root path

	// Set by DockerStage (if applicable)
	DockerImage        string
	DockerMounts       []docker.Mount
	DockerSecurityOpts []string
	DockerCapAdd       []string

	// Set by EnvStage (if applicable)
	EnvVars map[string]string // custom env vars to pass into Docker container

	// Set by DockerStage for SSH agent forwarding cleanup
	SSHAgentReady   bool
	SSHAgentCleanup func()

	// Set by DockerStage for container runtime socket mounting
	ContainerSockReady bool

	// Set by DockerStage when gh_token is enabled
	GhTokenValue string

	// Container environment config (user, home, workspace paths)
	ContainerEnv containerenv.Config

	// CommandOverride replaces the default launch command when set (via -c flag)
	CommandOverride []string
}
