package cmd

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/konono/aw/v4/internal/containerenv"
	"github.com/konono/aw/v4/internal/docker"
	"github.com/konono/aw/v4/internal/pipeline"
	"github.com/konono/aw/v4/internal/profile"
	"github.com/konono/aw/v4/internal/stage"
)

func TestBuildApplyFlag(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{"default applies", []string{"build", "dev"}, true},
		{"explicit --apply", []string{"build", "dev", "--apply"}, true},
		{"--no-apply opts out", []string{"build", "dev", "--no-apply"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cli CLI
			parser, err := kong.New(&cli, kong.Name("aw"), kong.Exit(func(int) {}))
			if err != nil {
				t.Fatalf("kong.New: %v", err)
			}
			if _, err := parser.Parse(tt.args); err != nil {
				t.Fatalf("parse %v: %v", tt.args, err)
			}
			if cli.Build.Apply != tt.want {
				t.Fatalf("Apply = %v, want %v", cli.Build.Apply, tt.want)
			}
		})
	}
}

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

	for _, name := range []string{"mise.toml", ".mise.toml", "packages.txt"} {
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

func buildInputsEC(dir string, p profile.Profile) *pipeline.ExecutionContext {
	return &pipeline.ExecutionContext{Profile: p, OrigWorkDir: dir}
}

func TestHasBuildInputs(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		dir := t.TempDir()
		if hasBuildInputs(buildInputsEC(dir, profile.Profile{}), nil, nil) {
			t.Error("should return false with no inputs")
		}
	})

	t.Run("pinned image alone is not a build input", func(t *testing.T) {
		dir := t.TempDir()
		p := profile.Profile{Image: "local/custom:latest"}
		if hasBuildInputs(buildInputsEC(dir, p), nil, nil) {
			t.Error("should return false: there is nothing to bake onto the pinned image")
		}
	})

	t.Run("workspace file", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "mise.toml"), []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}
		if !hasBuildInputs(buildInputsEC(dir, profile.Profile{}), nil, nil) {
			t.Error("should return true with workspace file")
		}
	})

	t.Run("includes", func(t *testing.T) {
		dir := t.TempDir()
		if !hasBuildInputs(buildInputsEC(dir, profile.Profile{}), []profile.BuildInclude{{Src: "./a", Dst: "/a"}}, nil) {
			t.Error("should return true with includes")
		}
	})

	t.Run("env vars", func(t *testing.T) {
		dir := t.TempDir()
		if !hasBuildInputs(buildInputsEC(dir, profile.Profile{}), nil, map[string]string{"K": "V"}) {
			t.Error("should return true with env vars")
		}
	})

	t.Run("profile packages", func(t *testing.T) {
		dir := t.TempDir()
		p := profile.Profile{Packages: []string{"jq"}}
		if !hasBuildInputs(buildInputsEC(dir, p), nil, nil) {
			t.Error("should return true with profile packages")
		}
	})

	t.Run("build config includes via merged params", func(t *testing.T) {
		dir := t.TempDir()
		incl := []profile.BuildInclude{{Src: "./certs", Dst: "/certs"}}
		env := map[string]string{"HTTP_PROXY": "http://proxy:8080"}
		if !hasBuildInputs(buildInputsEC(dir, profile.Profile{}), incl, env) {
			t.Error("should return true with merged build config includes and env")
		}
	})

	t.Run("build_env", func(t *testing.T) {
		dir := t.TempDir()
		p := profile.Profile{BuildEnv: map[string]string{"GITHUB_TOKEN": "xxx"}}
		if !hasBuildInputs(buildInputsEC(dir, p), nil, nil) {
			t.Error("should return true with build_env")
		}
	})

	t.Run("profile dockerfile", func(t *testing.T) {
		dir := t.TempDir()
		p := profile.Profile{Dockerfile: "docker/Dockerfile.custom"}
		if !hasBuildInputs(buildInputsEC(dir, p), nil, nil) {
			t.Error("should return true with profile dockerfile")
		}
	})

	t.Run("build_env from cli build-arg merge", func(t *testing.T) {
		dir := t.TempDir()
		p := profile.Profile{}
		p.BuildEnv = map[string]string{"HTTP_PROXY": "http://proxy:8080"}
		if !hasBuildInputs(buildInputsEC(dir, p), nil, nil) {
			t.Error("should return true when BuildEnv is set via CLI --build-arg merge")
		}
	})

	// These three need Dockerfile layers, so they must not fall through to the
	// official image. hasBuildInputs defers to stage.HasBuildCustomizations so
	// the two predicates cannot drift apart.
	t.Run("ca_cert", func(t *testing.T) {
		dir := t.TempDir()
		p := profile.Profile{CACert: "certs/corp-ca.pem"}
		if !hasBuildInputs(buildInputsEC(dir, p), nil, nil) {
			t.Error("should return true with ca_cert")
		}
	})

	t.Run("non-default container_user", func(t *testing.T) {
		dir := t.TempDir()
		p := profile.Profile{ContainerUser: "dev"}
		if !hasBuildInputs(buildInputsEC(dir, p), nil, nil) {
			t.Error("should return true with a non-default container_user")
		}
	})

	t.Run("default container_user is not a build input", func(t *testing.T) {
		dir := t.TempDir()
		p := profile.Profile{ContainerUser: "agent"}
		if hasBuildInputs(buildInputsEC(dir, p), nil, nil) {
			t.Error("should return false: agent is what the official image already uses")
		}
	})

	t.Run("mount_zellij", func(t *testing.T) {
		dir := t.TempDir()
		mountZellij := true
		p := profile.Profile{MountZellij: &mountZellij}
		if hasBuildInputs(buildInputsEC(dir, p), nil, nil) {
			t.Error("should return false: the official image already ships zellij, panecom and aw-sockrelay")
		}
	})

	t.Run("kubernetes session_log", func(t *testing.T) {
		dir := t.TempDir()
		p := profile.Profile{Kubernetes: &profile.KubernetesConfig{SessionLog: true}}
		if !hasBuildInputs(buildInputsEC(dir, p), nil, nil) {
			t.Error("should return true with kubernetes.session_log")
		}
	})
}

