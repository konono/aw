package pipeline

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/konono/aw/v4/internal/containerenv"
	"github.com/konono/aw/v4/internal/profile"
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

func TestContainerEnvVars_AWPackages_FromWorkspaceFile(t *testing.T) {
	workDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(workDir, "packages.txt"), []byte("curl\nwget\n"), 0644); err != nil {
		t.Fatal(err)
	}

	ec := &ExecutionContext{
		Profile:      profile.Profile{},
		HomeDir:      t.TempDir(),
		OrigWorkDir:  workDir,
		ContainerEnv: containerenv.Default(),
	}
	envVars := ContainerEnvVars(ec, "claude")

	if envVars["AW_PACKAGES"] != "curl,wget" {
		t.Errorf("AW_PACKAGES = %q, want %q", envVars["AW_PACKAGES"], "curl,wget")
	}
}

func TestContainerEnvVars_ContainerSock_SetsBothHosts(t *testing.T) {
	ec := &ExecutionContext{
		Profile:            profile.Profile{},
		ContainerSockReady: true,
		ContainerEnv:       containerenv.Default(),
	}
	envVars := ContainerEnvVars(ec, "claude")

	want := "unix:///run/container.sock"
	if envVars["DOCKER_HOST"] != want {
		t.Errorf("DOCKER_HOST = %q, want %q", envVars["DOCKER_HOST"], want)
	}
	if envVars["CONTAINER_HOST"] != want {
		t.Errorf("CONTAINER_HOST = %q, want %q", envVars["CONTAINER_HOST"], want)
	}
}

func TestContainerEnvVars_ContainerSock_RespectsExisting(t *testing.T) {
	ec := &ExecutionContext{
		Profile:            profile.Profile{},
		ContainerSockReady: true,
		ContainerEnv:       containerenv.Default(),
		EnvVars: map[string]string{
			"DOCKER_HOST":    "unix:///custom/docker.sock",
			"CONTAINER_HOST": "unix:///custom/podman.sock",
		},
	}
	envVars := ContainerEnvVars(ec, "claude")

	if envVars["DOCKER_HOST"] != "unix:///custom/docker.sock" {
		t.Errorf("DOCKER_HOST should not be overridden, got %q", envVars["DOCKER_HOST"])
	}
	if envVars["CONTAINER_HOST"] != "unix:///custom/podman.sock" {
		t.Errorf("CONTAINER_HOST should not be overridden, got %q", envVars["CONTAINER_HOST"])
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

func TestCollectPackages_FromWorkspaceFile(t *testing.T) {
	workDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(workDir, "packages.txt"), []byte("jq\ntree\n# comment\n\ncurl\n"), 0644); err != nil {
		t.Fatal(err)
	}

	pkgs := CollectPackages(nil, workDir)
	want := []string{"jq", "tree", "curl"}
	if len(pkgs) != len(want) {
		t.Fatalf("CollectPackages() = %v, want %v", pkgs, want)
	}
	for i, p := range pkgs {
		if p != want[i] {
			t.Errorf("CollectPackages()[%d] = %q, want %q", i, p, want[i])
		}
	}
}

func TestCollectPackages_FromProfile(t *testing.T) {
	pkgs := CollectPackages([]string{"jq", "tree"})
	if len(pkgs) != 2 || pkgs[0] != "jq" || pkgs[1] != "tree" {
		t.Errorf("CollectPackages() = %v, want [jq tree]", pkgs)
	}
}

func TestCollectPackages_NoFileNoProfile(t *testing.T) {
	pkgs := CollectPackages(nil)
	if len(pkgs) != 0 {
		t.Errorf("CollectPackages() = %v, want empty", pkgs)
	}
}

func TestCollectPackages_FiltersInvalidNames(t *testing.T) {
	workDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(workDir, "packages.txt"), []byte("jq\n$(evil)\nvalid-pkg\n"), 0644); err != nil {
		t.Fatal(err)
	}

	pkgs := CollectPackages([]string{"good", "rm -rf /", "ok"}, workDir)
	want := []string{"good", "ok", "jq", "valid-pkg"}
	if len(pkgs) != len(want) {
		t.Fatalf("CollectPackages() = %v, want %v", pkgs, want)
	}
	for i, p := range pkgs {
		if p != want[i] {
			t.Errorf("CollectPackages()[%d] = %q, want %q", i, p, want[i])
		}
	}
}
