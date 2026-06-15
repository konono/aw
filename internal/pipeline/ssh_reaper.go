package pipeline

// SSHReaperInfo describes SSH agent forwarding state needed for post-container
// cleanup. Implemented by sshagent.ForwardedAgent on macOS+Podman; pipeline
// does not import sshagent to avoid an import cycle.
type SSHReaperInfo interface {
	ReaperTunnelPID() int
	ReaperSocketPath() string
	ReaperPodmanSSH() *PodmanSSHConfig
}

// PodmanSSHConfig holds SSH connection info for Podman VM cleanup tasks.
type PodmanSSHConfig struct {
	IdentityPath   string
	Port           int
	RemoteUsername string
}
