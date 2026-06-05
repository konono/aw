package cmd

import (
	"errors"
	"testing"
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
}
