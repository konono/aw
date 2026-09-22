// aw-sockrelay is a standalone Unix↔TCP socket relay for use inside containers.
// It creates a Unix domain socket and forwards connections to a TCP endpoint,
// allowing programs that expect a local Unix socket to communicate with a
// remote service via TCP.
//
// Usage:
//
//	aw-sockrelay --unix /path/to/socket --tcp host:port
package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/konono/aw/v4/internal/sockrelay"
)

func main() {
	unixPath := flag.String("unix", "", "Unix socket path to listen on")
	tcpAddr := flag.String("tcp", "", "TCP address to forward to (host:port)")
	flag.Parse()

	if *unixPath == "" || *tcpAddr == "" {
		fmt.Fprintf(os.Stderr, "Usage: aw-sockrelay --unix /path/to/socket --tcp host:port\n")
		os.Exit(1)
	}

	if err := os.MkdirAll(filepath.Dir(*unixPath), 0755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir %s: %v\n", filepath.Dir(*unixPath), err)
		os.Exit(1)
	}

	relay, err := sockrelay.UnixToTCP(*unixPath, *tcpAddr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sockrelay: %v\n", err)
		os.Exit(1)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-sig
		relay.Close()
	}()

	if err := relay.Serve(); err != nil {
		fmt.Fprintf(os.Stderr, "sockrelay: %v\n", err)
		os.Exit(1)
	}
}
