package profile

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const configFileName = ".agent-workspace.yml"
const globalConfigFileName = "config.yml"

// globalConfigDir returns the directory for the global config file (~/.config/aw).
var globalConfigDir = func() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "aw"), nil
}

// builtinConfig is used when no config file is found.
var builtinConfig = Config{
	Default: "worktree-zellij",
	Profiles: map[string]Profile{
		"claude": {
			Environment: EnvironmentContainer,
			Launch:      LaunchClaude,
		},
		"codex": {
			Environment: EnvironmentContainer,
			Launch:      LaunchCodex,
		},
		"worktree-zellij": {
			Worktree:    &WorktreeConfig{},
			Environment: EnvironmentContainer,
			Launch:      LaunchZellij,
			Zellij:      &ZellijConfig{Layout: "default"},
		},
	},
}

// Load finds and loads the config file.
// It merges configs in order: builtin → ~/.config/aw/config.yml → .agent-workspace.yml.
// Project config is looked up at the git repository root, or the current directory
// if not in a git repository. If no config file is found, it returns the built-in
// default config.
func Load() (*Config, error) {
	globalCfg, err := loadGlobalConfig()
	if err != nil {
		return nil, err
	}

	var projectPath string
	if repoRoot, err := findGitRoot(); err == nil {
		projectPath = filepath.Join(repoRoot, configFileName)
	} else if cwd, err := os.Getwd(); err == nil {
		projectPath = filepath.Join(cwd, configFileName)
	}

	var projectCfg *Config
	if projectPath != "" {
		data, err := os.ReadFile(projectPath)
		if err == nil {
			projectCfg, err = Parse(data)
			if err != nil {
				return nil, err
			}
		} else if !os.IsNotExist(err) {
			return nil, fmt.Errorf("reading config file: %w", err)
		}
	}

	merged := builtinConfig
	if globalCfg != nil {
		merged = MergeConfig(merged, *globalCfg)
	}
	if projectCfg != nil {
		merged = MergeConfig(merged, *projectCfg)
		merged.Source = ConfigSource{FilePath: projectPath}
	} else if globalCfg != nil {
		dir, _ := globalConfigDir()
		merged.Source = ConfigSource{FilePath: filepath.Join(dir, globalConfigFileName)}
	} else {
		merged.Source = ConfigSource{IsBuiltin: true}
	}

	applied := ApplyTopLevel(merged)
	return &applied, nil
}

func loadGlobalConfig() (*Config, error) {
	dir, err := globalConfigDir()
	if err != nil {
		return nil, nil
	}
	data, err := os.ReadFile(filepath.Join(dir, globalConfigFileName))
	if err != nil {
		return nil, nil
	}
	cfg, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("parsing global config %s: %w", filepath.Join(dir, globalConfigFileName), err)
	}
	return cfg, nil
}

// LoadFile loads a config from the given file path.
// If the file does not exist, it returns the built-in default config.
func LoadFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			cfg := builtinConfig
			cfg.Source = ConfigSource{IsBuiltin: true}
			return &cfg, nil
		}
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	userCfg, err := Parse(data)
	if err != nil {
		return nil, err
	}

	merged := MergeConfig(builtinConfig, *userCfg)
	merged.Source = ConfigSource{FilePath: path}
	applied := ApplyTopLevel(merged)
	return &applied, nil
}

// Parse parses YAML bytes into a Config.
func Parse(data []byte) (*Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	if cfg.Profiles == nil {
		cfg.Profiles = make(map[string]Profile)
	}

	return &cfg, nil
}

// findGitRoot returns the top-level directory of the current git repository.
var findGitRoot = func() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("not in a git repository")
	}
	return strings.TrimSpace(string(out)), nil
}
