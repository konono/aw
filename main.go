package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/alecthomas/kong"
	"github.com/konono/aw/internal/cmd"
	"github.com/konono/aw/internal/reaper"
	"github.com/konono/aw/internal/version"
)

func main() {
	args := os.Args[1:]

	// --internal-reaper must be handled before kong for fastest subprocess startup.
	if len(args) > 0 && args[0] == "--internal-reaper" {
		os.Exit(reaper.Run())
	}

	// Split -c passthrough before kong sees it.
	kongArgs, passthroughArgs := cmd.SplitAtDashC(args)
	pt := &cmd.PassthroughCmd{Args: passthroughArgs}

	var cli cmd.CLI
	parser, err := kong.New(&cli,
		kong.Name("aw"),
		kong.Description("agent-workspace"),
		kong.Vars{"version": version.Version},
		kong.UsageOnError(),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	ctx, err := parser.Parse(kongArgs)
	if err != nil {
		parser.FatalIfErrorf(err)
	}

	err = ctx.Run(pt)
	if err != nil {
		var exitErr cmd.ExitError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.Code)
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
