package profile

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func isTrustEnvSet() bool {
	v := strings.ToLower(os.Getenv("AW_TRUST_PROJECT"))
	return v == "1" || v == "true" || v == "yes"
}

// sensitiveFieldDescriptions maps field names to human-readable risk descriptions
// displayed in the trust prompt.
var sensitiveFieldDescriptions = map[string]string{
	"worktree.on-create": "execute shell commands on your HOST machine",
	"worktree.on-end":    "execute shell commands on your HOST machine",
	"mounts":             "expose host directories to the container",
	"dockerfile":         "use a custom Dockerfile for the container image",
	"image":              "use a pre-built container image",
	"env":                "set environment variables inside the container",
	"packages":           "install OS packages inside the container",
}

// hasSensitiveFields checks whether a parsed project config contains any
// security-sensitive fields (at top level or in any profile).
func hasSensitiveFields(cfg *Config) []string {
	var found []string

	defaults := cfg.Defaults.AsProfile()
	found = append(found, profileSensitiveFields("(defaults)", defaults)...)

	for name, p := range cfg.Profiles {
		found = append(found, profileSensitiveFields(name, p)...)
	}

	return found
}

func profileSensitiveFields(profileName string, p Profile) []string {
	var found []string
	prefix := ""
	if profileName != "" {
		prefix = profileName + ": "
	}

	if p.Worktree != nil {
		if p.Worktree.OnCreate != "" {
			found = append(found, fmt.Sprintf("%sworktree.on-create = %q", prefix, p.Worktree.OnCreate))
		}
		if p.Worktree.OnEnd != "" {
			found = append(found, fmt.Sprintf("%sworktree.on-end = %q", prefix, p.Worktree.OnEnd))
		}
	}
	if len(p.Mounts) > 0 {
		for _, m := range p.Mounts {
			mode := "ro"
			if !m.IsReadOnly() {
				mode = "rw"
			}
			found = append(found, fmt.Sprintf("%smounts: %s → %s (%s)", prefix, m.Source, m.Target, mode))
		}
	}
	if p.Dockerfile != "" {
		found = append(found, fmt.Sprintf("%sdockerfile = %q", prefix, p.Dockerfile))
	}
	if p.Image != "" {
		found = append(found, fmt.Sprintf("%simage = %q", prefix, p.Image))
	}
	if len(p.Env) > 0 {
		for k, v := range p.Env {
			found = append(found, fmt.Sprintf("%senv: %s=%s", prefix, k, v))
		}
	}
	if len(p.Packages) > 0 {
		found = append(found, fmt.Sprintf("%spackages: %s", prefix, strings.Join(p.Packages, ", ")))
	}

	return found
}

// trustDir returns the path to the trust store directory.
func trustDir() (string, error) {
	dir, err := globalConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "trusted"), nil
}

// trustFilePath returns the path to the trust hash file for a given config file.
func trustFilePath(configPath string) (string, error) {
	dir, err := trustDir()
	if err != nil {
		return "", err
	}
	pathHash := fmt.Sprintf("%x", sha256.Sum256([]byte(configPath)))
	return filepath.Join(dir, pathHash+".hash"), nil
}

// contentHash returns a hex-encoded SHA256 of the given data.
func contentHash(data []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

// isTrusted checks if the given config file content has been previously approved.
// The hash covers the entire file content intentionally: even whitespace or comment
// changes trigger re-approval, erring on the side of security over convenience.
func isTrusted(configPath string, data []byte) (bool, error) {
	fp, err := trustFilePath(configPath)
	if err != nil {
		return false, err
	}

	stored, err := os.ReadFile(fp)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	return strings.TrimSpace(string(stored)) == contentHash(data), nil
}

// saveTrust records the content hash of a trusted config file.
func saveTrust(configPath string, data []byte) error {
	fp, err := trustFilePath(configPath)
	if err != nil {
		return err
	}

	dir := filepath.Dir(fp)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	return os.WriteFile(fp, []byte(contentHash(data)+"\n"), 0600)
}

// promptTrust displays sensitive fields and asks the user for approval.
// Returns true if the user approves.
var promptTrust = func(configPath string, fields []string) bool {
	fmt.Fprintf(os.Stderr, "\nProject config %s contains security-sensitive settings:\n\n", configPath)
	for _, f := range fields {
		fmt.Fprintf(os.Stderr, "  - %s\n", f)
	}
	fmt.Fprintf(os.Stderr, "\nWhat these settings can do:\n")
	seen := make(map[string]bool)
	for _, key := range []string{"worktree.on-create", "worktree.on-end", "mounts", "packages", "dockerfile", "image", "env"} {
		desc := sensitiveFieldDescriptions[key]
		if !seen[desc] {
			fmt.Fprintf(os.Stderr, "  %s: %s\n", key, desc)
			seen[desc] = true
		}
	}
	fmt.Fprintf(os.Stderr, "\nDo you trust this project config? [y/N] ")

	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
		return answer == "y" || answer == "yes"
	}
	return false
}

// stripSensitiveFields returns a copy of the config with sensitive fields removed.
func stripSensitiveFields(cfg *Config) *Config {
	stripped := *cfg
	stripped.Profiles = make(map[string]Profile, len(cfg.Profiles))

	defaults := cfg.Defaults.AsProfile()
	defaults = stripProfileSensitive(defaults)
	stripped.Defaults = ProfileDefaultsFromProfile(defaults)

	for name, p := range cfg.Profiles {
		stripped.Profiles[name] = stripProfileSensitive(p)
	}

	return &stripped
}

func stripProfileSensitive(p Profile) Profile {
	if p.Worktree != nil {
		wt := *p.Worktree
		wt.OnCreate = ""
		wt.OnEnd = ""
		p.Worktree = &wt
	}
	p.Mounts = nil
	p.Packages = nil
	p.Dockerfile = ""
	p.Image = ""
	p.Env = nil
	return p
}

// CheckProjectTrust verifies that the user trusts a project config file
// before its sensitive fields take effect. If the config has no sensitive
// fields, it is returned as-is. If not yet trusted, the user is prompted;
// if they decline, sensitive fields are stripped.
func CheckProjectTrust(configPath string, data []byte, cfg *Config) (*Config, error) {
	fields := hasSensitiveFields(cfg)
	if len(fields) == 0 {
		return cfg, nil
	}

	trusted, err := isTrusted(configPath, data)
	if err != nil {
		return nil, fmt.Errorf("checking trust status: %w", err)
	}
	if trusted {
		return cfg, nil
	}

	if isTrustEnvSet() {
		if err := saveTrust(configPath, data); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not save trust state: %v\n", err)
		}
		return cfg, nil
	}

	if promptTrust(configPath, fields) {
		if err := saveTrust(configPath, data); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not save trust state: %v\n", err)
		}
		return cfg, nil
	}

	fmt.Fprintf(os.Stderr, "Sensitive fields stripped from project config. Only safe settings applied.\n")
	return stripSensitiveFields(cfg), nil
}
