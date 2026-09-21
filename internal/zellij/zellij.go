package zellij

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
)

// ForwardedZellij holds the result of zellij socket forwarding setup.
type ForwardedZellij struct {
	SocketPath  string // path to the session socket file to mount
	SessionName string // zellij session name
	Cleanup     func() // call on shutdown to release resources
}

// Setup configures zellij socket forwarding for the given container runtime.
// It reads ZELLIJ_SESSION_NAME and ZELLIJ_SOCKET_DIR from the host environment.
// On Linux and macOS+Docker, the socket is returned directly.
// On macOS+Podman, an SSH tunnel is established into the Podman VM.
func Setup(containerRuntime, containerName string) (*ForwardedZellij, error) {
	sessionName := os.Getenv("ZELLIJ_SESSION_NAME")
	if sessionName == "" {
		return nil, fmt.Errorf("ZELLIJ_SESSION_NAME is not set; run aw from within a zellij session")
	}

	socketPath, err := resolveSocketPath(sessionName)
	if err != nil {
		return nil, err
	}

	if runtime.GOOS == "darwin" && containerRuntime == "podman" {
		return setupPodmanDarwin(socketPath, sessionName, containerName)
	}

	return &ForwardedZellij{
		SocketPath:  socketPath,
		SessionName: sessionName,
		Cleanup:     func() {},
	}, nil
}

func resolveSocketPath(sessionName string) (string, error) {
	socketDir := os.Getenv("ZELLIJ_SOCKET_DIR")
	if socketDir == "" {
		tmpDir := os.TempDir()
		tmpDir = strings.TrimRight(tmpDir, "/")
		u, err := user.Current()
		if err != nil {
			return "", fmt.Errorf("getting current user: %w", err)
		}
		socketDir = filepath.Join(tmpDir, fmt.Sprintf("zellij-%s", u.Uid))
	}

	socketPath := filepath.Join(socketDir, "contract_version_1", sessionName)
	if _, err := os.Stat(socketPath); err != nil {
		return "", fmt.Errorf("zellij socket %q does not exist: %w", socketPath, err)
	}

	return socketPath, nil
}
