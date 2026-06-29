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

	writeTemplateFile(configDir, "mise.toml", miseTomlTemplate, i.Force)
	writeTemplateFile(configDir, "devbox.json", devboxJSONTemplate, i.Force)

	fmt.Println("Run `aw` to start immediately, or edit these files to customize your setup.")
	return nil
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
# go = "1.25"
# gh = "latest"
#
# [tasks.install]
# run = "echo 'Global tools installed'"
`

// devboxJSONTemplate is the starter devbox.json for package_manager: devbox (deprecated).
const devboxJSONTemplate = `{
  "_comment": "DEPRECATED: devbox (Nix) package manager. Use package_manager: apt instead.",
  "packages": []
}
`

func globalConfigPath() (string, error) {
	return filepath.Join(platform.ConfigDir(), "config.yml"), nil
}
