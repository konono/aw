package pipeline

import (
	"github.com/konono/aw/internal/docker"
)

// BuildRunConfig constructs a docker.RunConfig from pipeline state.
func BuildRunConfig(ec *ExecutionContext, runtime string, command []string, envVars map[string]string) docker.RunConfig {
	return docker.RunConfig{
		ImageName:    ec.DockerImage,
		Mounts:       ec.DockerMounts,
		EnvVars:      envVars,
		WorkDir:      ec.WorkDir,
		Command:      command,
		SecurityOpts: ec.DockerSecurityOpts,
		CapAdd:       ec.DockerCapAdd,
		User:         docker.HostUserID(),
		Userns:       docker.PodmanUserns(runtime),
	}
}

// AuthRunConfig constructs a minimal RunConfig for one-shot auth commands.
func AuthRunConfig(ec *ExecutionContext, runtime string, tool string, command []string) docker.RunConfig {
	envVars := ContainerEnvVars(ec, tool)
	envVars["HOST_WORKSPACE"] = ec.WorkDir
	return BuildRunConfig(ec, runtime, command, envVars)
}

// ShellEnvTool returns the tool name used when building container env for shell launch.
// Shell profiles have no EffectiveTool; claude paths are used for staging compatibility.
func ShellEnvTool(ec *ExecutionContext) string {
	tool := ec.Profile.EffectiveTool()
	if tool == "" {
		return "claude"
	}
	return tool
}

// ShellRunConfig constructs a RunConfig for an interactive shell in the container.
func ShellRunConfig(ec *ExecutionContext, runtime string, command []string) docker.RunConfig {
	envVars := ContainerEnvVars(ec, ShellEnvTool(ec))
	envVars["HOST_WORKSPACE"] = ec.WorkDir
	return BuildRunConfig(ec, runtime, command, envVars)
}

// ToolRunConfig constructs a RunConfig for launching an AI tool in the container.
func ToolRunConfig(ec *ExecutionContext, runtime, tool string, command []string) docker.RunConfig {
	envVars := ContainerEnvVars(ec, tool)
	envVars["HOST_WORKSPACE"] = ec.WorkDir
	return BuildRunConfig(ec, runtime, command, envVars)
}
