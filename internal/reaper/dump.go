package reaper

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const dumpCommandTimeout = 5 * time.Second

func DumpDir() string {
	return filepath.Join(ReaperDir(), "dumps")
}

func shouldCollectDump(exitCode int, collectLogs string) bool {
	switch collectLogs {
	case "always":
		return true
	case "never":
		return false
	case "on_failure", "":
		return exitCode != 0
	default:
		return exitCode != 0
	}
}

func collectDump(runtime, containerName string, exitCode int, collectLogs string) string {
	if !shouldCollectDump(exitCode, collectLogs) {
		return ""
	}

	ts := time.Now().Format("2006-01-02T15-04-05")
	dir := filepath.Join(DumpDir(), fmt.Sprintf("%s-%s", ts, containerName))
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("reaper: creating dump dir: %v", err)
		return ""
	}

	type dumpItem struct {
		filename string
		collect  func(ctx context.Context) ([]byte, error)
	}

	items := []dumpItem{
		{
			filename: "logs.txt",
			collect: func(ctx context.Context) ([]byte, error) {
				return exec.CommandContext(ctx, runtime, "logs", containerName).CombinedOutput()
			},
		},
		{
			filename: "inspect.json",
			collect: func(ctx context.Context) ([]byte, error) {
				return exec.CommandContext(ctx, runtime, "inspect", containerName).Output()
			},
		},
		{
			filename: "dmesg.txt",
			collect: func(ctx context.Context) ([]byte, error) {
				return exec.CommandContext(ctx, "dmesg").CombinedOutput()
			},
		},
	}

	var wg sync.WaitGroup
	for _, item := range items {
		wg.Add(1)
		go func(it dumpItem) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), dumpCommandTimeout)
			defer cancel()
			data, err := it.collect(ctx)
			if err != nil {
				data = []byte(fmt.Sprintf("(collection failed: %v)\n", err))
			}
			if len(data) > 0 {
				path := filepath.Join(dir, it.filename)
				if writeErr := os.WriteFile(path, data, 0644); writeErr != nil {
					log.Printf("reaper: writing dump %s: %v", it.filename, writeErr)
				}
			}
		}(item)
	}
	wg.Wait()

	return dir
}

func ClearDumps() {
	dumpDir := DumpDir()
	entries, err := os.ReadDir(dumpDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			_ = os.RemoveAll(filepath.Join(dumpDir, e.Name()))
		}
	}
}

func ShowDump(dumpPath string) int {
	entries, err := os.ReadDir(dumpPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading dump directory: %v\n", err)
		return 1
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	for i, name := range files {
		if i > 0 {
			fmt.Println()
		}
		fmt.Printf("=== %s ===\n", name)
		data, err := os.ReadFile(filepath.Join(dumpPath, name))
		if err != nil {
			fmt.Fprintf(os.Stderr, "(read error: %v)\n", err)
			continue
		}
		content := strings.TrimRight(string(data), "\n")
		fmt.Println(content)
	}

	return 0
}
