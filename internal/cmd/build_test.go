package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/konono/aw/internal/profile"
)

func TestParseBuildIncludes(t *testing.T) {
	t.Run("valid single", func(t *testing.T) {
		inc, err := parseBuildIncludes([]string{"./certs:/usr/local/share/ca-certificates"})
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if len(inc) != 1 {
			t.Fatalf("len = %d, want 1", len(inc))
		}
		if inc[0].Src != "./certs" || inc[0].Dst != "/usr/local/share/ca-certificates" {
			t.Fatalf("got %+v", inc[0])
		}
	})

	t.Run("valid multiple", func(t *testing.T) {
		inc, err := parseBuildIncludes([]string{"./a:/a", "./b:/b"})
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if len(inc) != 2 {
			t.Fatalf("len = %d, want 2", len(inc))
		}
	})

	t.Run("bad format no colon", func(t *testing.T) {
		_, err := parseBuildIncludes([]string{"nodelimiter"})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("bad format empty src", func(t *testing.T) {
		_, err := parseBuildIncludes([]string{":/dst"})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("bad format empty dst", func(t *testing.T) {
		_, err := parseBuildIncludes([]string{"src:"})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("empty list", func(t *testing.T) {
		inc, err := parseBuildIncludes(nil)
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if len(inc) != 0 {
			t.Fatalf("len = %d, want 0", len(inc))
		}
	})
}

func TestMergeBuildFields(t *testing.T) {
	t.Run("nil config", func(t *testing.T) {
		inc, env := mergeBuildFields(nil, nil, nil)
		if len(inc) != 0 {
			t.Errorf("includes should be empty, got %v", inc)
		}
		if len(env) != 0 {
			t.Errorf("env should be empty, got %v", env)
		}
	})

	t.Run("config only", func(t *testing.T) {
		cfg := &profile.BuildConfig{
			Include: []profile.BuildInclude{{Src: "./a", Dst: "/a"}},
			Env:     map[string]string{"X": "1"},
		}
		inc, env := mergeBuildFields(nil, nil, cfg)
		if len(inc) != 1 || inc[0].Src != "./a" {
			t.Errorf("includes = %v, want [{./a /a}]", inc)
		}
		if env["X"] != "1" {
			t.Errorf("env[X] = %q, want %q", env["X"], "1")
		}
	})

	t.Run("cli overrides config env", func(t *testing.T) {
		cfg := &profile.BuildConfig{
			Env: map[string]string{"A": "from-config", "B": "keep"},
		}
		_, env := mergeBuildFields(nil, map[string]string{"A": "from-cli"}, cfg)
		if env["A"] != "from-cli" {
			t.Errorf("env[A] = %q, want %q (cli should override)", env["A"], "from-cli")
		}
		if env["B"] != "keep" {
			t.Errorf("env[B] = %q, want %q (should be preserved)", env["B"], "keep")
		}
	})

	t.Run("cli and config includes combine", func(t *testing.T) {
		cfg := &profile.BuildConfig{
			Include: []profile.BuildInclude{{Src: "./from-config", Dst: "/config"}},
		}
		inc, _ := mergeBuildFields([]profile.BuildInclude{{Src: "./from-cli", Dst: "/cli"}}, nil, cfg)
		if len(inc) != 2 {
			t.Errorf("includes len = %d, want 2 (config + cli)", len(inc))
		}
	})
}

func TestComputeBuildImageName(t *testing.T) {
	emptyDir := t.TempDir()

	name := computeBuildImageName("claude", "ghcr.io/konono/aw-claude:3.5.0-debian12", nil, nil, emptyDir)
	if !strings.HasPrefix(name, "aw-build:claude-") {
		t.Errorf("expected prefix 'aw-build:claude-', got %q", name)
	}
	parts := strings.SplitN(name, "-", 3)
	if len(parts) < 3 {
		t.Fatalf("expected 'aw-build:claude-<hash>', got %q", name)
	}
	hash := parts[2]
	if len(hash) != 12 {
		t.Errorf("hash should be 12 chars, got %d (%q)", len(hash), hash)
	}

	name2 := computeBuildImageName("claude", "ghcr.io/konono/aw-claude:3.5.0-debian12", nil, nil, emptyDir)
	if name != name2 {
		t.Errorf("same inputs should produce same name: %q != %q", name, name2)
	}

	name3 := computeBuildImageName("claude", "different-base:image", nil, nil, emptyDir)
	if name == name3 {
		t.Errorf("different base images should produce different names")
	}

	name4 := computeBuildImageName("claude", "ghcr.io/konono/aw-claude:3.5.0-debian12",
		[]profile.BuildInclude{{Src: "./certs", Dst: "/certs"}}, nil, emptyDir)
	if name == name4 {
		t.Errorf("different includes should produce different names")
	}

	name5 := computeBuildImageName("claude", "ghcr.io/konono/aw-claude:3.5.0-debian12",
		nil, map[string]string{"HTTP_PROXY": "http://proxy:8080"}, emptyDir)
	if name == name5 {
		t.Errorf("different env vars should produce different names")
	}

	name6 := computeBuildImageName("my/custom profile", "base:img", nil, nil, emptyDir)
	if strings.ContainsAny(name6[strings.Index(name6, ":")+1:], "/ ") {
		t.Errorf("profile name with special chars should be sanitized in tag, got %q", name6)
	}

	t.Run("different workspace files produce different names", func(t *testing.T) {
		dirA := t.TempDir()
		dirB := t.TempDir()
		if err := os.WriteFile(filepath.Join(dirA, "mise.toml"), []byte("[tools]\nnode = \"20\"\n"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dirB, "mise.toml"), []byte("[tools]\npython = \"3.12\"\n"), 0644); err != nil {
			t.Fatal(err)
		}
		nameA := computeBuildImageName("claude", "base:img", nil, nil, dirA)
		nameB := computeBuildImageName("claude", "base:img", nil, nil, dirB)
		if nameA == nameB {
			t.Errorf("different workspace mise.toml should produce different names")
		}
	})
}

func TestHasWorkspaceFiles(t *testing.T) {
	t.Run("empty dir", func(t *testing.T) {
		dir := t.TempDir()
		if hasWorkspaceFiles(dir) {
			t.Error("should return false for empty dir")
		}
	})

	for _, name := range []string{"mise.toml", ".mise.toml", "devbox.json", "packages.txt"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, name), []byte("test"), 0644); err != nil {
				t.Fatal(err)
			}
			if !hasWorkspaceFiles(dir) {
				t.Errorf("should return true when %s exists", name)
			}
		})
	}
}

func TestHasBuildInputs(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		dir := t.TempDir()
		if hasBuildInputs(dir, nil, nil, profile.Profile{}) {
			t.Error("should return false with no inputs")
		}
	})

	t.Run("workspace file", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "mise.toml"), []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}
		if !hasBuildInputs(dir, nil, nil, profile.Profile{}) {
			t.Error("should return true with workspace file")
		}
	})

	t.Run("includes", func(t *testing.T) {
		dir := t.TempDir()
		if !hasBuildInputs(dir, []profile.BuildInclude{{Src: "./a", Dst: "/a"}}, nil, profile.Profile{}) {
			t.Error("should return true with includes")
		}
	})

	t.Run("env vars", func(t *testing.T) {
		dir := t.TempDir()
		if !hasBuildInputs(dir, nil, map[string]string{"K": "V"}, profile.Profile{}) {
			t.Error("should return true with env vars")
		}
	})

	t.Run("profile packages", func(t *testing.T) {
		dir := t.TempDir()
		p := profile.Profile{Packages: []string{"jq"}}
		if !hasBuildInputs(dir, nil, nil, p) {
			t.Error("should return true with profile packages")
		}
	})

	t.Run("build config includes via merged params", func(t *testing.T) {
		dir := t.TempDir()
		incl := []profile.BuildInclude{{Src: "./certs", Dst: "/certs"}}
		env := map[string]string{"HTTP_PROXY": "http://proxy:8080"}
		if !hasBuildInputs(dir, incl, env, profile.Profile{}) {
			t.Error("should return true with merged build config includes and env")
		}
	})
}

