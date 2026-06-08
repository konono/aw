package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/konono/aw/internal/profile"
)

func TestParseExportArgs(t *testing.T) {
	t.Run("profile only", func(t *testing.T) {
		opts, err := parseExportArgs([]string{"claude"})
		if err != nil {
			t.Fatalf("parseExportArgs() error = %v", err)
		}
		if opts.ProfileName != "claude" {
			t.Fatalf("ProfileName = %q, want %q", opts.ProfileName, "claude")
		}
		if opts.OutputPath != "" {
			t.Fatalf("OutputPath = %q, want empty", opts.OutputPath)
		}
	})

	t.Run("profile and output", func(t *testing.T) {
		opts, err := parseExportArgs([]string{"claude", "-o", "image.tar"})
		if err != nil {
			t.Fatalf("parseExportArgs() error = %v", err)
		}
		if opts.ProfileName != "claude" {
			t.Fatalf("ProfileName = %q, want %q", opts.ProfileName, "claude")
		}
		if opts.OutputPath != "image.tar" {
			t.Fatalf("OutputPath = %q, want %q", opts.OutputPath, "image.tar")
		}
	})

	t.Run("help", func(t *testing.T) {
		_, err := parseExportArgs([]string{"--help"})
		if !errors.Is(err, errExportHelp) {
			t.Fatalf("parseExportArgs(--help) error = %v, want errExportHelp", err)
		}
	})

	t.Run("missing profile", func(t *testing.T) {
		_, err := parseExportArgs([]string{})
		if err == nil || err.Error() != "profile name is required" {
			t.Fatalf("parseExportArgs() error = %v, want profile name is required", err)
		}
	})

	t.Run("missing output path", func(t *testing.T) {
		_, err := parseExportArgs([]string{"claude", "-o"})
		if err == nil || err.Error() != "-o requires an output path" {
			t.Fatalf("parseExportArgs() error = %v, want missing output path", err)
		}
	})

	t.Run("unknown flag", func(t *testing.T) {
		_, err := parseExportArgs([]string{"claude", "--output"})
		if err == nil || err.Error() != "unknown flag \"--output\"" {
			t.Fatalf("parseExportArgs() error = %v, want unknown flag", err)
		}
	})

	t.Run("too many targets", func(t *testing.T) {
		_, err := parseExportArgs([]string{"claude", "codex"})
		if err == nil || err.Error() != "too many export targets" {
			t.Fatalf("parseExportArgs() error = %v, want too many export targets", err)
		}
	})

	t.Run("snapshot flag", func(t *testing.T) {
		opts, err := parseExportArgs([]string{"claude", "--snapshot"})
		if err != nil {
			t.Fatalf("parseExportArgs() error = %v", err)
		}
		if !opts.Snapshot {
			t.Fatal("Snapshot should be true")
		}
	})

	t.Run("include single", func(t *testing.T) {
		opts, err := parseExportArgs([]string{"claude", "--include", "./certs:/usr/local/share/ca-certificates"})
		if err != nil {
			t.Fatalf("parseExportArgs() error = %v", err)
		}
		if len(opts.Include) != 1 {
			t.Fatalf("Include len = %d, want 1", len(opts.Include))
		}
		if opts.Include[0].Src != "./certs" || opts.Include[0].Dst != "/usr/local/share/ca-certificates" {
			t.Fatalf("Include[0] = %+v, want {./certs /usr/local/share/ca-certificates}", opts.Include[0])
		}
	})

	t.Run("include multiple", func(t *testing.T) {
		opts, err := parseExportArgs([]string{"claude", "--include", "./a:/a", "--include", "./b:/b"})
		if err != nil {
			t.Fatalf("parseExportArgs() error = %v", err)
		}
		if len(opts.Include) != 2 {
			t.Fatalf("Include len = %d, want 2", len(opts.Include))
		}
	})

	t.Run("include missing value", func(t *testing.T) {
		_, err := parseExportArgs([]string{"claude", "--include"})
		if err == nil || err.Error() != "--include requires src:dst argument" {
			t.Fatalf("parseExportArgs() error = %v, want missing arg error", err)
		}
	})

	t.Run("include bad format", func(t *testing.T) {
		_, err := parseExportArgs([]string{"claude", "--include", "nodelimiter"})
		if err == nil || err.Error() != "--include requires format src:dst" {
			t.Fatalf("parseExportArgs() error = %v, want format error", err)
		}
	})

	t.Run("env single", func(t *testing.T) {
		opts, err := parseExportArgs([]string{"claude", "--env", "FOO=bar"})
		if err != nil {
			t.Fatalf("parseExportArgs() error = %v", err)
		}
		if opts.Env["FOO"] != "bar" {
			t.Fatalf("Env[FOO] = %q, want %q", opts.Env["FOO"], "bar")
		}
	})

	t.Run("env with equals in value", func(t *testing.T) {
		opts, err := parseExportArgs([]string{"claude", "--env", "FOO=bar=baz"})
		if err != nil {
			t.Fatalf("parseExportArgs() error = %v", err)
		}
		if opts.Env["FOO"] != "bar=baz" {
			t.Fatalf("Env[FOO] = %q, want %q", opts.Env["FOO"], "bar=baz")
		}
	})

	t.Run("env missing value", func(t *testing.T) {
		_, err := parseExportArgs([]string{"claude", "--env"})
		if err == nil || err.Error() != "--env requires KEY=VAL argument" {
			t.Fatalf("parseExportArgs() error = %v, want missing arg error", err)
		}
	})

	t.Run("env bad format", func(t *testing.T) {
		_, err := parseExportArgs([]string{"claude", "--env", "NOEQUALS"})
		if err == nil || err.Error() != "--env requires format KEY=VAL" {
			t.Fatalf("parseExportArgs() error = %v, want format error", err)
		}
	})

	t.Run("apply flag", func(t *testing.T) {
		opts, err := parseExportArgs([]string{"claude", "--apply"})
		if err != nil {
			t.Fatalf("parseExportArgs() error = %v", err)
		}
		if !opts.Apply {
			t.Fatal("Apply should be true")
		}
	})

	t.Run("all flags combined", func(t *testing.T) {
		opts, err := parseExportArgs([]string{
			"claude", "--snapshot",
			"--include", "./certs:/certs",
			"--env", "FOO=bar",
			"--apply",
			"-o", "out.tar",
		})
		if err != nil {
			t.Fatalf("parseExportArgs() error = %v", err)
		}
		if opts.ProfileName != "claude" {
			t.Errorf("ProfileName = %q, want %q", opts.ProfileName, "claude")
		}
		if !opts.Snapshot {
			t.Error("Snapshot should be true")
		}
		if !opts.Apply {
			t.Error("Apply should be true")
		}
		if len(opts.Include) != 1 {
			t.Errorf("Include len = %d, want 1", len(opts.Include))
		}
		if opts.Env["FOO"] != "bar" {
			t.Errorf("Env[FOO] = %q, want %q", opts.Env["FOO"], "bar")
		}
		if opts.OutputPath != "out.tar" {
			t.Errorf("OutputPath = %q, want %q", opts.OutputPath, "out.tar")
		}
	})
}

