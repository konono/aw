package stage

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// DetectGhToken retrieves the GitHub token from the gh CLI.
func DetectGhToken() (string, error) {
	out, err := exec.Command("gh", "auth", "token").Output()
	if err != nil {
		return "", fmt.Errorf("failed to get token from 'gh auth token': %w (is gh CLI installed and authenticated?)", err)
	}
	token := strings.TrimSpace(string(out))
	if token == "" {
		return "", fmt.Errorf("'gh auth token' returned empty token")
	}
	return token, nil
}

type cursorAuth struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
}

func seedCursorAuth(stageDir, homeDir string) error {
	authPath := filepath.Join(stageDir, "auth.json")
	if _, err := os.Stat(authPath); err == nil {
		return nil
	}

	if data := CursorAuthFromKeychain(); data != nil {
		if err := os.MkdirAll(stageDir, 0755); err != nil {
			return err
		}
		return os.WriteFile(authPath, data, 0600)
	}

	if data := cursorAuthFromFile(homeDir); data != nil {
		if err := os.MkdirAll(stageDir, 0755); err != nil {
			return err
		}
		return os.WriteFile(authPath, data, 0600)
	}

	return nil
}

// CursorAuthFromKeychain reads Cursor auth tokens from macOS Keychain.
// Returns nil if not on macOS or if tokens are not found.
func CursorAuthFromKeychain() []byte {
	if runtime.GOOS != "darwin" {
		return nil
	}

	access, err := readKeychainPassword("cursor-access-token", "cursor-user")
	if err != nil || access == "" {
		return nil
	}
	refresh, err := readKeychainPassword("cursor-refresh-token", "cursor-user")
	if err != nil || refresh == "" {
		return nil
	}

	data, err := json.MarshalIndent(cursorAuth{
		AccessToken:  access,
		RefreshToken: refresh,
	}, "", "  ")
	if err != nil {
		return nil
	}
	return append(data, '\n')
}

func cursorAuthFromFile(homeDir string) []byte {
	path := filepath.Join(homeDir, ".config", "cursor", "auth.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var auth cursorAuth
	if err := json.Unmarshal(data, &auth); err != nil {
		return nil
	}
	if auth.AccessToken == "" {
		return nil
	}
	return data
}

func readKeychainPassword(service, account string) (string, error) {
	out, err := exec.Command("security", "find-generic-password",
		"-s", service, "-a", account, "-w").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
