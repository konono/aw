package cmd

import (
	"fmt"
	"os"

	"github.com/konono/aw/internal/profile"
	"github.com/konono/aw/internal/team"
)

func runTeam(args []string) int {
	if len(args) == 0 {
		printTeamHelp()
		return 1
	}

	switch args[0] {
	case "start":
		return runTeamStart(args[1:])
	case "stop":
		return runTeamStop(args[1:])
	case "status":
		return runTeamStatus(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "Unknown team command: %s\n", args[0])
		printTeamHelp()
		return 1
	}
}

func printTeamHelp() {
	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprintln(os.Stderr, "  aw team start <team-name>   Start all team members")
	fmt.Fprintln(os.Stderr, "  aw team stop <team-name>    Stop all team members")
	fmt.Fprintln(os.Stderr, "  aw team status [team-name]  Show team status")
}

func runTeamStart(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: aw team start <team-name>")
		return 1
	}
	teamName := args[0]

	cfg, err := profile.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		return 1
	}
	if err := profile.ValidateConfig(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	t, ok := cfg.Teams[teamName]
	if !ok {
		fmt.Fprintf(os.Stderr, "Error: team %q not found\n", teamName)
		if len(cfg.Teams) > 0 {
			fmt.Fprintln(os.Stderr, "Available teams:")
			for name := range cfg.Teams {
				fmt.Fprintf(os.Stderr, "  %s\n", name)
			}
		}
		return 1
	}

	mgr := team.NewManager()
	members := mgr.ResolveMembers(teamName, t)

	fmt.Printf("[team:%s] Resolved members:\n", teamName)
	for _, m := range members {
		fg := ""
		if m.Foreground {
			fg = " (foreground)"
		}
		fmt.Printf("  %s  profile=%s  role=%s%s\n", m.AgentName, m.Profile, m.Role, fg)
	}

	// TODO: Launch containers for each member.
	// Background members first, then foreground.
	// For now, just show what would be launched.
	fmt.Fprintf(os.Stderr, "\nTeam container launch is not yet implemented.\n")
	fmt.Fprintf(os.Stderr, "This will launch %d containers (%d background + 1 foreground).\n",
		len(members), len(members)-1)

	return 0
}

func runTeamStop(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: aw team stop <team-name>")
		return 1
	}
	teamName := args[0]

	state, err := team.LoadState(teamName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: team %q is not running: %v\n", teamName, err)
		return 1
	}

	// TODO: Stop containers via docker stop
	fmt.Printf("[team:%s] Stopping %d members...\n", teamName, len(state.Members))
	for _, m := range state.Members {
		fmt.Printf("  Stopping %s (%s)...\n", m.AgentName, m.ContainerName)
	}

	if err := team.RemoveState(teamName); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not remove state file: %v\n", err)
	}

	fmt.Printf("[team:%s] Stopped.\n", teamName)
	return 0
}

func runTeamStatus(args []string) int {
	if len(args) > 0 {
		return runTeamStatusOne(args[0])
	}

	states, err := team.ListStates()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	if len(states) == 0 {
		fmt.Println("No running teams")
		return 0
	}

	for _, s := range states {
		fmt.Printf("Team: %s (started: %s)\n", s.Name, s.StartedAt)
		for _, m := range s.Members {
			fg := ""
			if m.Foreground {
				fg = " [fg]"
			}
			fmt.Printf("  %-20s %-10s %-10s %s%s\n", m.AgentName, m.Profile, m.Role, m.Status, fg)
		}
		fmt.Println()
	}
	return 0
}

func runTeamStatusOne(teamName string) int {
	state, err := team.LoadState(teamName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Team %q is not running\n", teamName)
		return 1
	}

	fmt.Printf("Team: %s (started: %s)\n", state.Name, state.StartedAt)
	for _, m := range state.Members {
		fg := ""
		if m.Foreground {
			fg = " [fg]"
		}
		fmt.Printf("  %-20s %-10s %-10s %s%s\n", m.AgentName, m.Profile, m.Role, m.Status, fg)
	}
	return 0
}
