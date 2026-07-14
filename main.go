package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/alecthomas/kong"
	kongcompletion "github.com/jotaen/kong-completion"
	"github.com/konono/aw/v4/internal/cmd"
	"github.com/konono/aw/v4/internal/completion"
	"github.com/konono/aw/v4/internal/reaper"
	"github.com/konono/aw/v4/internal/version"
)

func main() {
	args := os.Args[1:]

	// --internal-reaper must be handled before kong for fastest subprocess startup.
	if len(args) > 0 && args[0] == "--internal-reaper" {
		os.Exit(reaper.Run())
	}

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

	// Split passthrough (-- or legacy -c) only for the default run command.
	// When an explicit subcommand is used (e.g. "completion -c"),
	// leave args for kong to handle as subcommand flags.
	kongArgs := args
	var passthroughArgs []string
	var usedLegacy bool
	if !cmd.IsSubcommand(parser, args) {
		var splitErr error
		kongArgs, passthroughArgs, usedLegacy, splitErr = cmd.ExtractRunPassthrough(args)
		if splitErr != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", splitErr)
			os.Exit(1)
		}
	}
	pt := &cmd.PassthroughCmd{Args: passthroughArgs, UsedLegacy: usedLegacy}

	kongcompletion.Register(parser,
		kongcompletion.WithPredictor("profile", completion.ProfilePredictor{}),
		kongcompletion.WithPredictor("tool", completion.ToolPredictor{}),
		kongcompletion.WithPredictor("team", completion.TeamPredictor{}),
	)

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
