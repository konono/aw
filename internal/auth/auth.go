package auth

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/konono/aw/internal/docker"
	"github.com/konono/aw/internal/pipeline"
	"github.com/konono/aw/internal/profile"
	"github.com/konono/aw/internal/toolinfo"
)

type Action string

const (
	ActionLogin  Action = "login"
	ActionLogout Action = "logout"
	ActionStatus Action = "status"
)

type CheckResult struct {
	Supported bool
	LoggedIn  bool
	Message   string
}

type commandSpec struct {
	tool    string
	command []string
}

func Run(ctx context.Context, ec *pipeline.ExecutionContext, action Action) error {
	spec, err := resolveActionCommand(ec.Profile, action)
	if err != nil {
		return err
	}

	switch ec.Profile.Environment {
	case profile.EnvironmentHost:
		return runHostCommand(ctx, spec.command)
	case profile.EnvironmentContainer:
		return runContainerCommand(ctx, ec, spec.tool, spec.command)
	default:
		return fmt.Errorf("unsupported environment: %q", ec.Profile.Environment)
	}
}

func CheckLaunch(ctx context.Context, ec *pipeline.ExecutionContext) (CheckResult, error) {
	tool := ec.Profile.EffectiveTool()
	if tool == "" {
		return CheckResult{
			Supported: false,
			Message:   "launch: shell の profile には auth.on_launch.check を使えません",
		}, nil
	}

	if tool == "opencode" {
		return CheckResult{
			Supported: false,
			Message:   "OpenCode は認証状態を厳密に判定しにくいため、auth.on_launch.check は未対応です。必要なら `aw auth status opencode` か `aw auth status --profile <name>` を使ってください",
		}, nil
	}

	if usesExternalAuth(ec.Profile, tool) {
		return CheckResult{
			Supported: false,
			Message:   fmt.Sprintf("この profile は %s の外部認証を使っています。auth.on_launch.check は省略し、env / mounts / 外部資格情報で管理してください", tool),
		}, nil
	}

	spec, err := resolveActionCommand(ec.Profile, ActionStatus)
	if err != nil {
		return CheckResult{}, err
	}

	result, err := runCapturedCommand(ctx, ec, spec.tool, spec.command)
	if err != nil {
		return CheckResult{}, err
	}

	switch tool {
	case "codex", "claude":
		switch result.exitCode {
		case 0:
			return CheckResult{Supported: true, LoggedIn: true}, nil
		case 1:
			return CheckResult{
				Supported: true,
				LoggedIn:  false,
				Message:   fmt.Sprintf("%s はまだ認証されていないようです。必要なら `aw auth login %s` を実行してください", tool, ec.ProfileName),
			}, nil
		default:
			msg := strings.TrimSpace(result.output)
			if msg == "" {
				msg = fmt.Sprintf("%s の認証状態確認に失敗しました", tool)
			}
			return CheckResult{}, fmt.Errorf("%s", msg)
		}
	default:
		return CheckResult{
			Supported: false,
			Message:   fmt.Sprintf("%s の auth.on_launch.check は未対応です", tool),
		}, nil
	}
}

type capturedResult struct {
	output   string
	exitCode int
}

func resolveActionCommand(p profile.Profile, action Action) (commandSpec, error) {
	tool := p.EffectiveTool()
	if tool == "" {
		return commandSpec{}, fmt.Errorf("launch: %q では `aw auth` は使えません", p.Launch)
	}

	switch tool {
	case "codex":
		if usesExternalAuth(p, tool) && (p.Auth == nil || p.Auth.Codex == nil) {
			return commandSpec{}, fmt.Errorf("この profile は環境変数ベースの Codex 認証を使っています。`auth.codex` ではなく外部資格情報を確認してください")
		}
		cfg := profile.CodexAuthConfig{}
		if p.Auth != nil && p.Auth.Codex != nil {
			cfg = *p.Auth.Codex
		}
		return commandSpec{tool: tool, command: buildCodexCommand(action, cfg)}, nil
	case "claude":
		if usesExternalAuth(p, tool) && (p.Auth == nil || p.Auth.Claude == nil) {
			return commandSpec{}, fmt.Errorf("この profile は Claude の外部認証を使っています。`auth.claude` ではなく env / mounts / 外部資格情報を確認してください")
		}
		cfg := profile.ClaudeAuthConfig{}
		if p.Auth != nil && p.Auth.Claude != nil {
			cfg = *p.Auth.Claude
		}
		return commandSpec{tool: tool, command: buildClaudeCommand(action, cfg)}, nil
	case "opencode":
		cfg := profile.OpenCodeAuthConfig{}
		if p.Auth != nil && p.Auth.OpenCode != nil {
			cfg = *p.Auth.OpenCode
		}
		return commandSpec{tool: tool, command: buildOpenCodeCommand(action, cfg)}, nil
	default:
		return commandSpec{}, fmt.Errorf("unsupported auth tool: %q", tool)
	}
}

func buildCodexCommand(action Action, cfg profile.CodexAuthConfig) []string {
	switch action {
	case ActionLogin:
		cmd := []string{"codex", "login"}
		switch cfg.LoginMode {
		case "":
			cmd = append(cmd, "--device-auth")
		case profile.CodexLoginModeBrowser:
			// Explicit browser login.
		case profile.CodexLoginModeDevice:
			cmd = append(cmd, "--device-auth")
		case profile.CodexLoginModeAPIKey:
			cmd = append(cmd, "--with-api-key")
		case profile.CodexLoginModeAccessToken:
			cmd = append(cmd, "--with-access-token")
		}
		return append(cmd, cfg.LoginArgs...)
	case ActionLogout:
		return []string{"codex", "logout"}
	case ActionStatus:
		return []string{"codex", "login", "status"}
	default:
		return []string{"codex", "login"}
	}
}

