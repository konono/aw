package stage

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"

	"github.com/konono/aw/v4/internal/envfile"
	"github.com/konono/aw/v4/internal/pipeline"
)

const (
	envFileName        = ".aw-env"
	profileEnvFileName = ".aw-profile-env"
)

// EnvStage loads custom environment variables from the profile config,
// .aw-profile-env file (written by parent process), and .aw-env file,
// merging them into the execution context.
//
// Override priority (highest wins):
//  1. .aw-env (dynamic, from on-create hook, in WorkDir)
//  2. profile.Env (static, from current profile's env and env_file fields)
//  3. .aw-profile-env (static, written by parent process's profile env, in cache dir)
type EnvStage struct{}

// profileEnvCacheDir returns the cache directory for .aw-profile-env,
// scoped by OrigWorkDir hash to isolate concurrent workspaces.
func profileEnvCacheDir(homeDir, origWorkDir string) string {
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(origWorkDir)))[:12]
	return filepath.Join(homeDir, ".cache", "agent-workspace", "env", hash)
}

func (s *EnvStage) Name() string { return "env" }

func (s *EnvStage) Run(_ context.Context, ec *pipeline.ExecutionContext) error {
	merged := make(map[string]string)

	cacheDir := profileEnvCacheDir(ec.HomeDir, ec.OrigWorkDir)
	profileEnvFilePath := filepath.Join(cacheDir, profileEnvFileName)

	if len(ec.Profile.Env) > 0 {
		// 1. Start with .aw-profile-env (lowest priority, written by parent process)
		profileFileEnv, err := envfile.ParseFile(profileEnvFilePath)
		if err != nil {
			return fmt.Errorf("reading %s: %w", profileEnvFileName, err)
		}
		for k, v := range profileFileEnv {
			merged[k] = v
		}

		// 2. Overlay with current profile's env vars (env: plus env_file)
		for k, v := range ec.Profile.Env {
			merged[k] = v
		}
	}

	// 3. Write the current profile's env: vars to .aw-profile-env for child
	// processes. Values loaded from env_file are left out so they never
	// leak into a later, unrelated profile run from the same directory.
	if inheritable := ec.Profile.InheritableEnv(); len(inheritable) > 0 {
		if err := os.MkdirAll(cacheDir, 0700); err != nil {
			return fmt.Errorf("creating profile env cache dir: %w", err)
		}
		// MkdirAll leaves an existing directory's mode alone; tighten
		// caches created by older versions with 0755.
		if err := os.Chmod(cacheDir, 0700); err != nil {
			return fmt.Errorf("securing profile env cache dir: %w", err)
		}
		if err := envfile.WriteFile(profileEnvFilePath, inheritable); err != nil {
			return fmt.Errorf("writing %s: %w", profileEnvFileName, err)
		}
	} else {
		// Nothing to hand down — remove stale cache from previous runs
		_ = os.Remove(profileEnvFilePath)
	}

	// 4. Overlay with .aw-env file vars (highest priority, from on-create hook)
	envFilePath := filepath.Join(ec.WorkDir, envFileName)
	fileEnv, err := envfile.ParseFile(envFilePath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", envFileName, err)
	}
	for k, v := range fileEnv {
		merged[k] = v
	}

	if len(merged) > 0 {
		fmt.Fprintf(os.Stderr, "Loaded %d custom env var(s)\n", len(merged))
	}

	ec.EnvVars = merged
	return nil
}
