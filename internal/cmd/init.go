package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/konono/aw/internal/profile"
)

type initFlags struct {
	force  bool
	update bool
}

func runInit(args []string) int {
	flags, err := parseInitArgs(args)
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

	existingData, statErr := os.ReadFile(configPath)
	configExists := statErr == nil

	if flags.update && configExists {
		return runMigrate(configPath, existingData)
	}

	action := "Created"
	if configExists {
		if !flags.force {
			fmt.Fprintf(os.Stderr, "%s already exists. Use --force to overwrite or --update to migrate.\n", configPath)
			return 1
		}
		action = "Overwrote"
	}

	if err := os.WriteFile(configPath, profile.DefaultConfigYAML(), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing config: %v\n", err)
		return 1
	}

	fmt.Printf("%s %s\n", action, configPath)
	fmt.Println("Run `aw` to start immediately, or edit this file to customize your profiles.")
	return 0
}

func runMigrate(configPath string, existingData []byte) int {
	userCfg, err := profile.Parse(existingData)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing existing config: %v\n", err)
		return 1
	}

	migrated, err := profile.Migrate(userCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error migrating config: %v\n", err)
		return 1
	}

	backupPath := configPath + ".bak"
	if err := os.WriteFile(backupPath, existingData, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating backup: %v\n", err)
		return 1
	}

	if err := os.WriteFile(configPath, migrated, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing migrated config: %v\n", err)
		return 1
	}

	fmt.Printf("Updated %s (backed up to %s)\n", configPath, filepath.Base(backupPath))
	return 0
}

func parseInitArgs(args []string) (initFlags, error) {
	var flags initFlags
	for _, arg := range args {
		switch arg {
		case "--force":
			flags.force = true
		case "--update":
			flags.update = true
		default:
			return flags, fmt.Errorf("unknown init flag %q", arg)
		}
	}
	return flags, nil
}

func globalConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "aw", "config.yml"), nil
}