func TestPrepareBuildProfile(t *testing.T) {
	t.Run("image preserved when no dockerfile and no no-cache", func(t *testing.T) {
		p := profile.Profile{Image: "my-image:latest"}
		prepareBuildProfile(&p, false)
		if p.Image != "my-image:latest" {
			t.Error("image should be preserved")
		}
	})

	t.Run("image cleared with dockerfile", func(t *testing.T) {
		p := profile.Profile{Image: "my-image:latest", Dockerfile: "Dockerfile"}
		prepareBuildProfile(&p, false)
		if p.Image != "" {
			t.Error("image should be cleared when dockerfile is set")
		}
	})

	t.Run("image cleared with no-cache", func(t *testing.T) {
		p := profile.Profile{Image: "my-image:latest"}
		prepareBuildProfile(&p, true)
		if p.Image != "" {
			t.Error("image should be cleared when no-cache is true")
		}
	})

	t.Run("skip flags cleared", func(t *testing.T) {
		tr := true
		p := profile.Profile{SkipMiseInstall: &tr}
		prepareBuildProfile(&p, false)
		if p.SkipMiseInstall != nil {
			t.Error("SkipMiseInstall should be nil")
		}
	})

	t.Run("no-cache sets image pull policy to build", func(t *testing.T) {
		p := profile.Profile{}
		prepareBuildProfile(&p, true)
		if p.ImagePullPolicy != profile.ImagePullPolicyBuild {
			t.Errorf("ImagePullPolicy should be 'build', got %q", p.ImagePullPolicy)
		}
	})
}

