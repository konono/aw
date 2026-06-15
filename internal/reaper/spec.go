package reaper

import (
	"encoding/json"

	"github.com/konono/aw/internal/pipeline"
	"github.com/konono/aw/internal/sshagent"
)

// BuildSpec constructs a ReaperSpec from the current execution context.
func BuildSpec(ec *pipeline.ExecutionContext) ReaperSpec {
	spec := ReaperSpec{
		Timeout:       60,
		ContainerName: ec.ContainerName,
		Runtime:       ec.Profile.EffectiveContainerRuntime(),
	}

	// SSH tunnel cleanup (macOS + Podman only)
	if ec.SSHForwardedAgent != nil {
		if agent, ok := ec.SSHForwardedAgent.(*sshagent.ForwardedAgent); ok {
			if agent.SSHTunnelPID > 0 {
				cfg, _ := json.Marshal(KillProcessConfig{
					PID:    agent.SSHTunnelPID,
					Signal: 15, // SIGTERM
				})
				spec.Tasks = append(spec.Tasks, ReaperTask{
					Type:   "kill_process",
					Label:  "ssh-tunnel",
					Config: cfg,
				})
			}
			if agent.SSHConfig != nil {
				spec.PodmanSSH = &PodmanSSHConfig{
					IdentityPath:   agent.SSHConfig.IdentityPath,
					Port:           agent.SSHConfig.Port,
					RemoteUsername: agent.SSHConfig.RemoteUsername,
				}
				rmCfg, _ := json.Marshal(RemoveFileConfig{
					Path: agent.SocketPath,
					Host: "podman-vm",
				})
				spec.Tasks = append(spec.Tasks, ReaperTask{
					Type:   "remove_file",
					Label:  "vm-socket",
					Config: rmCfg,
				})
			}
		}
	}

	// on-end hook
	if ec.Profile.Worktree != nil && ec.Profile.Worktree.OnEnd != "" && ec.WorktreePath != "" {
		shellCfg, _ := json.Marshal(ShellConfig{
			Command: ec.Profile.Worktree.OnEnd,
			Dir:     ec.WorktreePath,
		})
		spec.Tasks = append(spec.Tasks, ReaperTask{
			Type:   "shell",
			Label:  "on-end",
			Config: shellCfg,
		})
	}

	return spec
}
