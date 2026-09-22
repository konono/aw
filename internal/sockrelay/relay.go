// Package sockrelay provides a generic bidirectional relay between TCP and
// Unix domain sockets. It is designed for forwarding Unix sockets across
// VM/container boundaries where direct socket access is not possible
// (e.g., macOS host → Podman VM → container).
package sockrelay

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
)

// Relay bridges connections between a listener and a dialer.
// For each accepted connection on the listener side, it dials the target
// and copies data bidirectionally.
type Relay struct {
	listener net.Listener
	dialFunc func() (net.Conn, error)
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

// TCPToUnix creates a relay that listens on a TCP address and forwards
// each connection to a Unix domain socket. Runs on the host side.
func TCPToUnix(tcpAddr, unixPath string) (*Relay, error) {
	ln, err := net.Listen("tcp", tcpAddr)
	if err != nil {
		return nil, fmt.Errorf("listen tcp %s: %w", tcpAddr, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	r := &Relay{
		listener: ln,
		dialFunc: func() (net.Conn, error) {
			return net.Dial("unix", unixPath)
		},
		ctx:    ctx,
		cancel: cancel,
	}
	return r, nil
}

// UnixToTCP creates a relay that listens on a Unix domain socket and
// forwards each connection to a TCP address. Runs on the container side.
func UnixToTCP(unixPath, tcpAddr string) (*Relay, error) {
	_ = os.Remove(unixPath)
	ln, err := net.Listen("unix", unixPath)
	if err != nil {
		return nil, fmt.Errorf("listen unix %s: %w", unixPath, err)
	}
	if err := os.Chmod(unixPath, 0666); err != nil {
		_ = ln.Close()
		return nil, fmt.Errorf("chmod %s: %w", unixPath, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	r := &Relay{
		listener: ln,
		dialFunc: func() (net.Conn, error) {
			return net.Dial("tcp", tcpAddr)
		},
		ctx:    ctx,
		cancel: cancel,
	}
	return r, nil
}

// Addr returns the listener's address.
func (r *Relay) Addr() net.Addr {
	return r.listener.Addr()
}

// Serve accepts connections and relays them until the context is cancelled
// or the listener is closed. It blocks until all connections are done.
func (r *Relay) Serve() error {
	go func() {
		<-r.ctx.Done()
		_ = r.listener.Close()
	}()

	for {
		conn, err := r.listener.Accept()
		if err != nil {
			select {
			case <-r.ctx.Done():
				r.wg.Wait()
				return nil
			default:
				return fmt.Errorf("accept: %w", err)
			}
		}
		r.wg.Add(1)
		go func() {
			defer r.wg.Done()
			r.handleConn(conn)
		}()
	}
}

// Close stops the relay and waits for active connections to finish.
func (r *Relay) Close() {
	r.cancel()
	r.wg.Wait()
}

func (r *Relay) handleConn(src net.Conn) {
	dst, err := r.dialFunc()
	if err != nil {
		_ = src.Close()
		return
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(dst, src)
		_ = dst.Close()
	}()
	go func() {
		defer wg.Done()
		_, _ = io.Copy(src, dst)
		_ = src.Close()
	}()
	wg.Wait()
}