func TestPrepareBuildImageSelection_PackagesUseTemplate(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "packages.txt"), []byte("gcc\n"), 0644); err != nil {
		t.Fatal(err)
	}
	ec := &pipeline.ExecutionContext{
		OrigWorkDir: dir,
		Profile: profile.Profile{
			Image:           "ghcr.io/team/base:latest",
			ImagePullPolicy: profile.ImagePullPolicyAuto,
		},
	}
	prepareBuildImageSelection(ec)
	if ec.Profile.Image != "" || ec.Profile.ImagePullPolicy != profile.ImagePullPolicyBuild {
		t.Fatalf("aw build must bake packages into a template image, got image=%q policy=%q", ec.Profile.Image, ec.Profile.ImagePullPolicy)
	}
}

func TestBuildCmd_Validate_BuildArgAWPrefix(t *testing.T) {
	b := BuildCmd{
		ProfileName: "test",
		NoCache:     true,
		BuildArg:    map[string]string{"AW_FOO": "bar"},
	}
	err := b.Validate()
	if err == nil {
		t.Fatal("expected error for AW_* prefix build arg")
	}
	if !strings.Contains(err.Error(), "AW_FOO") {
		t.Errorf("error should mention the key, got: %v", err)
	}

	b.BuildArg = map[string]string{"GITHUB_TOKEN": "xxx"}
	if err := b.Validate(); err != nil {
		t.Errorf("non-AW_ key should pass validation: %v", err)
	}
}

