package reaper

import (
	"encoding/json"

	"github.com/konono/aw/internal/pipeline"
)

// BuildSpec constructs a ReaperSpec from the current execution context.
func BuildSpec(ec *pipeline.ExecutionContext) ReaperSpec {
	timeout := 60
	keepContainer := false
	if ec.Profile.Reaper != nil {
		if ec.Profile.Reaper.Timeout > 0 {
			timeout = ec.Profile.Reaper.Timeout
		}
		keepContainer = ec.Profile.Reaper.KeepContainer
	}

	spec := ReaperSpec{
		Timeout:       timeout,
		ContainerName: ec.ContainerName,
		Runtime:       ec.Profile.EffectiveContainerRuntime(),
		KeepContainer: keepContainer,
		TTY:           DetectTTY(),
	}

	// SSH tunnel cleanup (macOS + Podman only)
	if ec.SSHReaperInfo != nil {
		if pid := ec.SSHReaperInfo.ReaperTunnelPID(); pid > 0 {
			cfg, _ := json.Marshal(KillProcessConfig{
				PID:    pid,
				Signal: 15, // SIGTERM
			})
			spec.Tasks = append(spec.Tasks, ReaperTask{
				Type:   "kill_process",
				Label:  "ssh-tunnel",
				Config: cfg,
			})
		}
		if sshCfg := ec.SSHReaperInfo.ReaperPodmanSSH(); sshCfg != nil {
			spec.PodmanSSH = &PodmanSSHConfig{
				IdentityPath:   sshCfg.IdentityPath,
				Port:           sshCfg.Port,
				RemoteUsername: sshCfg.RemoteUsername,
			}
			rmCfg, _ := json.Marshal(RemoveFileConfig{
				Path: ec.SSHReaperInfo.ReaperSocketPath(),
				Host: "podman-vm",
			})
			spec.Tasks = append(spec.Tasks, ReaperTask{
				Type:   "remove_file",
				Label:  "vm-socket",
				Config: rmCfg,
			})
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