func TestMergeExportOptions(t *testing.T) {
	t.Run("cli only snapshot", func(t *testing.T) {
		opts := exportOptions{Snapshot: true}
		snap, inc, env := mergeExportOptions(opts, nil)
		if !snap {
			t.Error("snapshot should be true")
		}
		if len(inc) != 0 {
			t.Errorf("includes should be empty, got %v", inc)
		}
		if len(env) != 0 {
			t.Errorf("env should be empty, got %v", env)
		}
	})

	t.Run("config only", func(t *testing.T) {
		cfg := &profile.ExportConfig{
			Snapshot: true,
			Include:  []profile.ExportInclude{{Src: "./a", Dst: "/a"}},
			Env:      map[string]string{"X": "1"},
		}
		snap, inc, env := mergeExportOptions(exportOptions{}, cfg)
		if !snap {
			t.Error("snapshot should be true from config")
		}
		if len(inc) != 1 || inc[0].Src != "./a" {
			t.Errorf("includes = %v, want [{./a /a}]", inc)
		}
		if env["X"] != "1" {
			t.Errorf("env[X] = %q, want %q", env["X"], "1")
		}
	})

	t.Run("cli overrides config env", func(t *testing.T) {
		cfg := &profile.ExportConfig{
			Env: map[string]string{"A": "from-config", "B": "keep"},
		}
		opts := exportOptions{
			Env: map[string]string{"A": "from-cli"},
		}
		_, _, env := mergeExportOptions(opts, cfg)
		if env["A"] != "from-cli" {
			t.Errorf("env[A] = %q, want %q (cli should override)", env["A"], "from-cli")
		}
		if env["B"] != "keep" {
			t.Errorf("env[B] = %q, want %q (should be preserved)", env["B"], "keep")
		}
	})

	t.Run("include implies snapshot", func(t *testing.T) {
		opts := exportOptions{
			Include: []profile.ExportInclude{{Src: "./x", Dst: "/x"}},
		}
		snap, _, _ := mergeExportOptions(opts, nil)
		if !snap {
			t.Error("snapshot should be implicitly true when includes are present")
		}
	})

	t.Run("env implies snapshot", func(t *testing.T) {
		opts := exportOptions{
			Env: map[string]string{"K": "V"},
		}
		snap, _, _ := mergeExportOptions(opts, nil)
		if !snap {
			t.Error("snapshot should be implicitly true when env vars are present")
		}
	})

	t.Run("cli and config includes combine", func(t *testing.T) {
		cfg := &profile.ExportConfig{
			Include: []profile.ExportInclude{{Src: "./from-config", Dst: "/config"}},
		}
		opts := exportOptions{
			Include: []profile.ExportInclude{{Src: "./from-cli", Dst: "/cli"}},
		}
		_, inc, _ := mergeExportOptions(opts, cfg)
		if len(inc) != 2 {
			t.Errorf("includes len = %d, want 2 (config + cli)", len(inc))
		}
	})
}

