package cmd

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/konono/aw/v4/internal/docker"
	"github.com/konono/aw/v4/internal/profile"
)

func TestExtractProfileName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"simple", "aw-claude-1735000000000", "claude", false},
		{"hyphenated", "aw-my-profile-1735000000000", "my-profile", false},
		{"multi-hyphen", "aw-a-b-c-123456", "a-b-c", false},
		{"short digits", "aw-shell-1", "shell", false},
		{"no aw prefix", "container-123", "", true},
		{"no digits suffix", "aw-claude-abc", "", true},
		{"empty", "", "", true},
		{"snapshot container", "aw-snapshot-abcdef01", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractProfileName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("extractProfileName(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("extractProfileName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestComputeSaveImageName(t *testing.T) {
	name := computeSaveImageName("claude")
	if !strings.HasPrefix(name, "aw-save:claude-") {
		t.Errorf("got %q, want prefix 'aw-save:claude-'", name)
	}
	parts := strings.SplitN(name, ":", 2)
	if len(parts) != 2 {
		t.Fatalf("expected repo:tag format, got %q", name)
	}
	if parts[0] != "aw-save" {
		t.Errorf("repo = %q, want 'aw-save'", parts[0])
	}
}

func TestComputeSaveImageName_UnsafeChars(t *testing.T) {
	name := computeSaveImageName("my/profile:v1")
	if strings.ContainsAny(name[len("aw-save:"):], "/: ") {
		t.Errorf("image name contains unsafe chars: %q", name)
	}
}

func TestFormatStatus(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Up 2 hours", "running"},
		{"up 5 minutes", "running"},
		{"Exited (0) 3 minutes ago", "exited"},
		{"exited (137) 1 hour ago", "exited"},
		{"Created", "Created"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := formatStatus(tt.input)
			if got != tt.want {
				t.Errorf("formatStatus(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFilterUserEntries_SnapshotExcluded(t *testing.T) {
	entries := []containerEntry{
		{ContainerInfo: docker.ContainerInfo{Name: "aw-claude-123456"}, Runtime: "docker"},
		{ContainerInfo: docker.ContainerInfo{Name: "aw-snapshot-abcdef01"}, Runtime: "docker"},
		{ContainerInfo: docker.ContainerInfo{Name: "aw-shell-789"}, Runtime: "podman"},
		{ContainerInfo: docker.ContainerInfo{Name: "aw-snapshot-12345678"}, Runtime: "docker"},
	}
	result := filterUserEntries(entries)
	if len(result) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result))
	}
	if result[0].Name != "aw-claude-123456" {
		t.Errorf("result[0].Name = %q, want 'aw-claude-123456'", result[0].Name)
	}
	if result[1].Name != "aw-shell-789" {
		t.Errorf("result[1].Name = %q, want 'aw-shell-789'", result[1].Name)
	}
}

func TestFilterUserEntries_PreservesRuntime(t *testing.T) {
	entries := []containerEntry{
		{ContainerInfo: docker.ContainerInfo{Name: "aw-claude-123"}, Runtime: "podman"},
	}
	result := filterUserEntries(entries)
	if len(result) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result))
	}
	if result[0].Runtime != "podman" {
		t.Errorf("runtime = %q, want 'podman'", result[0].Runtime)
	}
}

func TestDetectRuntimes_Explicit(t *testing.T) {
	rts, err := detectRuntimes("docker")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rts) != 1 || rts[0] != "docker" {
		t.Errorf("got %v, want [docker]", rts)
	}
}

func TestDetectRuntimes_Invalid(t *testing.T) {
	_, err := detectRuntimes("rkt")
	if err == nil {
		t.Fatal("expected error for invalid runtime")
	}
}

func TestCommitBaseChanges(t *testing.T) {
	if len(commitBaseChanges) < 2 {
		t.Fatal("commitBaseChanges should have at least ENTRYPOINT and CMD")
	}
	hasEntrypoint := false
	hasCmd := false
	for _, c := range commitBaseChanges {
		if strings.Contains(c, "ENTRYPOINT") {
			hasEntrypoint = true
		}
		if strings.Contains(c, "CMD") {
			hasCmd = true
		}
	}
	if !hasEntrypoint {
		t.Error("commitBaseChanges missing ENTRYPOINT")
	}
	if !hasCmd {
		t.Error("commitBaseChanges missing CMD")
	}
}

func TestResolveConfigPath(t *testing.T) {
	dir := t.TempDir()
	got, err := resolveConfigPath(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == "" {
		t.Fatal("resolveConfigPath returned empty string")
	}
}

func TestResolveConfigPath_ExistingConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".aw.yml")
	if err := os.WriteFile(configPath, []byte("profiles: {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	got, err := resolveConfigPath(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasSuffix(got, ".aw.yml") {
		t.Errorf("got %q, want path ending with .aw.yml", got)
	}
}

func TestResolveConfigPath_SubdirUsesGitRoot(t *testing.T) {
	root := t.TempDir()
	// Resolve symlinks (macOS /var → /private/var)
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "init", root)
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		t.Skipf("git init failed: %v", err)
	}
	configPath := filepath.Join(root, ".aw.yml")
	if err := os.WriteFile(configPath, []byte("profiles: {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	subdir := filepath.Join(root, "subdir")
	if err := os.Mkdir(subdir, 0755); err != nil {
		t.Fatal(err)
	}

	got, err := resolveConfigPath(subdir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Resolve symlinks on the result too for macOS comparison
	gotResolved, _ := filepath.EvalSymlinks(got)
	if gotResolved != configPath {
		t.Errorf("got %q, want git root config %q", gotResolved, configPath)
	}
}

func TestDetectPackageManager_Default(t *testing.T) {
	dir := t.TempDir()
	got := detectPackageManager(dir, "nonexistent")
	if got != profile.PackageManagerApt {
		t.Errorf("got %q, want %q (default)", got, profile.PackageManagerApt)
	}
}

func TestListAllAwContainers_EmptyRuntimes(t *testing.T) {
	entries, err := listAllAwContainers(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}
