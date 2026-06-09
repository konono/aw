package profile

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

var configFileNames = []string{".aw.yml", ".aw.yaml"}

const legacyConfigFileName = ".agent-workspace.yml"
var globalConfigFileNames = []string{"config.yml", "config.yaml"}

// globalConfigDir returns the directory for the global config file (~/.config/aw).
var globalConfigDir = func() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "aw"), nil
}

// Load finds and loads the config file.
// It merges configs in order: builtin → ~/.config/aw/config.yml → .aw.yml.
// Project config is looked up at the git repository root, or the current directory
// if not in a git repository. If no config file is found, it returns the built-in
// default config.
func Load() (*Config, error) {
	globalCfg, err := loadGlobalConfig()
	if err != nil {
		return nil, err
	}

	var dir string
	if repoRoot, err := findGitRoot(); err == nil {
		dir = repoRoot
	} else if cwd, err := os.Getwd(); err == nil {
		dir = cwd
	}

	var projectCfg *Config
	var projectPath string
	if dir != "" {
		var deprecated bool
		projectPath, deprecated = findProjectConfig(dir)
		if projectPath != "" {
			if deprecated {
				fmt.Fprintf(os.Stderr, "Warning: %s is deprecated, rename to .aw.yml\n", legacyConfigFileName)
			}
			data, err := os.ReadFile(projectPath)
			if err == nil {
				projectCfg, err = Parse(data)
				if err != nil {
					return nil, err
				}
				projectCfg, err = CheckProjectTrust(projectPath, data, projectCfg)
				if err != nil {
					return nil, fmt.Errorf("checking project config trust: %w", err)
				}
			} else if !os.IsNotExist(err) {
				return nil, fmt.Errorf("reading config file: %w", err)
			}
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
		if d, err := globalConfigDir(); err == nil {
			merged.Source = ConfigSource{FilePath: findGlobalConfig(d)}
		}
	} else {
		merged.Source = ConfigSource{IsBuiltin: true}
	}

	applied := ApplyDefaults(merged)
	return &applied, nil
}

// findGlobalConfig searches dir for a global config file (config.yml or config.yaml).
func findGlobalConfig(dir string) string {
	for _, name := range globalConfigFileNames {
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func loadGlobalConfig() (*Config, error) {
	dir, err := globalConfigDir()
	if err != nil {
		return nil, nil
	}
	path := findGlobalConfig(dir)
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil
	}
	cfg, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("parsing global config %s: %w", path, err)
	}
	return cfg, nil
}

// LoadFile loads a config from the given file path.
// If the file does not exist, it returns the built-in default config.
func LoadFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			cfg := ApplyDefaults(builtinConfig)
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
	applied := ApplyDefaults(merged)
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

// findProjectConfig searches dir for a project config file.
// It checks .aw.yml, .aw.yaml (in order), then falls back to the legacy
// .agent-workspace.yml. Returns the path and whether it matched the legacy name.
func findProjectConfig(dir string) (path string, deprecated bool) {
	for _, name := range configFileNames {
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); err == nil {
			return p, false
		}
	}
	p := filepath.Join(dir, legacyConfigFileName)
	if _, err := os.Stat(p); err == nil {
		return p, true
	}
	return "", false
}

// FindProfileSource returns the config file path where the given profile is
// defined. It checks project config first, then global config, following the
// same priority as Load(). Returns empty string if the profile only exists in
// the builtin config.
func FindProfileSource(profileName string) string {
	checkProfile := func(path string) bool {
		if data, err := os.ReadFile(path); err == nil {
			if cfg, err := Parse(data); err == nil {
				if _, ok := cfg.Profiles[profileName]; ok {
					return true
				}
			}
		}
		return false
	}

	var dir string
	if repoRoot, err := findGitRoot(); err == nil {
		dir = repoRoot
	} else if cwd, err := os.Getwd(); err == nil {
		dir = cwd
	}
	if dir != "" {
		if path, _ := findProjectConfig(dir); path != "" {
			if checkProfile(path) {
				return path
			}
		}
	}

	if cfgDir, err := globalConfigDir(); err == nil {
		if path := findGlobalConfig(cfgDir); path != "" {
			if checkProfile(path) {
				return path
			}
		}
	}

	return ""
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