func TestApplyExportResult(t *testing.T) {
	t.Run("adds image to profile", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "config.yml")
		if err := os.WriteFile(cfgPath, []byte(`# comment
profiles:
  dev:
    launch: shell
`), 0644); err != nil {
			t.Fatal(err)
		}

		if err := applyExportResult(cfgPath, "dev", "aw-container:abc123", false); err != nil {
			t.Fatalf("applyExportResult() error = %v", err)
		}

		data, _ := os.ReadFile(cfgPath)
		content := string(data)
		if !strings.Contains(content, "image: aw-container:abc123") {
			t.Errorf("config should contain image, got:\n%s", content)
		}
		if !strings.Contains(content, "# comment") {
			t.Error("comment should be preserved")
		}
	})

	t.Run("adds skip flags when snapshot", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "config.yml")
		if err := os.WriteFile(cfgPath, []byte(`profiles:
  dev:
    launch: shell
`), 0644); err != nil {
			t.Fatal(err)
		}

		if err := applyExportResult(cfgPath, "dev", "aw-container:abc123", true); err != nil {
			t.Fatalf("applyExportResult() error = %v", err)
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

		if err := applyExportResult(cfgPath, "dev", "aw-container:new456", false); err != nil {
			t.Fatalf("applyExportResult() error = %v", err)
		}

		data, _ := os.ReadFile(cfgPath)
		content := string(data)
		if !strings.Contains(content, "image: aw-container:new456") {
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

		if err := applyExportResult(cfgPath, "newprofile", "img:123", true); err != nil {
			t.Fatalf("applyExportResult() error = %v", err)
		}

		data, _ := os.ReadFile(cfgPath)
		content := string(data)
		if !strings.Contains(content, "newprofile") {
			t.Errorf("new profile should be added, got:\n%s", content)
		}
		if !strings.Contains(content, "image: img:123") {
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

		if err := applyExportResult(cfgPath, "dev", "img:456", false); err != nil {
			t.Fatalf("applyExportResult() error = %v", err)
		}

		data, _ := os.ReadFile(cfgPath)
		content := string(data)
		if !strings.Contains(content, "profiles") {
			t.Errorf("profiles section should be created, got:\n%s", content)
		}
		if !strings.Contains(content, "image: img:456") {
			t.Errorf("image should be set, got:\n%s", content)
		}
	})

	t.Run("clears skip flags when not snapshot", func(t *testing.T) {
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

		if err := applyExportResult(cfgPath, "dev", "new:456", false); err != nil {
			t.Fatalf("applyExportResult() error = %v", err)
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
}
