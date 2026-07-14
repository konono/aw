package sshagent

import "github.com/konono/aw/v4/internal/pipeline"

var _ pipeline.SSHReaperInfo = (*ForwardedAgent)(nil)

func (a *ForwardedAgent) ReaperTunnelPID() int {
	if a == nil {
		return 0
	}
	return a.SSHTunnelPID
}

func (a *ForwardedAgent) ReaperSocketPath() string {
	if a == nil {
		return ""
	}
	return a.SocketPath
}

func (a *ForwardedAgent) ReaperPodmanSSH() *pipeline.PodmanSSHConfig {
	if a == nil || a.SSHConfig == nil {
		return nil
	}
	return &pipeline.PodmanSSHConfig{
		IdentityPath:   a.SSHConfig.IdentityPath,
		Port:           a.SSHConfig.Port,
		RemoteUsername: a.SSHConfig.RemoteUsername,
	}
}
