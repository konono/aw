package profile

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/konono/aw/v4/internal/envfile"
	"github.com/konono/aw/v4/internal/pathutil"
	"gopkg.in/yaml.v3"
)

// StringList is a list of strings that also accepts a single scalar in YAML,
// so `env_file: .env` and `env_file: [.env, .env.local]` are both valid.
type StringList []string

// UnmarshalYAML implements yaml.Unmarshaler.
func (s *StringList) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.ScalarNode:
		var v string
		if err := node.Decode(&v); err != nil {
			return err
		}
		*s = StringList{v}
		return nil
	case yaml.SequenceNode:
		var v []string
		if err := node.Decode(&v); err != nil {
			return err
		}
		*s = StringList(v)
		return nil
	default:
		return fmt.Errorf("line %d: expected a string or a list of strings", node.Line)
	}
}

// resolveEnvFilePaths rewrites every env_file entry in cfg (top-level defaults
// and each profile) to an absolute path. Relative paths are resolved against
// baseDir, the directory of the config file that declared them, so the value
// keeps pointing at the same file regardless of the current directory or a
// later worktree switch. A leading ~/ is expanded to the home directory.
func resolveEnvFilePaths(cfg *Config, baseDir string) {
	if cfg == nil {
		return
	}
	if abs, err := filepath.Abs(baseDir); err == nil {
		baseDir = abs
	}
	home, _ := os.UserHomeDir()
	resolve := func(list StringList) StringList {
		if list == nil {
			return nil
		}
		out := make(StringList, len(list))
		for i, f := range list {
			if f == "" {
				// Left empty so Validate reports it.
				continue
			}
			if home != "" {
				f = pathutil.ExpandTilde(f, home)
			}
			if !filepath.IsAbs(f) {
				f = filepath.Join(baseDir, f)
			}
			out[i] = f
		}
		return out
	}

	cfg.Defaults.EnvFile = resolve(cfg.Defaults.EnvFile)
	for name, p := range cfg.Profiles {
		p.EnvFile = resolve(p.EnvFile)
		cfg.Profiles[name] = p
	}
}

// ResolveEnvFile loads the files listed in p.EnvFile and merges their
// variables into p.Env. Files are applied in order (a later file wins over an
// earlier one), and keys defined in the profile's own env: always win over
// keys loaded from a file. A listed file that does not exist is an error.
//
// p.EnvFile is cleared afterwards so calling this twice is a no-op.
func ResolveEnvFile(p *Profile) error {
	if len(p.EnvFile) == 0 {
		return nil
	}

	merged := make(map[string]string)
	fromFile := make(map[string]bool)
	for _, path := range p.EnvFile {
		if _, err := os.Stat(path); err != nil {
			return fmt.Errorf("env_file %s: %w", path, err)
		}
		vars, err := envfile.ParseFile(path)
		if err != nil {
			return fmt.Errorf("env_file %s: %w", path, err)
		}
		for k, v := range vars {
			merged[k] = v
			fromFile[k] = true
		}
	}
	for k, v := range p.Env {
		merged[k] = v
		delete(fromFile, k)
	}

	p.Env = merged
	p.EnvFile = nil
	p.envFileKeys = fromFile
	return nil
}

// InheritableEnv returns the part of p.Env written directly in the config
// (env:), excluding values loaded from env_file. Only this part is handed to
// child aw processes through the .aw-profile-env cache: env_file values are
// often secrets and are re-read from their source by any profile that lists
// the file, so they must not outlive the run that loaded them.
func (p Profile) InheritableEnv() map[string]string {
	if len(p.envFileKeys) == 0 {
		return p.Env
	}
	out := make(map[string]string, len(p.Env))
	for k, v := range p.Env {
		if !p.envFileKeys[k] {
			out[k] = v
		}
	}
	return out
}
