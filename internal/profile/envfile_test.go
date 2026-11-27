package profile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

func TestParse_EnvFileScalarAndList(t *testing.T) {
	cfg, err := Parse([]byte(`
env_file: .env
profiles:
  a:
    environment: container
    launch: shell
    env_file: [.env, .env.local]
  b:
    environment: container
    launch: shell
`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got := cfg.Defaults.EnvFile; len(got) != 1 || got[0] != ".env" {
		t.Errorf("defaults env_file = %v, want [.env]", got)
	}
	if got := cfg.Profiles["a"].EnvFile; len(got) != 2 || got[1] != ".env.local" {
		t.Errorf("profile a env_file = %v, want [.env .env.local]", got)
	}
}

func TestParse_EnvFileRejectsMap(t *testing.T) {
	_, err := Parse([]byte(`
profiles:
  a:
    env_file:
      foo: bar
`))
	if err == nil {
		t.Fatal("expected error for env_file mapping")
	}
}

func TestResolveEnvFilePaths(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home dir")
	}
	base := t.TempDir()
	abs := filepath.Join(t.TempDir(), ".env") // absolute on every OS
	cfg := &Config{
		Defaults: ProfileDefaults{EnvFile: StringList{".env"}},
		Profiles: map[string]Profile{
			"a": {EnvFile: StringList{"sub/.env", abs, "~/x.env"}},
			"b": {},
		},
	}
	resolveEnvFilePaths(cfg, base)

	if got, want := cfg.Defaults.EnvFile[0], filepath.Join(base, ".env"); got != want {
		t.Errorf("defaults = %q, want %q", got, want)
	}
	a := cfg.Profiles["a"].EnvFile
	want := []string{filepath.Join(base, "sub/.env"), abs, filepath.Join(home, "x.env")}
	for i := range want {
		if a[i] != want[i] {
			t.Errorf("a[%d] = %q, want %q", i, a[i], want[i])
		}
	}
	if cfg.Profiles["b"].EnvFile != nil {
		t.Errorf("b env_file = %v, want nil", cfg.Profiles["b"].EnvFile)
	}
}

func TestResolveEnvFile_ProfileEnvWins(t *testing.T) {
	dir := t.TempDir()
	f1 := filepath.Join(dir, ".env")
	f2 := filepath.Join(dir, ".env.local")
	writeFile(t, f1, "A=from-file1\nB=from-file1\nC=from-file1\n")
	writeFile(t, f2, "B=from-file2\n")

	p := Profile{
		EnvFile: StringList{f1, f2},
		Env:     map[string]string{"C": "from-yaml", "D": "yaml-only"},
	}
	if err := ResolveEnvFile(&p); err != nil {
		t.Fatalf("ResolveEnvFile: %v", err)
	}
	want := map[string]string{
		"A": "from-file1",
		"B": "from-file2", // later file wins
		"C": "from-yaml",  // .aw.yml env wins over file
		"D": "yaml-only",
	}
	if len(p.Env) != len(want) {
		t.Errorf("Env = %v, want %v", p.Env, want)
	}
	for k, v := range want {
		if p.Env[k] != v {
			t.Errorf("Env[%s] = %q, want %q", k, p.Env[k], v)
		}
	}
	if p.EnvFile != nil {
		t.Errorf("EnvFile = %v, want nil after resolve", p.EnvFile)
	}
}

func TestResolveEnvFile_MissingFile(t *testing.T) {
	p := Profile{EnvFile: StringList{filepath.Join(t.TempDir(), "nope.env")}}
	err := ResolveEnvFile(&p)
	if err == nil || !strings.Contains(err.Error(), "nope.env") {
		t.Fatalf("err = %v, want missing file error mentioning path", err)
	}
}

func TestResolveEnvFile_NoEnvFile(t *testing.T) {
	p := Profile{Env: map[string]string{"A": "1"}}
	if err := ResolveEnvFile(&p); err != nil {
		t.Fatalf("ResolveEnvFile: %v", err)
	}
	if len(p.Env) != 1 || p.Env["A"] != "1" {
		t.Errorf("Env = %v, want unchanged", p.Env)
	}
}

func TestMergeProfile_EnvFileOverrideReplaces(t *testing.T) {
	base := Profile{EnvFile: StringList{"/base.env"}}
	got := MergeProfile(base, Profile{})
	if len(got.EnvFile) != 1 || got.EnvFile[0] != "/base.env" {
		t.Errorf("inherit: %v", got.EnvFile)
	}
	got = MergeProfile(base, Profile{EnvFile: StringList{"/override.env"}})
	if len(got.EnvFile) != 1 || got.EnvFile[0] != "/override.env" {
		t.Errorf("override: %v", got.EnvFile)
	}
}

func TestValidate_EmptyEnvFileEntry(t *testing.T) {
	p := Profile{Environment: EnvironmentContainer, Launch: LaunchShell, EnvFile: StringList{""}}
	if err := Validate(p); err == nil {
		t.Fatal("expected error for empty env_file entry")
	}
}

func TestTrust_EnvFileIsSensitiveAndStripped(t *testing.T) {
	cfg := &Config{Profiles: map[string]Profile{"a": {EnvFile: StringList{".env"}}}}
	found := hasSensitiveFields(cfg)
	if len(found) != 1 || !strings.Contains(found[0], "env_file") {
		t.Errorf("sensitive fields = %v, want env_file", found)
	}
	stripped := stripProfileSensitive(cfg.Profiles["a"])
	if stripped.EnvFile != nil {
		t.Errorf("EnvFile not stripped: %v", stripped.EnvFile)
	}
}

func TestLoadFile_EnvFileRelativeToConfigDir(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".env"), "TOKEN=file\nSHARED=file\n")
	cfgPath := filepath.Join(dir, ".aw.yml")
	writeFile(t, cfgPath, `
profiles:
  a:
    environment: container
    launch: shell
    env_file: .env
    env:
      SHARED: yaml
`)
	t.Chdir(t.TempDir()) // resolution must not depend on cwd

	cfg, err := LoadFile(cfgPath)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	p := cfg.Profiles["a"]
	if err := ResolveEnvFile(&p); err != nil {
		t.Fatalf("ResolveEnvFile: %v", err)
	}
	if p.Env["TOKEN"] != "file" || p.Env["SHARED"] != "yaml" {
		t.Errorf("Env = %v", p.Env)
	}
}

func TestInheritableEnv_ExcludesFileOnlyKeys(t *testing.T) {
	f := filepath.Join(t.TempDir(), ".env")
	writeFile(t, f, "FILE_ONLY=1\nBOTH=file\n")
	p := Profile{EnvFile: StringList{f}, Env: map[string]string{"BOTH": "yaml", "YAML_ONLY": "y"}}
	if err := ResolveEnvFile(&p); err != nil {
		t.Fatal(err)
	}
	got := p.InheritableEnv()
	if _, ok := got["FILE_ONLY"]; ok {
		t.Errorf("InheritableEnv includes env_file key: %v", got)
	}
	if got["BOTH"] != "yaml" || got["YAML_ONLY"] != "y" || len(got) != 2 {
		t.Errorf("InheritableEnv = %v", got)
	}
}

func TestValidate_EnvFileRejectedOnHost(t *testing.T) {
	p := Profile{Environment: EnvironmentHost, Launch: LaunchShell, EnvFile: StringList{"/x.env"}}
	if err := Validate(p); err == nil || !strings.Contains(err.Error(), "container") {
		t.Fatalf("err = %v, want container-only error", err)
	}
}
