package launcher

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/konono/aw/internal/docker"
	"github.com/konono/aw/internal/pipeline"
	"github.com/konono/aw/internal/profile"
)

// layoutData holds template variables for the zellij layout.
type layoutData struct {
	ScriptsDir   string
	AgentCommand string
	AgentName    string
}

// ZellijLauncher launches a zellij session with multiple panes.
type ZellijLauncher struct{}

func (l *ZellijLauncher) Launch(_ context.Context, ec *pipeline.ExecutionContext) error {
	if _, err := exec.LookPath("zellij"); err != nil {
		return fmt.Errorf("zellij is not installed (brew install zellij)")
	}

	sessionName := ec.WorktreeBranch
	if sessionName == "" {
		sessionName = ec.ProfileName
	}

	dir, err := l.prepareFiles(ec, sessionName)
	if err != nil {
		return fmt.Errorf("preparing zellij files: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Launching zellij session: %s\n", sessionName)
	return l.launchZellij(ec.WorkDir, dir, sessionName, ec.WorktreeBase)
}

func (l *ZellijLauncher) prepareFiles(ec *pipeline.ExecutionContext, sessionName string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home dir: %w", err)
	}

	baseDir := filepath.Join(homeDir, ".cache", "agent-workspace", "zellij", sessionName)
	scriptsDir := filepath.Join(baseDir, "scripts")
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		return "", fmt.Errorf("creating scripts dir: %w", err)
	}

	// Write shell scripts
	scripts := map[string][]byte{
		"plans-watcher.sh":   plansWatcherSh,
		"git-diff-picker.sh": gitDiffPickerSh,
		"pr-status.sh":       prStatusSh,
	}
	for name, content := range scripts {
		path := filepath.Join(scriptsDir, name)
		if err := os.WriteFile(path, content, 0755); err != nil {
			return "", fmt.Errorf("writing %s: %w", name, err)
		}
	}

	// Build agent command based on environment and tool
	tool := ec.Profile.EffectiveTool()
	agentCmd := l.buildAgentCommand(ec, tool)
	agentName := "Claude Code"
	switch tool {
	case "codex":
		agentName = "Codex"
	case "opencode":
		agentName = "OpenCode"
	}

	// Render and write layout template
	tmpl, err := template.New("layout").Parse(string(layoutKdlTmpl))
	if err != nil {
		return "", fmt.Errorf("parsing layout template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, layoutData{
		ScriptsDir:   scriptsDir,
		AgentCommand: agentCmd,
		AgentName:    agentName,
	}); err != nil {
		return "", fmt.Errorf("rendering layout template: %w", err)
	}

	layoutPath := filepath.Join(baseDir, "layout.kdl")
	if err := os.WriteFile(layoutPath, buf.Bytes(), 0644); err != nil {
		return "", fmt.Errorf("writing layout file: %w", err)
	}

	return baseDir, nil
}

func (l *ZellijLauncher) buildAgentCommand(ec *pipeline.ExecutionContext, tool string) string {
	agentCmd := "claude --permission-mode bypassPermissions"
	agentBin := "claude"
	if tool == "codex" {
		agentCmd = "codex -a never"
		agentBin = "codex"
	} else if tool == "opencode" {
		agentCmd = "opencode"
		agentBin = "opencode"
	}

	switch ec.Profile.Environment {
	case profile.EnvironmentContainer:
		envVars := buildContainerEnvVars(ec, tool)
		envVars["HOST_WORKSPACE"] = ec.WorkDir

		runConfig := docker.RunConfig{
			ImageName: ec.DockerImage,
			Mounts:    ec.DockerMounts,
			EnvVars:   envVars,
			WorkDir:   ec.WorkDir,
			Command:   []string{"bash", "-c", agentCmd + "; exec bash -i"},
		}
		args := docker.BuildRunArgs(runConfig)
		return ec.Profile.EffectiveContainerRuntime() + " " + shellJoin(args)
	default:
		return agentBin
	}
}

// shellJoin quotes arguments for safe shell embedding.
func shellJoin(args []string) string {
	quoted := make([]string, len(args))
	for i, a := range args {
		if strings.ContainsAny(a, " \t\n\"'\\$`!#&|;(){}") {
			quoted[i] = "'" + strings.ReplaceAll(a, "'", "'\"'\"'") + "'"
		} else {
			quoted[i] = a
		}
	}
	return strings.Join(quoted, " ")
}

func (l *ZellijLauncher) launchZellij(workDir, dir, sessionName, baseRef string) error {
	layoutPath := filepath.Join(dir, "layout.kdl")

	// If already inside a zellij session, open a new tab instead of nesting
	if os.Getenv("ZELLIJ") != "" {
		fmt.Fprintf(os.Stderr, "Already inside zellij, opening new tab: %s\n", sessionName)
		cmd := exec.Command("zellij", "action", "new-tab", "--layout", layoutPath, "--name", sessionName)
		cmd.Dir = workDir
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Env = os.Environ()
		if baseRef != "" {
			cmd.Env = append(cmd.Env, "AW_BASE_REF="+baseRef)
		}
		return cmd.Run()
	}

	cmd := exec.Command("zellij",
		"--new-session-with-layout", layoutPath,
		"-s", sessionName)
	cmd.Dir = workDir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	if baseRef != "" {
		cmd.Env = append(cmd.Env, "AW_BASE_REF="+baseRef)
	}
	return cmd.Run()
}
