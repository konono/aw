package reaper

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"

	"github.com/konono/aw/internal/platform"
)

func executeTask(ctx context.Context, task ReaperTask, spec *ReaperSpec) error {
	switch task.Type {
	case "kill_process":
		var cfg KillProcessConfig
		if err := json.Unmarshal(task.Config, &cfg); err != nil {
			return fmt.Errorf("unmarshal kill_process config: %w", err)
		}
		return killProcess(cfg)
	case "remove_file":
		var cfg RemoveFileConfig
		if err := json.Unmarshal(task.Config, &cfg); err != nil {
			return fmt.Errorf("unmarshal remove_file config: %w", err)
		}
		return removeFile(cfg, spec.PodmanSSH)
	case "shell":
		var cfg ShellConfig
		if err := json.Unmarshal(task.Config, &cfg); err != nil {
			return fmt.Errorf("unmarshal shell config: %w", err)
		}
		return runShell(ctx, cfg)
	default:
		log.Printf("unknown task type: %s (skipped)", task.Type)
		return nil
	}
}

func killProcess(cfg KillProcessConfig) error {
	return platform.KillProcessIfSSH(cfg.PID, cfg.Signal)
}

func removeFile(cfg RemoveFileConfig, sshCfg *PodmanSSHConfig) error {
	if cfg.Host == "podman-vm" && sshCfg != nil {
		sshArgs := []string{
			"-o", "StrictHostKeyChecking=no",
			"-o", "UserKnownHostsFile=/dev/null",
			"-o", "LogLevel=ERROR",
			"-i", sshCfg.IdentityPath,
			"-p", strconv.Itoa(sshCfg.Port),
			fmt.Sprintf("%s@localhost", sshCfg.RemoteUsername),
			"--", "rm", "-f", cfg.Path,
		}
		return exec.Command("ssh", sshArgs...).Run()
	}
	if err := os.Remove(cfg.Path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func runShell(ctx context.Context, cfg ShellConfig) error {
	return platform.RunShellCommand(ctx, cfg.Command, cfg.Dir)
}
