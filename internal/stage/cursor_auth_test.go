package stage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSeedCursorAuth_SkipsExistingStagingFile(t *testing.T) {
	stageDir := t.TempDir()
	authPath := filepath.Join(stageDir, "auth.json")
	existing := `{"accessToken":"stage","refreshToken":"stage-refresh"}`
	if err := os.WriteFile(authPath, []byte(existing), 0600); err != nil {
		t.Fatalf("writing auth.json: %v", err)
	}

	if err := seedCursorAuth(stageDir, t.TempDir()); err != nil {
		t.Fatalf("seedCursorAuth() error: %v", err)
	}

	data, err := os.ReadFile(authPath)
	if err != nil {
		t.Fatalf("reading auth.json: %v", err)
	}
	if string(data) != existing {
		t.Fatalf("auth.json = %q, want existing staging copy preserved", string(data))
	}
}

func TestSeedCursorAuth_FromHostConfigFile(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("seedCursorAuth prefers macOS Keychain over file source")
	}

	homeDir := t.TempDir()
	hostAuthDir := filepath.Join(homeDir, ".config", "cursor")
	if err := os.MkdirAll(hostAuthDir, 0755); err != nil {
		t.Fatalf("creating host auth dir: %v", err)
	}
	hostAuth := cursorAuth{
		AccessToken:  "host-access",
		RefreshToken: "host-refresh",
	}
	hostData, err := json.MarshalIndent(hostAuth, "", "  ")
	if err != nil {
		t.Fatalf("marshal host auth: %v", err)
	}
	hostData = append(hostData, '\n')
	if err := os.WriteFile(filepath.Join(hostAuthDir, "auth.json"), hostData, 0600); err != nil {
		t.Fatalf("writing host auth.json: %v", err)
	}

	stageDir := filepath.Join(t.TempDir(), "cursor")
	if err := seedCursorAuth(stageDir, homeDir); err != nil {
		t.Fatalf("seedCursorAuth() error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(stageDir, "auth.json"))
	if err != nil {
		t.Fatalf("reading staged auth.json: %v", err)
	}
	if string(data) != string(hostData) {
		t.Fatalf("staged auth.json = %q, want %q", string(data), string(hostData))
	}
}

func TestSeedCursorAuth_NoSourceIsNoError(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("macOS Keychain may provide cursor tokens on developer machines")
	}

	stageDir := filepath.Join(t.TempDir(), "cursor")
	if err := seedCursorAuth(stageDir, t.TempDir()); err != nil {
		t.Fatalf("seedCursorAuth() error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(stageDir, "auth.json")); err == nil {
		t.Fatal("auth.json should not be created when no token source exists")
	}
}

func TestCursorAuthFromFile_ReturnsValidJSON(t *testing.T) {
	homeDir := t.TempDir()
	hostAuthDir := filepath.Join(homeDir, ".config", "cursor")
	if err := os.MkdirAll(hostAuthDir, 0755); err != nil {
		t.Fatalf("creating host auth dir: %v", err)
	}
	hostAuth := cursorAuth{
		AccessToken:  "host-access",
		RefreshToken: "host-refresh",
	}
	hostData, err := json.MarshalIndent(hostAuth, "", "  ")
	if err != nil {
		t.Fatalf("marshal host auth: %v", err)
	}
	hostData = append(hostData, '\n')
	if err := os.WriteFile(filepath.Join(hostAuthDir, "auth.json"), hostData, 0600); err != nil {
		t.Fatalf("writing host auth.json: %v", err)
	}

	got := cursorAuthFromFile(homeDir)
	if string(got) != string(hostData) {
		t.Fatalf("cursorAuthFromFile() = %q, want %q", string(got), string(hostData))
	}
}

func TestCursorAuthFromFile_RejectsMissingAccessToken(t *testing.T) {
	homeDir := t.TempDir()
	hostAuthDir := filepath.Join(homeDir, ".config", "cursor")
	if err := os.MkdirAll(hostAuthDir, 0755); err != nil {
		t.Fatalf("creating host auth dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(hostAuthDir, "auth.json"), []byte(`{"refreshToken":"only-refresh"}`), 0600); err != nil {
		t.Fatalf("writing host auth.json: %v", err)
	}

	if got := cursorAuthFromFile(homeDir); got != nil {
		t.Fatalf("cursorAuthFromFile() = %q, want nil for missing accessToken", string(got))
	}
}
