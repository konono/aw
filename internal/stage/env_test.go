package stage

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/konono/aw/v4/internal/pipeline"
	"github.com/konono/aw/v4/internal/profile"
)

func TestEnvStage_Name(t *testing.T) {
	s := &EnvStage{}
	if s.Name() != "env" {
		t.Errorf("Name() = %q, want %q", s.Name(), "env")
	}
}

func TestEnvStage_NoEnvNoFile(t *testing.T) {
	dir := t.TempDir()
	ec := &pipeline.ExecutionContext{
		Profile:     profile.Profile{},
		HomeDir:     t.TempDir(),
		OrigWorkDir: dir,
		WorkDir:     dir,
	}

	s := &EnvStage{}
	if err := s.Run(context.Background(), ec); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ec.EnvVars == nil {
		t.Fatal("EnvVars should not be nil")
	}
	if len(ec.EnvVars) != 0 {
		t.Errorf("got %d env vars, want 0", len(ec.EnvVars))
	}
}

func TestEnvStage_ProfileEnvOnly(t *testing.T) {
	dir := t.TempDir()
	ec := &pipeline.ExecutionContext{
		Profile: profile.Profile{
			Env: map[string]string{
				"FOO": "bar",
				"BAZ": "qux",
			},
		},
		HomeDir:     t.TempDir(),
		OrigWorkDir: dir,
		WorkDir:     dir,
	}

	s := &EnvStage{}
	if err := s.Run(context.Background(), ec); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ec.EnvVars["FOO"] != "bar" {
		t.Errorf("FOO = %q, want %q", ec.EnvVars["FOO"], "bar")
	}
	if ec.EnvVars["BAZ"] != "qux" {
		t.Errorf("BAZ = %q, want %q", ec.EnvVars["BAZ"], "qux")
	}
}

func TestEnvStage_FileOnly(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".aw-env")
	if err := os.WriteFile(envFile, []byte("KEY=value\n"), 0644); err != nil {
		t.Fatal(err)
	}

	ec := &pipeline.ExecutionContext{
		Profile:     profile.Profile{},
		HomeDir:     t.TempDir(),
		OrigWorkDir: dir,
		WorkDir:     dir,
	}

	s := &EnvStage{}
	if err := s.Run(context.Background(), ec); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ec.EnvVars["KEY"] != "value" {
		t.Errorf("KEY = %q, want %q", ec.EnvVars["KEY"], "value")
	}
}

func TestEnvStage_FileOverridesProfile(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".aw-env")
	if err := os.WriteFile(envFile, []byte("FOO=from-file\nFILE_ONLY=yes\n"), 0644); err != nil {
		t.Fatal(err)
	}

	ec := &pipeline.ExecutionContext{
		Profile: profile.Profile{
			Env: map[string]string{
				"FOO":          "from-profile",
				"PROFILE_ONLY": "yes",
			},
		},
		HomeDir:     t.TempDir(),
		OrigWorkDir: dir,
		WorkDir:     dir,
	}

	s := &EnvStage{}
	if err := s.Run(context.Background(), ec); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ec.EnvVars["FOO"] != "from-file" {
		t.Errorf("FOO = %q, want %q (file should override profile)", ec.EnvVars["FOO"], "from-file")
	}
	if ec.EnvVars["PROFILE_ONLY"] != "yes" {
		t.Errorf("PROFILE_ONLY = %q, want %q", ec.EnvVars["PROFILE_ONLY"], "yes")
	}
	if ec.EnvVars["FILE_ONLY"] != "yes" {
		t.Errorf("FILE_ONLY = %q, want %q", ec.EnvVars["FILE_ONLY"], "yes")
	}
}

func TestEnvStage_WritesProfileEnvFile(t *testing.T) {
	dir := t.TempDir()
	homeDir := t.TempDir()
	ec := &pipeline.ExecutionContext{
		Profile: profile.Profile{
			Env: map[string]string{
				"STATIC_KEY": "static_value",
			},
		},
		HomeDir:     homeDir,
		OrigWorkDir: dir,
		WorkDir:     dir,
	}

	s := &EnvStage{}
	if err := s.Run(context.Background(), ec); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// .aw-profile-env should have been written to cache dir
	cacheDir := profileEnvCacheDir(homeDir, dir)
	profileEnvFile := filepath.Join(cacheDir, ".aw-profile-env")
	data, err := os.ReadFile(profileEnvFile)
	if err != nil {
		t.Fatalf("expected .aw-profile-env to be created in cache dir: %v", err)
	}
	if string(data) != "STATIC_KEY=static_value\n" {
		t.Errorf("file content = %q, want %q", string(data), "STATIC_KEY=static_value\n")
	}
}

