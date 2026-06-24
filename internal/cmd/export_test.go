package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/konono/aw/internal/profile"
)

func TestParseExportIncludes(t *testing.T) {
	t.Run("valid single", func(t *testing.T) {
		inc, err := parseExportIncludes([]string{"./certs:/usr/local/share/ca-certificates"})
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
		inc, err := parseExportIncludes([]string{"./a:/a", "./b:/b"})
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if len(inc) != 2 {
			t.Fatalf("len = %d, want 2", len(inc))
		}
	})

	t.Run("bad format no colon", func(t *testing.T) {
		_, err := parseExportIncludes([]string{"nodelimiter"})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("bad format empty src", func(t *testing.T) {
		_, err := parseExportIncludes([]string{":/dst"})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("bad format empty dst", func(t *testing.T) {
		_, err := parseExportIncludes([]string{"src:"})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("empty list", func(t *testing.T) {
		inc, err := parseExportIncludes(nil)
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if len(inc) != 0 {
			t.Fatalf("len = %d, want 0", len(inc))
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
