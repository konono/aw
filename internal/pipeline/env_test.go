package pipeline

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/konono/aw/internal/containerenv"
	"github.com/konono/aw/internal/profile"
)

func TestContainerEnvVars_IncludesAWUserAndHome(t *testing.T) {
	ec := &ExecutionContext{
		Profile:      profile.Profile{},
		ContainerEnv: containerenv.Default(),
	}
	envVars := ContainerEnvVars(ec, "claude")

	if envVars["AW_USER"] != "agent" {
		t.Errorf("AW_USER = %q, want %q", envVars["AW_USER"], "agent")
	}
	if envVars["AW_HOME"] != "/home/agent" {
		t.Errorf("AW_HOME = %q, want %q", envVars["AW_HOME"], "/home/agent")
	}
}

func TestContainerEnvVars_CustomUser(t *testing.T) {
	ec := &ExecutionContext{
		Profile:      profile.Profile{},
		ContainerEnv: containerenv.FromUser("dev"),
	}
	envVars := ContainerEnvVars(ec, "claude")

	if envVars["AW_USER"] != "dev" {
		t.Errorf("AW_USER = %q, want %q", envVars["AW_USER"], "dev")
	}
	if envVars["AW_HOME"] != "/home/dev" {
		t.Errorf("AW_HOME = %q, want %q", envVars["AW_HOME"], "/home/dev")
	}
}

func TestContainerEnvVars_AWPackages_FromProfile(t *testing.T) {
	ec := &ExecutionContext{
		Profile: profile.Profile{
			Packages: []string{"jq", "tree"},
		},
		HomeDir:      t.TempDir(),
		ContainerEnv: containerenv.Default(),
	}
	envVars := ContainerEnvVars(ec, "claude")

	if envVars["AW_PACKAGES"] != "jq,tree" {
		t.Errorf("AW_PACKAGES = %q, want %q", envVars["AW_PACKAGES"], "jq,tree")
	}
}

func TestContainerEnvVars_AWPackages_FromFile(t *testing.T) {
	homeDir := t.TempDir()
	configDir := filepath.Join(homeDir, ".config", "aw")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "packages.txt"), []byte("curl\nwget\n"), 0644); err != nil {
		t.Fatal(err)
	}

	ec := &ExecutionContext{
		Profile:      profile.Profile{},
		HomeDir:      homeDir,
		ContainerEnv: containerenv.Default(),
	}
	envVars := ContainerEnvVars(ec, "claude")

	if envVars["AW_PACKAGES"] != "curl,wget" {
		t.Errorf("AW_PACKAGES = %q, want %q", envVars["AW_PACKAGES"], "curl,wget")
	}
}

func TestContainerEnvVars_AWPackages_NotSetWhenEmpty(t *testing.T) {
	ec := &ExecutionContext{
		Profile:      profile.Profile{},
		HomeDir:      t.TempDir(),
		ContainerEnv: containerenv.Default(),
	}
	envVars := ContainerEnvVars(ec, "claude")

	if _, ok := envVars["AW_PACKAGES"]; ok {
		t.Error("AW_PACKAGES should not be set when no packages")
	}
}

func TestCollectEnvPackages_MergedAndDeduped(t *testing.T) {
	homeDir := t.TempDir()
	configDir := filepath.Join(homeDir, ".config", "aw")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "packages.txt"), []byte("jq\ncurl\n"), 0644); err != nil {
		t.Fatal(err)
	}

	result := collectEnvPackages(homeDir, []string{"curl", "tree"})
	if result != "jq,curl,tree" {
		t.Errorf("collectEnvPackages() = %q, want %q", result, "jq,curl,tree")
	}
}
