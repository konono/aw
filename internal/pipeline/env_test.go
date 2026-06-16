package pipeline

import (
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
