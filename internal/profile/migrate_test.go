package profile

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestMigrate_UnmodifiedConfig(t *testing.T) {
	templateCfg, err := Parse(DefaultConfigYAML())
	if err != nil {
		t.Fatalf("parse template: %v", err)
	}

	result, err := Migrate(templateCfg)
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	resultCfg, err := Parse(result)
	if err != nil {
		t.Fatalf("parse result: %v", err)
	}

	if resultCfg.Default != templateCfg.Default {
		t.Errorf("Default = %q, want %q", resultCfg.Default, templateCfg.Default)
	}

	for name := range templateCfg.Profiles {
		if _, ok := resultCfg.Profiles[name]; !ok {
			t.Errorf("missing builtin profile %q in migrated output", name)
		}
	}
}

func TestMigrate_PreservesComments(t *testing.T) {
	templateCfg, err := Parse(DefaultConfigYAML())
	if err != nil {
		t.Fatalf("parse template: %v", err)
	}

	result, err := Migrate(templateCfg)
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	output := string(result)
	if !strings.Contains(output, "agent-workspace") {
		t.Error("expected header comment to be preserved in output")
	}
}

func TestMigrate_TopLevelScalarChanges(t *testing.T) {
	userYAML := `
default: shell
environment: container
os: debian12
container_runtime: docker
mount_ssh: true
ssh_agent_forwarding: true
profiles:
  shell:
    launch: shell
`
	userCfg, err := Parse([]byte(userYAML))
	if err != nil {
		t.Fatalf("parse user config: %v", err)
	}

	result, err := Migrate(userCfg)
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	resultCfg, err := Parse(result)
	if err != nil {
		t.Fatalf("parse result: %v", err)
	}

	if resultCfg.Default != "shell" {
		t.Errorf("Default = %q, want %q", resultCfg.Default, "shell")
	}
	if resultCfg.Defaults.ContainerRuntime != ContainerRuntimeDocker {
		t.Errorf("ContainerRuntime = %q, want %q", resultCfg.Defaults.ContainerRuntime, ContainerRuntimeDocker)
	}
	if resultCfg.Defaults.MountSSH == nil || !*resultCfg.Defaults.MountSSH {
		t.Error("MountSSH should be true")
	}
	if resultCfg.Defaults.SSHAgentForwarding == nil || !*resultCfg.Defaults.SSHAgentForwarding {
		t.Error("SSHAgentForwarding should be true")
	}
}

func TestMigrate_BuiltinProfileCustomization(t *testing.T) {
	userYAML := `
default: claude
environment: container
os: debian12
container_runtime: podman
profiles:
  claude:
    launch: claude
    auth:
      on_launch:
        check: warn
      claude:
        login_mode: sso
  shell:
    launch: shell
`
	userCfg, err := Parse([]byte(userYAML))
	if err != nil {
		t.Fatalf("parse user config: %v", err)
	}

	result, err := Migrate(userCfg)
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	resultCfg, err := Parse(result)
	if err != nil {
		t.Fatalf("parse result: %v", err)
	}

	claude, ok := resultCfg.Profiles["claude"]
	if !ok {
		t.Fatal("claude profile missing")
	}
	if claude.Auth == nil || claude.Auth.Claude == nil {
		t.Fatal("claude auth config should be preserved")
	}
	if claude.Auth.Claude.LoginMode != ClaudeLoginModeSSO {
		t.Errorf("claude.auth.claude.login_mode = %q, want %q", claude.Auth.Claude.LoginMode, ClaudeLoginModeSSO)
	}
	if claude.Auth.OnLaunch == nil || claude.Auth.OnLaunch.Check != AuthOnLaunchCheckWarn {
		t.Error("claude.auth.on_launch.check should be 'warn'")
	}
}

func TestMigrate_UserOnlyProfiles(t *testing.T) {
	userYAML := `
default: claude
environment: container
os: debian12
container_runtime: podman
profiles:
  claude:
    launch: claude
  shell:
    launch: shell
  my-custom:
    environment: host
    launch: shell
    env:
      FOO: bar
  vertex-claude:
    launch: claude
    env:
      CLAUDE_CODE_USE_VERTEX: "1"
    mounts:
      - source: "~/.config/gcloud"
        target: "/home/agent/.config/gcloud"
`
	userCfg, err := Parse([]byte(userYAML))
	if err != nil {
		t.Fatalf("parse user config: %v", err)
	}

	result, err := Migrate(userCfg)
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	resultCfg, err := Parse(result)
	if err != nil {
		t.Fatalf("parse result: %v", err)
	}

	custom, ok := resultCfg.Profiles["my-custom"]
	if !ok {
		t.Fatal("user profile 'my-custom' should be preserved")
	}
	if custom.Environment != EnvironmentHost {
		t.Errorf("my-custom.environment = %q, want %q", custom.Environment, EnvironmentHost)
	}
	if custom.Launch != LaunchShell {
		t.Errorf("my-custom.launch = %q, want %q", custom.Launch, LaunchShell)
	}
	if custom.Env["FOO"] != "bar" {
		t.Errorf("my-custom.env.FOO = %q, want %q", custom.Env["FOO"], "bar")
	}

	vertex, ok := resultCfg.Profiles["vertex-claude"]
	if !ok {
		t.Fatal("user profile 'vertex-claude' should be preserved")
	}
	if len(vertex.Mounts) != 1 {
		t.Fatalf("vertex-claude should have 1 mount, got %d", len(vertex.Mounts))
	}
	if vertex.Mounts[0].Source != "~/.config/gcloud" {
		t.Errorf("mount source = %q, want %q", vertex.Mounts[0].Source, "~/.config/gcloud")
	}
}

