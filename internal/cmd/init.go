package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/konono/aw/internal/profile"
)

func runInit(args []string) int {
	force, err := parseInitArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	configPath, err := globalConfigPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating config directory: %v\n", err)
		return 1
	}

	action := "Created"
	if _, err := os.Stat(configPath); err == nil {
		if !force {
			fmt.Fprintf(os.Stderr, "%s already exists. Use --force to overwrite.\n", configPath)
			return 1
		}
		action = "Overwrote"
	} else if !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error checking existing config: %v\n", err)
		return 1
	}

	if err := os.WriteFile(configPath, profile.DefaultConfigYAML(), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing config: %v\n", err)
		return 1
	}

	fmt.Printf("%s %s\n", action, configPath)
	fmt.Println("Run `aw` to start immediately, or edit this file to customize your profiles.")
	return 0
}

func parseInitArgs(args []string) (bool, error) {
	force := false
	for _, arg := range args {
		switch arg {
		case "--force":
			force = true
		default:
			return false, fmt.Errorf("unknown init flag %q", arg)
		}
	}
	return force, nil
}

func globalConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "aw", "config.yml"), nil
}