func TestEnvStage_CleansStaleProfileEnvFile(t *testing.T) {
	// When the current profile has no env vars, stale .aw-profile-env from a
	// previous run should be deleted so its vars don't leak into this session.
	dir := t.TempDir()
	homeDir := t.TempDir()
	cacheDir := profileEnvCacheDir(homeDir, dir)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatal(err)
	}
	profileEnvFile := filepath.Join(cacheDir, ".aw-profile-env")
	if err := os.WriteFile(profileEnvFile, []byte("STALE_KEY=stale_value\n"), 0644); err != nil {
		t.Fatal(err)
	}

	ec := &pipeline.ExecutionContext{
		Profile:     profile.Profile{}, // empty — no env vars
		HomeDir:     homeDir,
		OrigWorkDir: dir,
		WorkDir:     dir,
	}

	s := &EnvStage{}
	if err := s.Run(context.Background(), ec); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(ec.EnvVars) != 0 {
		t.Errorf("got %d env vars, want 0 (stale cache should not leak)", len(ec.EnvVars))
	}
	if _, err := os.Stat(profileEnvFile); !os.IsNotExist(err) {
		t.Error("stale .aw-profile-env should have been deleted")
	}
}

func TestEnvStage_ProfileEnvOverridesProfileEnvFile(t *testing.T) {
	dir := t.TempDir()
	homeDir := t.TempDir()
	cacheDir := profileEnvCacheDir(homeDir, dir)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatal(err)
	}
	profileEnvFile := filepath.Join(cacheDir, ".aw-profile-env")
	if err := os.WriteFile(profileEnvFile, []byte("SHARED=from-parent-file\nPARENT_ONLY=yes\n"), 0644); err != nil {
		t.Fatal(err)
	}

	ec := &pipeline.ExecutionContext{
		Profile: profile.Profile{
			Env: map[string]string{
				"SHARED":       "from-current-profile",
				"CURRENT_ONLY": "yes",
			},
		},
		HomeDir:     homeDir,
		OrigWorkDir: dir,
		WorkDir:     dir,
	}

	s := &EnvStage{}
	if err := s.Run(context.Background(), ec); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ec.EnvVars["SHARED"] != "from-current-profile" {
		t.Errorf("SHARED = %q, want %q (current profile should override profile env file)", ec.EnvVars["SHARED"], "from-current-profile")
	}
	if ec.EnvVars["PARENT_ONLY"] != "yes" {
		t.Errorf("PARENT_ONLY = %q, want %q", ec.EnvVars["PARENT_ONLY"], "yes")
	}
	if ec.EnvVars["CURRENT_ONLY"] != "yes" {
		t.Errorf("CURRENT_ONLY = %q, want %q", ec.EnvVars["CURRENT_ONLY"], "yes")
	}
}

func TestEnvStage_AwEnvOverridesAll(t *testing.T) {
	dir := t.TempDir()
	homeDir := t.TempDir()

	// .aw-profile-env in cache dir (lowest priority)
	cacheDir := profileEnvCacheDir(homeDir, dir)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatal(err)
	}
	profileEnvFile := filepath.Join(cacheDir, ".aw-profile-env")
	if err := os.WriteFile(profileEnvFile, []byte("KEY=from-profile-env-file\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// .aw-env in WorkDir (highest priority)
	awEnvFile := filepath.Join(dir, ".aw-env")
	if err := os.WriteFile(awEnvFile, []byte("KEY=from-aw-env\n"), 0644); err != nil {
		t.Fatal(err)
	}

	ec := &pipeline.ExecutionContext{
		Profile: profile.Profile{
			Env: map[string]string{
				"KEY": "from-current-profile",
			},
		},
		HomeDir:     homeDir,
		OrigWorkDir: dir,
		WorkDir:     dir,
	}

	s := &EnvStage{}
	if err := s.Run(context.Background(), ec); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ec.EnvVars["KEY"] != "from-aw-env" {
		t.Errorf("KEY = %q, want %q (.aw-env should override everything)", ec.EnvVars["KEY"], "from-aw-env")
	}
}

func TestEnvStage_DeletesCacheWhenProfileEnvEmpty(t *testing.T) {
	dir := t.TempDir()
	homeDir := t.TempDir()

	// Seed a stale cache file
	cacheDir := profileEnvCacheDir(homeDir, dir)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatal(err)
	}
	profileEnvFile := filepath.Join(cacheDir, ".aw-profile-env")
	if err := os.WriteFile(profileEnvFile, []byte("OLD=value\n"), 0644); err != nil {
		t.Fatal(err)
	}

	ec := &pipeline.ExecutionContext{
		Profile:     profile.Profile{}, // no Env
		HomeDir:     homeDir,
		OrigWorkDir: dir,
		WorkDir:     dir,
	}

	s := &EnvStage{}
	if err := s.Run(context.Background(), ec); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(profileEnvFile); !os.IsNotExist(err) {
		t.Error(".aw-profile-env should be deleted when profile.Env is empty")
	}
}

func TestEnvStage_InvalidFileReturnsError(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".aw-env")
	if err := os.WriteFile(envFile, []byte("INVALID_LINE_WITHOUT_EQUALS\n"), 0644); err != nil {
		t.Fatal(err)
	}

	ec := &pipeline.ExecutionContext{
		Profile:     profile.Profile{},
		HomeDir:     t.TempDir(),
		OrigWorkDir: dir,
		WorkDir:     dir,
	}

	s := &EnvStage{}
	err := s.Run(context.Background(), ec)
	if err == nil {
		t.Fatal("expected error for invalid .aw-env file")
	}
}

