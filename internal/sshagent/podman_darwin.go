//go:build darwin

package sshagent

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

type podmanSSHConfig struct {
	IdentityPath   string
	Port           int
	RemoteUsername string
}

const selinuxModuleName = "aw_agent_sock"

const selinuxPolicy = `module aw_agent_sock 1.0;

require {
    type container_t;
    type unconfined_t;
    type user_tmp_t;
    class unix_stream_socket connectto;
    class sock_file { read write getattr };
}

allow container_t unconfined_t:unix_stream_socket connectto;
allow container_t user_tmp_t:sock_file { read write getattr };
`

func setupPodmanDarwin(hostAuthSock string) (*ForwardedAgent, error) {
	sshCfg, err := podmanMachineSSHConfig()
	if err != nil {
		return nil, fmt.Errorf("reading podman machine SSH config: %w", err)
	}

	if err := ensureSELinuxModule(sshCfg); err != nil {
		return nil, fmt.Errorf("installing SELinux module: %w", err)
	}

	pid, err := startSSHTunnel(sshCfg, hostAuthSock)
	if err != nil {
		return nil, fmt.Errorf("starting SSH tunnel: %w", err)
	}

	cleanup := func() {
		if p, err := os.FindProcess(pid); err == nil {
			_ = p.Signal(syscall.SIGTERM)
			_, _ = p.Wait()
		}
		podmanMachineExec(sshCfg, "rm", "-f", VMSocketPath)
	}

	return &ForwardedAgent{
		SocketPath: VMSocketPath,
		Cleanup:    cleanup,
	}, nil
}

func podmanMachineSSHConfig() (*podmanSSHConfig, error) {
	out, err := exec.Command("podman", "machine", "inspect").Output()
	if err != nil {
		return nil, fmt.Errorf("podman machine inspect: %w", err)
	}

	var machines []struct {
		SSHConfig struct {
			IdentityPath   string `json:"IdentityPath"`
			Port           int    `json:"Port"`
			RemoteUsername string `json:"RemoteUsername"`
		} `json:"SSHConfig"`
	}
	if err := json.Unmarshal(out, &machines); err != nil {
		return nil, fmt.Errorf("parsing inspect output: %w", err)
	}
	if len(machines) == 0 {
		return nil, fmt.Errorf("no podman machine found")
	}

	m := machines[0]
	return &podmanSSHConfig{
		IdentityPath:   m.SSHConfig.IdentityPath,
		Port:           m.SSHConfig.Port,
		RemoteUsername: m.SSHConfig.RemoteUsername,
	}, nil
}

func startSSHTunnel(cfg *podmanSSHConfig, hostAuthSock string) (int, error) {
	podmanMachineExec(cfg, "rm", "-f", VMSocketPath)

	remoteForward := fmt.Sprintf("%s:%s", VMSocketPath, hostAuthSock)
	cmd := exec.Command("ssh",
		"-f", "-N",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=ERROR",
		"-o", "StreamLocalBindUnlink=yes",
		"-i", cfg.IdentityPath,
		"-p", strconv.Itoa(cfg.Port),
		"-R", remoteForward,
		fmt.Sprintf("%s@localhost", cfg.RemoteUsername),
	)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return 0, fmt.Errorf("ssh -R: %w", err)
	}

	podmanMachineExec(cfg, "chmod", "666", VMSocketPath)

	pid, err := findSSHTunnelPID(remoteForward)
	if err != nil {
		return 0, err
	}

	return pid, nil
}

func findSSHTunnelPID(remoteForward string) (int, error) {
	out, err := exec.Command("pgrep", "-f", "ssh.*"+VMSocketPath).Output()
	if err != nil {
		return 0, fmt.Errorf("finding SSH tunnel process: %w (forward=%s)", err, remoteForward)
	}
	lines := strings.Fields(strings.TrimSpace(string(out)))
	if len(lines) == 0 {
		return 0, fmt.Errorf("SSH tunnel process not found")
	}
	pid, err := strconv.Atoi(lines[0])
	if err != nil {
		return 0, fmt.Errorf("parsing PID %q: %w", lines[0], err)
	}
	return pid, nil
}

func ensureSELinuxModule(cfg *podmanSSHConfig) error {
	out, err := podmanMachineExec(cfg, "sudo", "semodule", "-l")
	if err != nil {
		return fmt.Errorf("listing SELinux modules: %w", err)
	}
	if strings.Contains(out, selinuxModuleName) {
		return nil
	}

	fmt.Fprintf(os.Stderr, "Installing SELinux module '%s' into Podman VM...\n", selinuxModuleName)
	return compileSELinuxModule(cfg)
}

func compileSELinuxModule(cfg *podmanSSHConfig) error {
	tePath := "/tmp/" + selinuxModuleName + ".te"
	modPath := "/tmp/" + selinuxModuleName + ".mod"
	ppPath := "/tmp/" + selinuxModuleName + ".pp"

	if _, err := podmanMachineExec(cfg, "bash", "-c",
		fmt.Sprintf("cat > %s << 'SELINUX_EOF'\n%sSELINUX_EOF", tePath, selinuxPolicy)); err != nil {
		return fmt.Errorf("writing .te file: %w", err)
	}

	compileScript := fmt.Sprintf(
		"podman run --rm --security-opt label=disable -v /tmp:/tmp fedora:41 bash -c "+
			"'dnf install -yq checkpolicy policycoreutils >/dev/null 2>&1 && "+
			"checkmodule -M -m -o %s %s && "+
			"semodule_package -o %s -m %s'",
		modPath, tePath, ppPath, modPath)

	if _, err := podmanMachineExec(cfg, "bash", "-c", compileScript); err != nil {
		return fmt.Errorf("compiling SELinux module: %w", err)
	}

	if _, err := podmanMachineExec(cfg, "sudo", "semodule", "-i", ppPath); err != nil {
		return fmt.Errorf("installing SELinux module: %w", err)
	}

	podmanMachineExec(cfg, "rm", "-f", tePath, modPath, ppPath)
	return nil
}

func podmanMachineExec(cfg *podmanSSHConfig, args ...string) (string, error) {
	sshArgs := []string{
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=ERROR",
		"-i", cfg.IdentityPath,
		"-p", strconv.Itoa(cfg.Port),
		fmt.Sprintf("%s@localhost", cfg.RemoteUsername),
	}
	sshArgs = append(sshArgs, "--")
	sshArgs = append(sshArgs, args...)

	cmd := exec.Command("ssh", sshArgs...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}
