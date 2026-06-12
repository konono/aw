package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/konono/aw/internal/profile"
)

type initFlags struct {
	force bool
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

	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating config directory: %v\n", err)
		return 1
	}

	_, statErr := os.ReadFile(configPath)
	configExists := statErr == nil

	action := "Created"
	if configExists {
		if !flags.force {
			fmt.Fprintf(os.Stderr, "%s already exists. Use --force to overwrite.\n", configPath)
			return 1
		}
		action = "Overwrote"
	}

	if err := os.WriteFile(configPath, profile.DefaultConfigYAML(), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing config: %v\n", err)
		return 1
	}

	fmt.Printf("%s %s\n", action, configPath)

	writeTemplateFile(configDir, "mise.toml", miseTomlTemplate, flags.force)

	fmt.Println("Run `aw` to start immediately, or edit these files to customize your setup.")
	return 0
}

func writeTemplateFile(dir, name, content string, force bool) {
	path := filepath.Join(dir, name)
	if _, err := os.Stat(path); err == nil && !force {
		return
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not write %s: %v\n", path, err)
		return
	}
	fmt.Printf("Created %s\n", path)
}

const miseTomlTemplate = `# ~/.config/aw/mise.toml — グローバル mise 設定
#
# ここに定義したツールは全コンテナのイメージビルド時にインストールされます。
# コメントを外して使いたいツールを有効にしてください。
#
# プロジェクトに mise.toml がある場合、グローバル設定と両方が適用されます。
# 同じツールが定義されている場合、プロジェクト側が優先されます。
#
# [tools]
# node = "22"
# python = "3.14"
# go = "1.23"
# gh = "latest"
#
# [tasks.install]
# run = "echo 'Global tools installed'"
`

func parseInitArgs(args []string) (initFlags, error) {
	var flags initFlags
	for _, arg := range args {
		switch arg {
		case "--force":
			flags.force = true
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
