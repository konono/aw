package cmd

import (
	"testing"
)

func TestParseRunArgs_ProfileOnly(t *testing.T) {
	opts, err := parseRunArgs([]string{"codex"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.ProfileName != "codex" {
		t.Errorf("got profile %q, want %q", opts.ProfileName, "codex")
	}
	if opts.Recent || opts.Cwd != "" || opts.Query != "" || opts.NoRecord {
		t.Error("unexpected flags set")
	}
}

func TestParseRunArgs_RecentAfterProfile(t *testing.T) {
	opts, err := parseRunArgs([]string{"codex", "--recent"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.ProfileName != "codex" {
		t.Errorf("got profile %q, want %q", opts.ProfileName, "codex")
	}
	if !opts.Recent {
		t.Error("expected Recent to be true")
	}
}

func TestParseRunArgs_RecentBeforeProfile(t *testing.T) {
	opts, err := parseRunArgs([]string{"--recent", "codex"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.ProfileName != "codex" {
		t.Errorf("got profile %q, want %q", opts.ProfileName, "codex")
	}
	if !opts.Recent {
		t.Error("expected Recent to be true")
	}
}

func TestParseRunArgs_RecentShortFlag(t *testing.T) {
	opts, err := parseRunArgs([]string{"codex", "-r"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.ProfileName != "codex" {
		t.Errorf("got profile %q, want %q", opts.ProfileName, "codex")
	}
	if !opts.Recent {
		t.Error("expected Recent to be true for -r")
	}
}

func TestParseRunArgs_RecentDir(t *testing.T) {
	opts, err := parseRunArgs([]string{"--recent-dir"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !opts.Recent {
		t.Error("expected Recent to be true for --recent-dir")
	}
}

func TestParseRunArgs_CwdAfterProfile(t *testing.T) {
	opts, err := parseRunArgs([]string{"codex", "-C", "/tmp/foo"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.ProfileName != "codex" {
		t.Errorf("got profile %q, want %q", opts.ProfileName, "codex")
	}
	if opts.Cwd != "/tmp/foo" {
		t.Errorf("got cwd %q, want %q", opts.Cwd, "/tmp/foo")
	}
}

func TestParseRunArgs_CwdBeforeProfile(t *testing.T) {
	opts, err := parseRunArgs([]string{"-C", "/tmp/foo", "codex"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.ProfileName != "codex" {
		t.Errorf("got profile %q, want %q", opts.ProfileName, "codex")
	}
	if opts.Cwd != "/tmp/foo" {
		t.Errorf("got cwd %q, want %q", opts.Cwd, "/tmp/foo")
	}
}

func TestParseRunArgs_LongCwd(t *testing.T) {
	opts, err := parseRunArgs([]string{"--cwd", "/tmp/bar"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.Cwd != "/tmp/bar" {
		t.Errorf("got cwd %q, want %q", opts.Cwd, "/tmp/bar")
	}
}

func TestParseRunArgs_CwdEquals(t *testing.T) {
	opts, err := parseRunArgs([]string{"--cwd=/tmp/baz"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.Cwd != "/tmp/baz" {
		t.Errorf("got cwd %q, want %q", opts.Cwd, "/tmp/baz")
	}
}

func TestParseRunArgs_QueryWithRecent(t *testing.T) {
	opts, err := parseRunArgs([]string{"claude", "--recent", "--query", "dotfiles"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.ProfileName != "claude" {
		t.Errorf("got profile %q, want %q", opts.ProfileName, "claude")
	}
	if !opts.Recent {
		t.Error("expected Recent to be true")
	}
	if opts.Query != "dotfiles" {
		t.Errorf("got query %q, want %q", opts.Query, "dotfiles")
	}
}

func TestParseRunArgs_QueryEquals(t *testing.T) {
	opts, err := parseRunArgs([]string{"--recent", "--query=test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.Query != "test" {
		t.Errorf("got query %q, want %q", opts.Query, "test")
	}
}

func TestParseRunArgs_NoRecord(t *testing.T) {
	opts, err := parseRunArgs([]string{"codex", "--no-record"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !opts.NoRecord {
		t.Error("expected NoRecord to be true")
	}
}

func TestParseRunArgs_NoCache(t *testing.T) {
	opts, err := parseRunArgs([]string{"codex", "--no-cache"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !opts.NoCache {
		t.Error("expected NoCache to be true")
	}
	if opts.ProfileName != "codex" {
		t.Errorf("got profile %q, want codex", opts.ProfileName)
	}
}

func TestParseRunArgs_RecentAndCwdConflict(t *testing.T) {
	_, err := parseRunArgs([]string{"codex", "--recent", "-C", "/tmp"})
	if err == nil {
		t.Fatal("expected error for --recent + -C conflict")
	}
}

func TestParseRunArgs_QueryWithoutRecent(t *testing.T) {
	_, err := parseRunArgs([]string{"--query", "test"})
	if err == nil {
		t.Fatal("expected error for --query without --recent")
	}
}

func TestParseRunArgs_UnknownFlag(t *testing.T) {
	_, err := parseRunArgs([]string{"--bogus"})
	if err == nil {
		t.Fatal("expected error for unknown flag")
	}
}

func TestParseRunArgs_CwdMissingValue(t *testing.T) {
	_, err := parseRunArgs([]string{"-C"})
	if err == nil {
		t.Fatal("expected error for -C without value")
	}
}

func TestParseRunArgs_QueryMissingValue(t *testing.T) {
	_, err := parseRunArgs([]string{"--recent", "--query"})
	if err == nil {
		t.Fatal("expected error for --query without value")
	}
}

func TestParseRunArgs_DuplicateProfile(t *testing.T) {
	_, err := parseRunArgs([]string{"codex", "claude"})
	if err == nil {
		t.Fatal("expected error for multiple profile names")
	}
}

func TestParseRunArgs_Empty(t *testing.T) {
	opts, err := parseRunArgs([]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.ProfileName != "" {
		t.Errorf("expected empty profile, got %q", opts.ProfileName)
	}
}