func TestExportNeedsSnapshot(t *testing.T) {
	t.Run("no flags no config", func(t *testing.T) {
		if exportNeedsSnapshot(false, nil, nil, nil) {
			t.Error("should be false with no flags and no config")
		}
	})

	t.Run("snapshot flag", func(t *testing.T) {
		if !exportNeedsSnapshot(true, nil, nil, nil) {
			t.Error("should be true with --snapshot flag")
		}
	})

	t.Run("include flag implies snapshot", func(t *testing.T) {
		if !exportNeedsSnapshot(false, []string{"./a:/a"}, nil, nil) {
			t.Error("should be true with --include flag")
		}
	})

	t.Run("env flag implies snapshot", func(t *testing.T) {
		if !exportNeedsSnapshot(false, nil, map[string]string{"K": "V"}, nil) {
			t.Error("should be true with --env flag")
		}
	})

	t.Run("profile LegacySnapshot", func(t *testing.T) {
		cfg := &profile.BuildConfig{LegacySnapshot: true}
		if !exportNeedsSnapshot(false, nil, nil, cfg) {
			t.Error("should be true with profile LegacySnapshot")
		}
	})

	t.Run("profile include implies snapshot", func(t *testing.T) {
		cfg := &profile.BuildConfig{
			Include: []profile.BuildInclude{{Src: "./a", Dst: "/a"}},
		}
		if !exportNeedsSnapshot(false, nil, nil, cfg) {
			t.Error("should be true with profile includes")
		}
	})

	t.Run("profile env implies snapshot", func(t *testing.T) {
		cfg := &profile.BuildConfig{
			Env: map[string]string{"K": "V"},
		}
		if !exportNeedsSnapshot(false, nil, nil, cfg) {
			t.Error("should be true with profile env")
		}
	})

	t.Run("empty profile config", func(t *testing.T) {
		cfg := &profile.BuildConfig{}
		if exportNeedsSnapshot(false, nil, nil, cfg) {
			t.Error("should be false with empty profile config")
		}
	})
}