func TestApplyBuildResult(t *testing.T) {
	// aw save pins the toggle because the container it commits already has its
	// tools installed. aw build passes nil; see TestApplyBuildResult_NilToggle.
	skipTrue, skipFalse := true, false

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

		if err := applyBuildResult(cfgPath, "dev", "aw-build:dev-abc123", &skipTrue); err != nil {
			t.Fatalf("applyBuildResult() error = %v", err)
		}

		data, _ := os.ReadFile(cfgPath)
		content := string(data)
		if !strings.Contains(content, "image: aw-build:dev-abc123") {
			t.Errorf("config should contain image, got:\n%s", content)
		}
		// A profile with no toggle yet gets the current key, not the
		// deprecated skip_mise_install.
		if !strings.Contains(content, "mise_install: false") {
			t.Errorf("config should contain mise_install: false, got:\n%s", content)
		}
		if strings.Contains(content, "skip_mise_install") {
			t.Errorf("the deprecated key should not be introduced, got:\n%s", content)
		}
		if strings.Contains(content, "skip_devbox_install") {
			t.Errorf("apt mode should not write skip_devbox_install, got:\n%s", content)
		}
		if !strings.Contains(content, "# comment") {
			t.Error("comment should be preserved")
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

		if err := applyBuildResult(cfgPath, "dev", "aw-build:dev-new456", &skipTrue); err != nil {
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

		if err := applyBuildResult(cfgPath, "newprofile", "aw-build:newprofile-abc", &skipTrue); err != nil {
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

		if err := applyBuildResult(cfgPath, "dev", "aw-build:dev-456", &skipTrue); err != nil {
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

		if err := applyBuildResult(cfgPath, "dev", "aw-build:dev-new", &skipTrue); err != nil {
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

		if err := applyBuildResult(cfgPath, "dev", "aw-build:dev-new", &skipFalse); err != nil {
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

		if err := applyBuildResult(cfgPath, "claude", "aw-build:claude-abc123", &skipTrue); err != nil {
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

// aw run validates the whole config before doing anything; build and manifest
// are entered directly, so they validate the profile they act on. Without that
// a config carrying a removed key reaches the build unchecked.
func TestBuildAndManifest_ValidateTargetProfile(t *testing.T) {
	dir := t.TempDir()
	dir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "init", dir).Run(); err != nil {
		t.Skipf("git init failed: %v", err)
	}
	cfg := `profiles:
  legacy:
    environment: container
    launch: claude
    package_manager: devbox
  fine:
    environment: container
    launch: claude
`
	if err := os.WriteFile(filepath.Join(dir, ".aw.yml"), []byte(cfg), 0644); err != nil {
		t.Fatal(err)
	}

	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	t.Run("build rejects the legacy profile", func(t *testing.T) {
		b := BuildCmd{ProfileName: "legacy"}
		err := b.Run()
		if err == nil {
			t.Fatal("expected build to reject package_manager: devbox")
		}
		if !strings.Contains(err.Error(), "package_manager") {
			t.Errorf("error should name package_manager, got: %v", err)
		}
	})

	t.Run("manifest rejects the legacy profile", func(t *testing.T) {
		m := ManifestCmd{ProfileName: "legacy"}
		err := m.Run()
		if err == nil {
			t.Fatal("expected manifest to reject package_manager: devbox")
		}
		if !strings.Contains(err.Error(), "package_manager") {
			t.Errorf("error should name package_manager, got: %v", err)
		}
	})

	// An unrelated legacy profile must not block a healthy one: aw-manager's
	// .aw.yml carries one, and its `aw manifest k8s-claude` has to keep working
	// until that config is cleaned up.
	t.Run("manifest accepts a healthy profile alongside it", func(t *testing.T) {
		m := ManifestCmd{ProfileName: "fine", Output: t.TempDir()}
		if err := m.Run(); err != nil {
			t.Fatalf("healthy profile should not be blocked: %v", err)
		}
	})
}

func TestRenderSnapshotScript_MiseFingerprint(t *testing.T) {
	cenv := containerenv.Default()

	t.Run("bakes the fingerprint in", func(t *testing.T) {
		script, err := renderSnapshotScript(cenv, "sha256:abc123")
		if err != nil {
			t.Fatalf("renderSnapshotScript() error = %v", err)
		}
		if !strings.Contains(script, "printf '%s\\n' 'sha256:abc123' > /home/agent/.aw_mise_fingerprint") {
			t.Errorf("script should write the fingerprint:\n%s", script)
		}
	})

	t.Run("writes nothing when there is no fingerprint", func(t *testing.T) {
		script, err := renderSnapshotScript(cenv, "")
		if err != nil {
			t.Fatalf("renderSnapshotScript() error = %v", err)
		}
		if strings.Contains(script, "printf '%s\\n' '' >") {
			t.Errorf("an empty fingerprint must not be written:\n%s", script)
		}
	})

	t.Run("always clears whatever the base image recorded", func(t *testing.T) {
		for _, fp := range []string{"sha256:abc123", ""} {
			script, err := renderSnapshotScript(cenv, fp)
			if err != nil {
				t.Fatalf("renderSnapshotScript() error = %v", err)
			}
			if !strings.Contains(script, "rm -f /home/agent/.aw_mise_fingerprint") {
				t.Errorf("fingerprint %q: script must drop an inherited fingerprint:\n%s", fp, script)
			}
		}
	})
}

func TestApplyBuildResult_NilToggle(t *testing.T) {
	// aw build leaves the install toggle to the entrypoint's fingerprint
	// check, and must not overwrite an explicit opt-out while doing so.
	t.Run("leaves an explicit mise_install alone", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "config.yml")
		if err := os.WriteFile(cfgPath, []byte("profiles:\n  dev:\n    launch: claude\n    mise_install: false\n"), 0644); err != nil {
			t.Fatal(err)
		}

		if err := applyBuildResult(cfgPath, "dev", "aw-build:dev-abc", nil); err != nil {
			t.Fatalf("applyBuildResult() error = %v", err)
		}

		out, err := os.ReadFile(cfgPath)
		if err != nil {
			t.Fatal(err)
		}
		got := string(out)
		if !strings.Contains(got, "image: aw-build:dev-abc") {
			t.Errorf("image should be written:\n%s", got)
		}
		if !strings.Contains(got, "mise_install: false") {
			t.Errorf("the user's opt-out must survive:\n%s", got)
		}
		// Writing skip_mise_install next to mise_install makes the next
		// profile.Validate fail: the two keys are mutually exclusive.
		if strings.Contains(got, "skip_mise_install") {
			t.Errorf("aw build must not add skip_mise_install:\n%s", got)
		}
	})

	t.Run("adds no toggle to a profile without one", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "config.yml")
		if err := os.WriteFile(cfgPath, []byte("profiles:\n  dev:\n    launch: claude\n"), 0644); err != nil {
			t.Fatal(err)
		}

		if err := applyBuildResult(cfgPath, "dev", "aw-build:dev-abc", nil); err != nil {
			t.Fatalf("applyBuildResult() error = %v", err)
		}

		out, err := os.ReadFile(cfgPath)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(out), "mise_install") {
			t.Errorf("aw build should not pin an install toggle:\n%s", out)
		}
	})
}

func TestSetMiseInstallToggle_KeyChoice(t *testing.T) {
	// The two keys are mutually exclusive in profile.Validate, so a write must
	// never leave both behind.
	t.Run("updates the deprecated key in place", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "config.yml")
		if err := os.WriteFile(cfgPath, []byte("profiles:\n  dev:\n    skip_mise_install: false\n"), 0644); err != nil {
			t.Fatal(err)
		}

		skip := true
		if err := applyBuildResult(cfgPath, "dev", "img:1", &skip); err != nil {
			t.Fatalf("applyBuildResult() error = %v", err)
		}

		data, _ := os.ReadFile(cfgPath)
		content := string(data)
		if !strings.Contains(content, "skip_mise_install: true") {
			t.Errorf("the existing key should be updated, got:\n%s", content)
		}
		if strings.Contains(content, "mise_install: false") {
			t.Errorf("a second, mutually exclusive key must not appear, got:\n%s", content)
		}
	})

	t.Run("honours a deprecated key set as a top-level default", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "config.yml")
		if err := os.WriteFile(cfgPath, []byte("skip_mise_install: true\nprofiles:\n  dev:\n    launch: shell\n"), 0644); err != nil {
			t.Fatal(err)
		}

		skip := true
		if err := applyBuildResult(cfgPath, "dev", "img:1", &skip); err != nil {
			t.Fatalf("applyBuildResult() error = %v", err)
		}

		data, _ := os.ReadFile(cfgPath)
		content := string(data)
		// Top-level keys are inline profile defaults, so a profile-level
		// mise_install would collide with them after the merge.
		if strings.Contains(content, "mise_install: false") {
			t.Errorf("should not add a key that collides with the top-level default, got:\n%s", content)
		}
	})
}

