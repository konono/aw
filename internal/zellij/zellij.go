package zellij

import (
	"fmt"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/konono/aw/v4/internal/sockrelay"
)

// ForwardedZellij holds the result of zellij socket forwarding setup.
type ForwardedZellij struct {
	SocketPath  string // host-side zellij session socket path
	SessionName string // zellij session name
	RelayAddr   string // TCP address the host relay is listening on
	Cleanup     func() // call on shutdown to release resources
}

// Setup locates the host zellij socket and starts a TCP relay so containers
// can access it via host.containers.internal (Podman) or host.docker.internal (Docker).
func Setup(containerRuntime, containerName string) (*ForwardedZellij, error) {
	sessionName := os.Getenv("ZELLIJ_SESSION_NAME")
	if sessionName == "" {
		return nil, fmt.Errorf("ZELLIJ_SESSION_NAME is not set; run aw from within a zellij session")
	}

	socketPath, err := resolveSocketPath(sessionName)
	if err != nil {
		return nil, err
	}

	relay, err := sockrelay.TCPToUnix("127.0.0.1:0", socketPath)
	if err != nil {
		return nil, fmt.Errorf("starting zellij relay: %w", err)
	}

	go func() { _ = relay.Serve() }()

	tcpAddr := relay.Addr().(*net.TCPAddr)

	return &ForwardedZellij{
		SocketPath:  socketPath,
		SessionName: sessionName,
		RelayAddr:   fmt.Sprintf("%d", tcpAddr.Port),
		Cleanup:     relay.Close,
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
