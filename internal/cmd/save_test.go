package cmd

import (
	"context"
	"errors"
	"fmt"
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

func TestResolveSaveProfile(t *testing.T) {
	cfg := &profile.Config{Profiles: map[string]profile.Profile{
		"claude-dev": {Environment: profile.EnvironmentContainer, Launch: profile.LaunchClaude},
	}}

	tests := []struct {
		name          string
		containerName string
		profileName   string
		wantErr       bool
	}{
		{
			name:          "regular container resolves to its profile",
			containerName: "aw-claude-dev-1234",
			profileName:   "claude-dev",
		},
		{
			// Leftover from the removed "aw team" command: the name matches
			// the aw-<...>-<digits> listing filter and yields a profile name
			// that does not exist.
			name:          "leftover team container is rejected",
			containerName: "aw-review-team-developer-1-1700000000000000000",
			profileName:   "review-team-developer-1",
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := resolveSaveProfile(cfg, tt.containerName, tt.profileName, "/workspace", "podman")
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for container %q", tt.containerName)
				}
				if !strings.Contains(err.Error(), tt.profileName) {
					t.Errorf("error should name the unresolved profile %q, got: %v", tt.profileName, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// Regression: a listing that mixes a regular container with a leftover team
// container must only be able to save the regular one.
func TestResolveSaveProfile_MixedListing(t *testing.T) {
	cfg := &profile.Config{Profiles: map[string]profile.Profile{
		"claude-dev": {Environment: profile.EnvironmentContainer, Launch: profile.LaunchClaude},
	}}

	containers := []string{
		"aw-claude-dev-1234",
		"aw-review-team-developer-1-1700000000000000000",
		"aw-review-team-reviewer-1-1700000000000000001",
	}

	var saveable []string
	for _, name := range containers {
		profileName, err := extractProfileName(name)
		if err != nil {
			continue
		}
		if err := resolveSaveProfile(cfg, name, profileName, "/workspace", "podman"); err != nil {
			continue
		}
		saveable = append(saveable, name)
	}

	if len(saveable) != 1 || saveable[0] != "aw-claude-dev-1234" {
		t.Errorf("saveable = %v, want only [aw-claude-dev-1234]", saveable)
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

// fakeSaver records what the save flow asks of the container runtime.
type fakeSaver struct {
	env       map[string]string
	failEnv   map[string]error // inspect command failures, keyed by env var
	commits   []string
	inspected []string
}

func (f *fakeSaver) InspectContainerEnv(_ context.Context, containerName, envKey string) (string, error) {
	f.inspected = append(f.inspected, envKey)
	if err, ok := f.failEnv[envKey]; ok {
		return "", err
	}
	v, ok := f.env[envKey]
	if !ok {
		return "", fmt.Errorf("%w: %q in container %q", docker.ErrEnvNotFound, envKey, containerName)
	}
	return v, nil
}

func (f *fakeSaver) Commit(_ context.Context, containerID, imageName string, _ []string) error {
	f.commits = append(f.commits, containerID+"=>"+imageName)
	return nil
}

// newSaveWorkspace creates a git repo with an .aw.yml defining profileName.
func newSaveWorkspace(t *testing.T, profileName string) string {
	t.Helper()
	root := t.TempDir()
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "init", root).Run(); err != nil {
		t.Skipf("git init failed: %v", err)
	}
	body := "profiles:\n  " + profileName + ":\n    launch: claude\n    environment: container\n"
	if err := os.WriteFile(filepath.Join(root, ".aw.yml"), []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	return root
}

// Regression: a leftover team container whose extracted name happens to match
// a real profile must still be rejected, and must not be committed.
func TestSaveSelectedContainer_LegacyTeamContainerWithCollidingProfile(t *testing.T) {
	const containerName = "aw-review-team-developer-1-1700000000000000000"
	const profileName = "review-team-developer-1"

	workspace := newSaveWorkspace(t, profileName)
	saver := &fakeSaver{env: map[string]string{
		"HOST_WORKSPACE": workspace,
		"AW_TEAM_NAME":   "review-team",
		"AW_AGENT_NAME":  "developer-1",
	}}
	entry := &containerEntry{
		ContainerInfo: docker.ContainerInfo{Name: containerName},
		Runtime:       "podman",
	}

	err := saveSelectedContainer(context.Background(), saver, entry, profileName, "")
	if err == nil {
		t.Fatal("expected the leftover team container to be rejected")
	}
	if !strings.Contains(err.Error(), "aw team") {
		t.Errorf("error should point at the removed team command, got: %v", err)
	}
	if len(saver.commits) != 0 {
		t.Errorf("Commit must not be called for a rejected container, got %v", saver.commits)
	}
}

// A regular container is committed and its config updated.
func TestSaveSelectedContainer_RegularContainer(t *testing.T) {
	const containerName = "aw-claude-dev-1234"
	const profileName = "claude-dev"

	workspace := newSaveWorkspace(t, profileName)
	saver := &fakeSaver{env: map[string]string{"HOST_WORKSPACE": workspace}}
	entry := &containerEntry{
		ContainerInfo: docker.ContainerInfo{Name: containerName},
		Runtime:       "podman",
	}

	if err := saveSelectedContainer(context.Background(), saver, entry, profileName, "aw-save:test"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(saver.commits) != 1 || saver.commits[0] != containerName+"=>aw-save:test" {
		t.Fatalf("commits = %v, want one commit of %q", saver.commits, containerName)
	}

	data, err := os.ReadFile(filepath.Join(workspace, ".aw.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "aw-save:test") {
		t.Errorf("config should record the saved image, got:\n%s", data)
	}
}

// A failing inspect must not be read as "this is not a team container": the
// container could be a leftover whose name collides with a real profile, in
// which case saving it would overwrite that profile's image.
func TestSaveSelectedContainer_InspectFailureAborts(t *testing.T) {
	const containerName = "aw-review-team-developer-1-1700000000000000000"
	const profileName = "review-team-developer-1"

	for _, envKey := range []string{"AW_TEAM_NAME", "AW_AGENT_NAME"} {
		t.Run(envKey, func(t *testing.T) {
			workspace := newSaveWorkspace(t, profileName)
			saver := &fakeSaver{
				env: map[string]string{
					"HOST_WORKSPACE": workspace,
					"AW_TEAM_NAME":   "review-team",
					"AW_AGENT_NAME":  "developer-1",
				},
				failEnv: map[string]error{
					envKey: errors.New("podman inspect: connection refused"),
				},
			}
			entry := &containerEntry{
				ContainerInfo: docker.ContainerInfo{Name: containerName},
				Runtime:       "podman",
			}

			err := saveSelectedContainer(context.Background(), saver, entry, profileName, "")
			if err == nil {
				t.Fatal("expected the failed inspect to abort the save")
			}
			if !strings.Contains(err.Error(), envKey) {
				t.Errorf("error should name the env var it could not read, got: %v", err)
			}
			if len(saver.commits) != 0 {
				t.Errorf("Commit must not be called after a failed inspect, got %v", saver.commits)
			}
		})
	}
}