func TestApplyBuildResult(t *testing.T) {
	t.Run("adds image to profile with apt", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "config.yml")
		if err := os.WriteFile(cfgPath, []byte(`# comment
profiles:
  dev:
    launch: shell
`), 0644); err != nil {
			t.Fatal(err)
		}

		if err := applyBuildResult(cfgPath, "dev", "aw-build:dev-abc123", profile.PackageManagerApt, true); err != nil {
			t.Fatalf("applyBuildResult() error = %v", err)
		}

		data, _ := os.ReadFile(cfgPath)
		content := string(data)
		if !strings.Contains(content, "image: aw-build:dev-abc123") {
			t.Errorf("config should contain image, got:\n%s", content)
		}
		if !strings.Contains(content, "skip_mise_install: true") {
			t.Errorf("config should contain skip_mise_install: true, got:\n%s", content)
		}
		if strings.Contains(content, "skip_devbox_install") {
			t.Errorf("apt mode should not write skip_devbox_install, got:\n%s", content)
		}
		if !strings.Contains(content, "# comment") {
			t.Error("comment should be preserved")
		}
	})

	t.Run("adds both skip flags with devbox", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "config.yml")
		if err := os.WriteFile(cfgPath, []byte(`profiles:
  dev:
    launch: shell
`), 0644); err != nil {
			t.Fatal(err)
		}

		if err := applyBuildResult(cfgPath, "dev", "aw-build:dev-abc123", profile.PackageManagerDevbox, true); err != nil {
			t.Fatalf("applyBuildResult() error = %v", err)
		}

		data, _ := os.ReadFile(cfgPath)
		content := string(data)
		if !strings.Contains(content, "skip_devbox_install: true") {
			t.Errorf("config should contain skip_devbox_install, got:\n%s", content)
		}
		if !strings.Contains(content, "skip_mise_install: true") {
			t.Errorf("config should contain skip_mise_install, got:\n%s", content)
		}
	})

	t.Run("updates existing image", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "config.yml")
		if err := os.WriteFile(cfgPath, []byte(`profiles:
  dev:
    launch: shell
    image: old-image:123
`), 0644); err != nil {
			t.Fatal(err)
		}

		if err := applyBuildResult(cfgPath, "dev", "aw-build:dev-new456", profile.PackageManagerApt, true); err != nil {
			t.Fatalf("applyBuildResult() error = %v", err)
		}

		data, _ := os.ReadFile(cfgPath)
		content := string(data)
		if !strings.Contains(content, "image: aw-build:dev-new456") {
			t.Errorf("image should be updated, got:\n%s", content)
		}
		if strings.Contains(content, "old-image") {
			t.Error("old image should be replaced")
		}
	})

	t.Run("creates profile if not in file", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "config.yml")
		if err := os.WriteFile(cfgPath, []byte(`profiles:
  shell:
    launch: shell
`), 0644); err != nil {
			t.Fatal(err)
		}

		if err := applyBuildResult(cfgPath, "newprofile", "aw-build:newprofile-abc", profile.PackageManagerApt, true); err != nil {
			t.Fatalf("applyBuildResult() error = %v", err)
		}

		data, _ := os.ReadFile(cfgPath)
		content := string(data)
		if !strings.Contains(content, "newprofile") {
			t.Errorf("new profile should be added, got:\n%s", content)
		}
		if !strings.Contains(content, "image: aw-build:newprofile-abc") {
			t.Errorf("image should be set, got:\n%s", content)
		}
		if !strings.Contains(content, "launch: shell") {
			t.Error("existing profile should be preserved")
		}
	})

	t.Run("creates profiles section if missing", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "config.yml")
		if err := os.WriteFile(cfgPath, []byte(`default: claude
`), 0644); err != nil {
			t.Fatal(err)
		}

		if err := applyBuildResult(cfgPath, "dev", "aw-build:dev-456", profile.PackageManagerApt, true); err != nil {
			t.Fatalf("applyBuildResult() error = %v", err)
		}

		data, _ := os.ReadFile(cfgPath)
		content := string(data)
		if !strings.Contains(content, "profiles") {
			t.Errorf("profiles section should be created, got:\n%s", content)
		}
		if !strings.Contains(content, "image: aw-build:dev-456") {
			t.Errorf("image should be set, got:\n%s", content)
		}
	})

	t.Run("apt mode removes stale skip_devbox_install", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "config.yml")
		if err := os.WriteFile(cfgPath, []byte(`profiles:
  dev:
    launch: shell
    image: old:123
    skip_devbox_install: true
    skip_mise_install: true
`), 0644); err != nil {
			t.Fatal(err)
		}

		if err := applyBuildResult(cfgPath, "dev", "aw-build:dev-new", profile.PackageManagerApt, true); err != nil {
			t.Fatalf("applyBuildResult() error = %v", err)
		}

		data, _ := os.ReadFile(cfgPath)
		content := string(data)
		if strings.Contains(content, "skip_devbox_install") {
			t.Errorf("apt mode should remove stale skip_devbox_install, got:\n%s", content)
		}
		if !strings.Contains(content, "skip_mise_install: true") {
			t.Errorf("skip_mise_install should remain true, got:\n%s", content)
		}
	})

	t.Run("no-snapshot clears skip flags", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "config.yml")
		if err := os.WriteFile(cfgPath, []byte(`profiles:
  dev:
    launch: shell
    image: old:123
    skip_devbox_install: true
    skip_mise_install: true
`), 0644); err != nil {
			t.Fatal(err)
		}

		if err := applyBuildResult(cfgPath, "dev", "aw-build:dev-new", profile.PackageManagerApt, false); err != nil {
			t.Fatalf("applyBuildResult() error = %v", err)
		}

		data, _ := os.ReadFile(cfgPath)
		content := string(data)
		if strings.Contains(content, "skip_devbox_install: true") {
			t.Errorf("skip_devbox_install should be false, got:\n%s", content)
		}
		if strings.Contains(content, "skip_mise_install: true") {
			t.Errorf("skip_mise_install should be false, got:\n%s", content)
		}
	})

	t.Run("creates new file when not exists", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, ".aw.yml")

		if err := applyBuildResult(cfgPath, "claude", "aw-build:claude-abc123", profile.PackageManagerApt, true); err != nil {
			t.Fatalf("applyBuildResult() error = %v", err)
		}

		data, err := os.ReadFile(cfgPath)
		if err != nil {
			t.Fatalf("file should be created: %v", err)
		}
		content := string(data)
		if !strings.Contains(content, "aw-build:claude-abc123") {
			t.Errorf("config should contain image name, got:\n%s", content)
		}
		if !strings.Contains(content, "claude") {
			t.Errorf("config should contain profile name, got:\n%s", content)
		}
		cfg, err := profile.Parse(data)
		if err != nil {
			t.Fatalf("created file should be valid YAML: %v", err)
			return
		}
		p, ok := cfg.Profiles["claude"]
		if !ok {
			t.Fatalf("profile 'claude' should exist in created config")
			return
		}
		if p.Image != "aw-build:claude-abc123" {
			t.Errorf("image = %q, want %q", p.Image, "aw-build:claude-abc123")
		}
	})
}