// Regression: env_file values of one profile must not reach a different
// profile launched later from the same work directory via .aw-profile-env,
// and the cache must be owner-only.
func TestEnvStage_EnvFileValuesNotInheritedAcrossProfiles(t *testing.T) {
	dir := t.TempDir()
	homeDir := t.TempDir()

	// Simulate a cache dir/file created by an older version with loose modes.
	cacheDir := profileEnvCacheDir(homeDir, dir)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatal(err)
	}
	cacheFile := filepath.Join(cacheDir, ".aw-profile-env")
	if err := os.WriteFile(cacheFile, []byte("OLD=1\n"), 0644); err != nil {
		t.Fatal(err)
	}

	secretFile := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(secretFile, []byte("SECRET_TOKEN=s3cr3t\nSHARED=file\n"), 0600); err != nil {
		t.Fatal(err)
	}

	// Run 1: profile A with env_file + env.
	pa := profile.Profile{
		Environment: profile.EnvironmentContainer,
		EnvFile:     profile.StringList{secretFile},
		Env:         map[string]string{"A_STATIC": "a", "SHARED": "yaml"},
	}
	if err := profile.ResolveEnvFile(&pa); err != nil {
		t.Fatalf("ResolveEnvFile: %v", err)
	}
	ecA := &pipeline.ExecutionContext{Profile: pa, HomeDir: homeDir, OrigWorkDir: dir, WorkDir: dir}
	if err := (&EnvStage{}).Run(context.Background(), ecA); err != nil {
		t.Fatalf("run A: %v", err)
	}
	if ecA.EnvVars["SECRET_TOKEN"] != "s3cr3t" || ecA.EnvVars["SHARED"] != "yaml" {
		t.Errorf("A EnvVars = %v", ecA.EnvVars)
	}

	data, err := os.ReadFile(cacheFile)
	if err != nil {
		t.Fatalf("reading cache: %v", err)
	}
	if strings.Contains(string(data), "SECRET_TOKEN") {
		t.Errorf("cache contains env_file value: %q", data)
	}
	if string(data) != "A_STATIC=a\nSHARED=yaml\n" {
		t.Errorf("cache = %q", data)
	}
	if fi, _ := os.Stat(cacheFile); fi.Mode().Perm() != 0600 {
		t.Errorf("cache file mode = %o, want 600", fi.Mode().Perm())
	}
	if di, _ := os.Stat(cacheDir); di.Mode().Perm() != 0700 {
		t.Errorf("cache dir mode = %o, want 700", di.Mode().Perm())
	}

	// Run 2: unrelated profile B with its own env, same work directory.
	ecB := &pipeline.ExecutionContext{
		Profile:     profile.Profile{Env: map[string]string{"B_STATIC": "b"}},
		HomeDir:     homeDir,
		OrigWorkDir: dir,
		WorkDir:     dir,
	}
	if err := (&EnvStage{}).Run(context.Background(), ecB); err != nil {
		t.Fatalf("run B: %v", err)
	}
	if _, ok := ecB.EnvVars["SECRET_TOKEN"]; ok {
		t.Errorf("profile B inherited env_file value from profile A: %v", ecB.EnvVars)
	}
	if ecB.EnvVars["B_STATIC"] != "b" {
		t.Errorf("B EnvVars = %v", ecB.EnvVars)
	}
}

// A profile whose env comes only from env_file writes no cache and removes
// a stale one.
func TestEnvStage_EnvFileOnlyWritesNoCache(t *testing.T) {
	dir := t.TempDir()
	homeDir := t.TempDir()
	cacheDir := profileEnvCacheDir(homeDir, dir)
	if err := os.MkdirAll(cacheDir, 0700); err != nil {
		t.Fatal(err)
	}
	cacheFile := filepath.Join(cacheDir, ".aw-profile-env")
	if err := os.WriteFile(cacheFile, []byte("STALE=1\n"), 0600); err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(f, []byte("ONLY_FILE=1\n"), 0600); err != nil {
		t.Fatal(err)
	}
	p := profile.Profile{Environment: profile.EnvironmentContainer, EnvFile: profile.StringList{f}}
	if err := profile.ResolveEnvFile(&p); err != nil {
		t.Fatal(err)
	}
	ec := &pipeline.ExecutionContext{Profile: p, HomeDir: homeDir, OrigWorkDir: dir, WorkDir: dir}
	if err := (&EnvStage{}).Run(context.Background(), ec); err != nil {
		t.Fatal(err)
	}
	if ec.EnvVars["ONLY_FILE"] != "1" {
		t.Errorf("EnvVars = %v", ec.EnvVars)
	}
	if _, err := os.Stat(cacheFile); !os.IsNotExist(err) {
		t.Errorf("cache should be removed, stat err = %v", err)
	}
}