func buildClaudeCommand(action Action, cfg profile.ClaudeAuthConfig) []string {
	switch action {
	case ActionLogin:
		cmd := []string{"claude", "auth", "login"}
		switch cfg.LoginMode {
		case "", profile.ClaudeLoginModeBrowser:
			// Default browser login.
		case profile.ClaudeLoginModeConsole:
			cmd = append(cmd, "--console")
		case profile.ClaudeLoginModeEmail:
			cmd = append(cmd, "--email")
		case profile.ClaudeLoginModeSSO:
			cmd = append(cmd, "--sso")
		}
		return append(cmd, cfg.LoginArgs...)
	case ActionLogout:
		return []string{"claude", "auth", "logout"}
	case ActionStatus:
		return []string{"claude", "auth", "status", "--text"}
	default:
		return []string{"claude", "auth", "login"}
	}
}

func buildOpenCodeCommand(action Action, cfg profile.OpenCodeAuthConfig) []string {
	switch action {
	case ActionLogin:
		cmd := []string{"opencode", "auth", "login"}
		if cfg.Provider != "" {
			cmd = append(cmd, "--provider", cfg.Provider)
		}
		if cfg.Method != "" {
			cmd = append(cmd, "--method", cfg.Method)
		}
		return append(cmd, cfg.LoginArgs...)
	case ActionLogout:
		return []string{"opencode", "auth", "logout"}
	case ActionStatus:
		return []string{"opencode", "auth", "list"}
	default:
		return []string{"opencode", "auth", "login"}
	}
}

func runHostCommand(ctx context.Context, command []string) error {
	path, err := exec.LookPath(command[0])
	if err != nil {
		return fmt.Errorf("%s is not installed or not in PATH", command[0])
	}
	cmd := exec.CommandContext(ctx, path, command[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	return cmd.Run()
}

func runContainerCommand(ctx context.Context, ec *pipeline.ExecutionContext, tool string, command []string) error {
	runtime := ec.Profile.EffectiveContainerRuntime()
	args := docker.BuildRunArgs(docker.RunConfig{
		ImageName: ec.DockerImage,
		Mounts:    ec.DockerMounts,
		EnvVars:   buildContainerEnvVars(ec, tool),
		WorkDir:   ec.WorkDir,
		Command:   command,
		User:      fmt.Sprintf("%d:0", os.Getuid()),
		Userns:    podmanUserns(runtime),
	})
	cmd := exec.CommandContext(ctx, runtime, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runCapturedCommand(ctx context.Context, ec *pipeline.ExecutionContext, tool string, command []string) (capturedResult, error) {
	switch ec.Profile.Environment {
	case profile.EnvironmentHost:
		path, err := exec.LookPath(command[0])
		if err != nil {
			return capturedResult{}, fmt.Errorf("%s is not installed or not in PATH", command[0])
		}
		cmd := exec.CommandContext(ctx, path, command[1:]...)
		cmd.Env = os.Environ()
		output, err := cmd.CombinedOutput()
		return capturedResultFromOutput(output, err)
	case profile.EnvironmentContainer:
		runtime := ec.Profile.EffectiveContainerRuntime()
		args := docker.BuildRunArgs(docker.RunConfig{
			ImageName: ec.DockerImage,
			Mounts:    ec.DockerMounts,
			EnvVars:   buildContainerEnvVars(ec, tool),
			WorkDir:   ec.WorkDir,
			Command:   command,
			User:      fmt.Sprintf("%d:0", os.Getuid()),
			Userns:    podmanUserns(runtime),
		})
		cmd := exec.CommandContext(ctx, runtime, args...)
		output, err := cmd.CombinedOutput()
		return capturedResultFromOutput(output, err)
	default:
		return capturedResult{}, fmt.Errorf("unsupported environment: %q", ec.Profile.Environment)
	}
}

func capturedResultFromOutput(output []byte, err error) (capturedResult, error) {
	if err == nil {
		return capturedResult{output: string(output), exitCode: 0}, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return capturedResult{output: string(output), exitCode: exitErr.ExitCode()}, nil
	}

	return capturedResult{}, err
}

func podmanUserns(runtime string) string {
	if runtime == "podman" {
		return "keep-id"
	}
	return ""
}

func buildContainerEnvVars(ec *pipeline.ExecutionContext, tool string) map[string]string {
	envVars := toolinfo.ContainerEnvVars(ec.EnvVars, tool)
	envVars["HOST_WORKSPACE"] = ec.WorkDir
	return envVars
}

func usesExternalAuth(p profile.Profile, tool string) bool {
	switch tool {
	case "claude":
		return hasAnyEnv(p.Env,
			"CLAUDE_CODE_USE_VERTEX",
			"CLAUDE_CODE_USE_BEDROCK",
			"CLAUDE_CODE_USE_FOUNDRY",
			"ANTHROPIC_API_KEY",
			"ANTHROPIC_AUTH_TOKEN",
			"CLAUDE_CODE_OAUTH_TOKEN",
		)
	case "codex":
		return hasAnyEnv(p.Env, "OPENAI_API_KEY", "CODEX_ACCESS_TOKEN")
	default:
		return false
	}
}

func hasAnyEnv(env map[string]string, keys ...string) bool {
	for _, key := range keys {
		if env[key] != "" {
			return true
		}
	}
	return false
}
