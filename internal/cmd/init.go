package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/konono/aw/internal/platform"
	"github.com/konono/aw/internal/profile"
)

// Run handles the init command.
func (i *InitCmd) Run() error {
	configPath, err := globalConfigPath()
	if err != nil {
		return err
	}

	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	_, statErr := os.ReadFile(configPath)
	configExists := statErr == nil

	action := "Created"
	if configExists {
		if !i.Force {
			fmt.Fprintf(os.Stderr, "%s already exists. Use --force to overwrite.\n", configPath)
			return ExitError{Code: 1}
		}
		action = "Overwrote"
	}

	if err := os.WriteFile(configPath, profile.DefaultConfigYAML(), 0644); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	fmt.Printf("%s %s\n", action, configPath)

	fmt.Println("Run `aw` to start immediately, or edit the config to customize your setup.")
	return nil
}



func globalConfigPath() (string, error) {
	return filepath.Join(platform.ConfigDir(), "config.yml"), nil
}
