package cmd

import (
	"fmt"
	"strings"
)

type runOptions struct {
	ProfileName string
	Recent      bool
	Query       string
	Cwd         string
	NoRecord    bool
	Command     []string
}

func parseRunArgs(args []string) (*runOptions, error) {
	opts := &runOptions{}

	i := 0
	for i < len(args) {
		a := args[i]
		switch {
		case a == "--recent" || a == "--recent-dir" || a == "-r":
			opts.Recent = true
		case a == "--no-record":
			opts.NoRecord = true
		case a == "--query":
			i++
			if i >= len(args) {
				return nil, fmt.Errorf("--query requires a value")
			}
			opts.Query = args[i]
		case strings.HasPrefix(a, "--query="):
			opts.Query = strings.TrimPrefix(a, "--query=")
		case a == "-C" || a == "--cwd":
			i++
			if i >= len(args) {
				return nil, fmt.Errorf("%s requires a path", a)
			}
			opts.Cwd = args[i]
		case strings.HasPrefix(a, "--cwd="):
			opts.Cwd = strings.TrimPrefix(a, "--cwd=")
		case a == "-c":
			i++
			if i >= len(args) {
				return nil, fmt.Errorf("-c requires a command")
			}
			opts.Command = args[i:]
			i = len(args)
		case strings.HasPrefix(a, "-"):
			return nil, fmt.Errorf("unknown option: %s", a)
		default:
			if opts.ProfileName != "" {
				return nil, fmt.Errorf("unexpected argument: %s", a)
			}
			opts.ProfileName = a
		}
		i++
	}

	if opts.Recent && opts.Cwd != "" {
		return nil, fmt.Errorf("--recent and -C/--cwd cannot be used together")
	}

	if opts.Query != "" && !opts.Recent {
		return nil, fmt.Errorf("--query requires --recent")
	}

	return opts, nil
}