func TestMigrate_TopLevelEnv(t *testing.T) {
	userYAML := `
default: claude
environment: container
os: debian12
container_runtime: podman
env:
  CLAUDE_CODE_USE_VERTEX: "1"
  CLOUD_ML_REGION: us-east5
profiles:
  claude:
    launch: claude
`
	userCfg, err := Parse([]byte(userYAML))
	if err != nil {
		t.Fatalf("parse user config: %v", err)
	}

	result, err := Migrate(userCfg)
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	resultCfg, err := Parse(result)
	if err != nil {
		t.Fatalf("parse result: %v", err)
	}

	if resultCfg.Defaults.Env["CLAUDE_CODE_USE_VERTEX"] != "1" {
		t.Error("top-level env CLAUDE_CODE_USE_VERTEX should be preserved")
	}
	if resultCfg.Defaults.Env["CLOUD_ML_REGION"] != "us-east5" {
		t.Error("top-level env CLOUD_ML_REGION should be preserved")
	}
}

func TestMigrate_RoundTripEffectiveConfig(t *testing.T) {
	userYAML := `
default: claude
environment: container
os: debian12
container_runtime: docker
mount_ssh: true
env:
  MY_VAR: hello
profiles:
  claude:
    launch: claude
    auth:
      claude:
        login_mode: sso
  shell:
    launch: shell
  my-project:
    environment: host
    launch: shell
    env:
      PROJECT: test
`
	userCfg, err := Parse([]byte(userYAML))
	if err != nil {
		t.Fatalf("parse user config: %v", err)
	}

	originalMerged := MergeConfig(builtinConfig, *userCfg)
	originalEffective := ApplyDefaults(originalMerged)

	migrated, err := Migrate(userCfg)
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	migratedCfg, err := Parse(migrated)
	if err != nil {
		t.Fatalf("parse migrated: %v", err)
	}

	migratedMerged := MergeConfig(builtinConfig, *migratedCfg)
	migratedEffective := ApplyDefaults(migratedMerged)

	for name, origProfile := range originalEffective.Profiles {
		migratedProfile, ok := migratedEffective.Profiles[name]
		if !ok {
			t.Errorf("profile %q missing in migrated config", name)
			continue
		}
		if origProfile.Environment != migratedProfile.Environment {
			t.Errorf("profile %q: Environment = %q, want %q", name, migratedProfile.Environment, origProfile.Environment)
		}
		if origProfile.Launch != migratedProfile.Launch {
			t.Errorf("profile %q: Launch = %q, want %q", name, migratedProfile.Launch, origProfile.Launch)
		}
		if origProfile.ContainerRuntime != migratedProfile.ContainerRuntime {
			t.Errorf("profile %q: ContainerRuntime = %q, want %q", name, migratedProfile.ContainerRuntime, origProfile.ContainerRuntime)
		}
		if origProfile.EffectiveMountSSH() != migratedProfile.EffectiveMountSSH() {
			t.Errorf("profile %q: EffectiveMountSSH = %v, want %v", name, migratedProfile.EffectiveMountSSH(), origProfile.EffectiveMountSSH())
		}
	}

	if migratedEffective.Profiles["my-project"].Env["PROJECT"] != "test" {
		t.Error("user profile env should be preserved through round-trip")
	}
}

func TestMigrate_MountGHExplicitTrue(t *testing.T) {
	userYAML := `
default: claude
environment: container
os: debian12
container_runtime: podman
mount_gh: true
profiles:
  claude:
    launch: claude
`
	userCfg, err := Parse([]byte(userYAML))
	if err != nil {
		t.Fatalf("parse user config: %v", err)
	}

	result, err := Migrate(userCfg)
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	resultCfg, err := Parse(result)
	if err != nil {
		t.Fatalf("parse result: %v", err)
	}

	if resultCfg.Defaults.MountGH == nil || !*resultCfg.Defaults.MountGH {
		t.Error("mount_gh should be true in migrated config")
	}
}

func TestMigrate_OutputIsValidYAML(t *testing.T) {
	userYAML := `
default: shell
environment: host
profiles:
  shell:
    launch: shell
  custom:
    environment: container
    launch: claude
    os: ubi10
    mounts:
      - source: ~/.config/gcloud
        target: /home/agent/.config/gcloud
        mode: ro
`
	userCfg, err := Parse([]byte(userYAML))
	if err != nil {
		t.Fatalf("parse user config: %v", err)
	}

	result, err := Migrate(userCfg)
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(result, &doc); err != nil {
		t.Fatalf("migrated output is invalid YAML: %v", err)
	}

	resultCfg, err := Parse(result)
	if err != nil {
		t.Fatalf("migrated output does not parse as Config: %v", err)
	}

	merged := MergeConfig(builtinConfig, *resultCfg)
	applied := ApplyDefaults(merged)
	if err := ValidateConfig(&applied); err != nil {
		t.Errorf("migrated config fails validation: %v", err)
	}
}
