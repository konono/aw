//go:build ignore

// aw-sockrelay is a standalone Unix↔TCP socket relay for use inside containers.
// It creates a Unix domain socket and forwards connections to a TCP endpoint.
//
// NOTE: The relay logic here is intentionally duplicated from internal/sockrelay/relay.go
// to keep this as a self-contained module with no external dependencies (required for
// the //go:embed + cross-compile pattern). Changes to the relay algorithm should be
// applied to both files.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
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

	_ = os.Remove(*unixPath)
	ln, err := net.Listen("unix", *unixPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "listen unix %s: %v\n", *unixPath, err)
		os.Exit(1)
	}
	if err := os.Chmod(*unixPath, 0666); err != nil {
		fmt.Fprintf(os.Stderr, "chmod %s: %v\n", *unixPath, err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-sig
		cancel()
		_ = ln.Close()
	}()

	target := *tcpAddr
	var wg sync.WaitGroup
	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				wg.Wait()
				return
			default:
				fmt.Fprintf(os.Stderr, "accept: %v\n", err)
				os.Exit(1)
			}
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			handleConn(conn, target)
		}()
	}
}

func handleConn(src net.Conn, tcpAddr string) {
	dst, err := net.Dial("tcp", tcpAddr)
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
