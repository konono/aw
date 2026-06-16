package pipeline

import (
	"github.com/konono/aw/internal/containerenv"
	"github.com/konono/aw/internal/docker"
	"github.com/konono/aw/internal/toolinfo"
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

// authContainerEnv returns ec.ContainerEnv when DockerStage has run, otherwise Default().
func authContainerEnv(ec *ExecutionContext) containerenv.Config {
	if ec.ContainerEnv.User != "" {
		return ec.ContainerEnv
	}
	return containerenv.Default()
}

// AuthRunConfig constructs a minimal RunConfig for one-shot auth commands.
// Unlike ShellRunConfig/ToolRunConfig, auth omits SSH agent, gh token, skip flags,
// and security/capability options to match pre-refactor behavior.
func AuthRunConfig(ec *ExecutionContext, runtime string, tool string, command []string) docker.RunConfig {
	cenv := authContainerEnv(ec)
	envVars := toolinfo.ContainerEnvVarsFor(ec.EnvVars, tool, cenv)
	envVars["AW_USER"] = cenv.User
	envVars["AW_HOME"] = cenv.Home
	envVars["HOST_WORKSPACE"] = ec.WorkDir
	return docker.RunConfig{
		ImageName: ec.DockerImage,
		Mounts:    ec.DockerMounts,
		EnvVars:   envVars,
		WorkDir:   ec.WorkDir,
		Command:   command,
		User:      docker.HostUserID(),
		Userns:    docker.PodmanUserns(runtime),
	}
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