// fakeDockerClient records what aw build asks the runtime to do.
type fakeDockerClient struct {
	docker.Client

	imageExists   bool
	imageExistsFn func(string) (bool, error)
	pullErr       error

	existsCalls []string
	pullCalls   []string
	saveCalls   []string
	tagCalls    []string
	pushCalls   []string
}

func (f *fakeDockerClient) ImageExists(_ context.Context, imageName string) (bool, error) {
	f.existsCalls = append(f.existsCalls, imageName)
	if f.imageExistsFn != nil {
		return f.imageExistsFn(imageName)
	}
	return f.imageExists, nil
}

func (f *fakeDockerClient) Pull(_ context.Context, imageName string) error {
	f.pullCalls = append(f.pullCalls, imageName)
	return f.pullErr
}

func (f *fakeDockerClient) Save(_ context.Context, imageName, outputPath string) error {
	f.saveCalls = append(f.saveCalls, imageName+"->"+outputPath)
	return nil
}

func (f *fakeDockerClient) Tag(_ context.Context, source, target string) error {
	f.tagCalls = append(f.tagCalls, source+"->"+target)
	return nil
}

func (f *fakeDockerClient) Push(_ context.Context, imageName string) error {
	f.pushCalls = append(f.pushCalls, imageName)
	return nil
}

// A pinned image with nothing to bake in still has to exist locally before
// --save tars it or --push retags it.
func TestImageWithoutBuildInputs_PinnedImageIsResolved(t *testing.T) {
	const pinned = "ghcr.io/myorg/base:v1"
	tarPath := "out.tar"

	t.Run("local image is used without pulling", func(t *testing.T) {
		client := &fakeDockerClient{imageExists: true}
		b := &BuildCmd{ProfileName: "dev", Save: &tarPath}

		img, reused, err := b.imageWithoutBuildInputs(context.Background(), profile.Profile{Image: pinned}, client)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if img != pinned || !reused {
			t.Fatalf("got (%q, %v), want (%q, true)", img, reused, pinned)
		}
		if len(client.existsCalls) != 1 {
			t.Errorf("the image should be checked before --save, got %v", client.existsCalls)
		}
		if len(client.pullCalls) != 0 {
			t.Errorf("a local image should not be pulled, got %v", client.pullCalls)
		}
	})

	t.Run("remote-only image is pulled", func(t *testing.T) {
		client := &fakeDockerClient{imageExists: false}
		b := &BuildCmd{ProfileName: "dev", Save: &tarPath}

		img, _, err := b.imageWithoutBuildInputs(context.Background(), profile.Profile{Image: pinned}, client)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if img != pinned {
			t.Fatalf("image = %q, want %q", img, pinned)
		}
		if len(client.pullCalls) != 1 || client.pullCalls[0] != pinned {
			t.Errorf("the pinned image should be pulled, got %v", client.pullCalls)
		}
	})

	t.Run("pull failure is fatal and does not fall back", func(t *testing.T) {
		client := &fakeDockerClient{imageExists: false, pullErr: errors.New("manifest unknown")}
		b := &BuildCmd{ProfileName: "dev", Save: &tarPath}

		img, _, err := b.imageWithoutBuildInputs(context.Background(), profile.Profile{Image: pinned}, client)
		if err == nil {
			t.Fatal("a missing pinned image must fail rather than silently save a different one")
		}
		if img != "" {
			t.Errorf("image = %q, want empty on failure", img)
		}
		if !strings.Contains(err.Error(), pinned) {
			t.Errorf("error should name the image, got: %v", err)
		}
		// Falling back here would tar the official image under the user's name.
		if strings.Contains(err.Error(), stage.OfficialImageRegistry) {
			t.Errorf("must not switch to the official image, got: %v", err)
		}
	})

	t.Run("image_pull_policy never fails instead of pulling", func(t *testing.T) {
		client := &fakeDockerClient{imageExists: false}
		b := &BuildCmd{ProfileName: "dev", Save: &tarPath}
		p := profile.Profile{Image: pinned, ImagePullPolicy: profile.ImagePullPolicyNever}

		if _, _, err := b.imageWithoutBuildInputs(context.Background(), p, client); err == nil {
			t.Fatal("image_pull_policy: never must not pull")
		}
		if len(client.pullCalls) != 0 {
			t.Errorf("no pull should happen, got %v", client.pullCalls)
		}
	})

	t.Run("image_pull_policy always pulls without checking locally", func(t *testing.T) {
		client := &fakeDockerClient{imageExists: true}
		b := &BuildCmd{ProfileName: "dev", Push: true}
		p := profile.Profile{Image: pinned, ImagePullPolicy: profile.ImagePullPolicyAlways}

		if _, _, err := b.imageWithoutBuildInputs(context.Background(), p, client); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(client.existsCalls) != 0 {
			t.Errorf("always should skip the local check, got %v", client.existsCalls)
		}
		if len(client.pullCalls) != 1 {
			t.Errorf("always should pull, got %v", client.pullCalls)
		}
	})

	t.Run("no save or push leaves the registry alone", func(t *testing.T) {
		client := &fakeDockerClient{imageExists: false}
		b := &BuildCmd{ProfileName: "dev", Apply: true}

		img, reused, err := b.imageWithoutBuildInputs(context.Background(), profile.Profile{Image: pinned}, client)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if img != pinned || !reused {
			t.Fatalf("got (%q, %v), want (%q, true)", img, reused, pinned)
		}
		// Nothing consumes the image here: the config keeps the name it
		// already had, so a pull would be pure cost.
		if len(client.pullCalls) != 0 || len(client.existsCalls) != 0 {
			t.Errorf("no runtime calls expected, got exists=%v pull=%v", client.existsCalls, client.pullCalls)
		}
	})
}
