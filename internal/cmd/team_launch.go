package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"time"

	"github.com/google/uuid"
	"github.com/konono/aw/v4/internal/docker"
	"github.com/konono/aw/v4/internal/launcher"
	"github.com/konono/aw/v4/internal/linuxbin"
	"github.com/konono/aw/v4/internal/messaging/inject"
	"github.com/konono/aw/v4/internal/messaging/roles"
	"github.com/konono/aw/v4/internal/pipeline"
	"github.com/konono/aw/v4/internal/profile"
	"github.com/konono/aw/v4/internal/reaper"
	"github.com/konono/aw/v4/internal/stage"
	"github.com/konono/aw/v4/internal/team"
)

type teamLaunchOpts struct {
	cfg             *profile.Config
	scope           string
	msgDBDir        string
	memberInfos     []roles.MemberData
	injectMembers   []inject.MemberInfo
	resume          bool
	task            string
	teamName        string
	repoRoot        string
}

func launchTeamMember(
	ctx context.Context,
	opts teamLaunchOpts,
	m team.ResolvedMember,
	containerName string,
	toolSessionID string,
	foreground bool,
) (team.MemberState, *os.File, error) {
	p, ok := opts.cfg.Profiles[m.Profile]
	if !ok {
		return team.MemberState{}, nil, fmt.Errorf("profile %q not found", m.Profile)
	}

	ec, err := buildExecutionContext(m.Profile, p)
	if err != nil {
		return team.MemberState{}, nil, fmt.Errorf("building execution context: %w", err)
	}
	ec.ContainerName = containerName

	var worktreePath, branchName string
	if opts.repoRoot != "" {
		branchName = teamBranchName(opts.teamName, m.AgentName)
		worktreePath = teamWorktreePath(opts.repoRoot, opts.teamName, m.AgentName)
		if err := ensureWorktree(opts.repoRoot, branchName, worktreePath, "HEAD", opts.resume); err != nil {
			return team.MemberState{}, nil, fmt.Errorf("creating worktree for %s: %w", m.AgentName, err)
		}
		ec.WorkDir = worktreePath
	}

	dockerStage := stage.NewDockerStage()
	if err := dockerStage.Run(ctx, ec); err != nil {
		return team.MemberState{}, nil, fmt.Errorf("docker stage: %w", err)
	}

	envStage := &stage.EnvStage{}
	if err := envStage.Run(ctx, ec); err != nil {
		return team.MemberState{}, nil, fmt.Errorf("env stage: %w", err)
	}

	tool := p.EffectiveTool()
	setupTeamMemberMCP(tool, ec, m, opts)
	injectTeamRoleContext(tool, ec, m, opts)

	awBin, err := linuxbin.Resolve(goruntime.GOARCH)
	if err != nil {
		return team.MemberState{}, nil, fmt.Errorf("resolving linux binary: %w", err)
	}

	ec.DockerMounts = append(ec.DockerMounts,
		docker.Mount{
			Source:   opts.msgDBDir,
			Target:   "/home/agent/.aw-msg",
			ReadOnly: false,
			Options:  "z",
		},
		docker.Mount{
			Source:   awBin,
			Target:   "/home/agent/.aw-msg/bin/aw",
			ReadOnly: true,
			Options:  "z",
		},
	)

	runtime := p.EffectiveContainerRuntime()
	command := ec.CommandOverride
	if len(command) == 0 {
		command = launcher.ToolContainerCommand(tool)
		if command == nil {
			command = []string{tool}
		}
	}

	if opts.resume {
		command = launcher.AppendResumeFlags(tool, command)
	}

	if !foreground && launcher.SupportsAgentLoop(tool) {
		command = []string{"/home/agent/.aw-msg/bin/aw", "internal-agent-loop"}
	}

	runConfig := pipeline.ToolRunConfig(ec, runtime, tool, command)
	runConfig.EnvVars["AW_AGENT_NAME"] = m.AgentName
	runConfig.EnvVars["AW_MSG_DB"] = "/home/agent/.aw-msg/messages.db"
	runConfig.EnvVars["AW_TEAM_NAME"] = opts.scope
	runConfig.EnvVars["AW_SESSION_ID"] = toolSessionID
	runConfig.EnvVars["AW_TOOL"] = tool

	if foreground {
		client := docker.NewShellClient(runtime)
		spec := reaper.BuildSpec(ec)
		return team.MemberState{}, nil, client.ExecRun(containerName, runConfig, func() (*os.File, func(), error) {
			handle, err := reaper.Spawn(spec)
			if err != nil {
				return nil, nil, err
			}
			return handle.Write, handle.Abort, nil
		})
	}

	client := docker.NewShellClient(runtime)
	if err := client.StartDetached(containerName, runConfig); err != nil {
		return team.MemberState{}, nil, fmt.Errorf("starting container: %w", err)
	}

	var reaperFd *os.File
	spec := reaper.BuildSpec(ec)
	handle, err := reaper.Spawn(spec)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: reaper spawn for %s: %v\n", m.AgentName, err)
	} else {
		reaperFd = handle.Write
	}

	return team.MemberState{
		AgentName:     m.AgentName,
		Profile:       m.Profile,
		Role:          m.Role,
		ContainerName: containerName,
		Runtime:       runtime,
		ToolSessionID: toolSessionID,
		Foreground:    false,
		Status:        "running",
		WorktreePath:  worktreePath,
		BranchName:    branchName,
	}, reaperFd, nil
}

func setupTeamMemberMCP(tool string, ec *pipeline.ExecutionContext, m team.ResolvedMember, opts teamLaunchOpts) {
	toolStageDir := filepath.Join(ec.HomeDir, ".agent-workspace", tool)
	deliveryMode := inject.DeliveryMode(ec.Profile.EffectiveDelivery(tool))

	injector, err := inject.ForTool(tool)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: messaging injection not supported for %s: %v\n", tool, err)
		return
	}
	injectCfg := inject.InjectorConfig{
		AgentName:    m.AgentName,
		TeamName:     opts.scope,
		Role:         m.Role,
		Members:      opts.injectMembers,
		StagingDir:   toolStageDir,
		WorkDir:      ec.WorkDir,
		MCPBinary:    "/home/agent/.aw-msg/bin/aw",
		DBPath:       "/home/agent/.aw-msg/messages.db",
		DeliveryMode: deliveryMode,
	}
	if ierr := injector.InjectMCP(injectCfg); ierr != nil {
		fmt.Fprintf(os.Stderr, "Warning: MCP injection: %v\n", ierr)
	}
	if ierr := injector.InjectHook(injectCfg); ierr != nil {
		fmt.Fprintf(os.Stderr, "Warning: hook injection: %v\n", ierr)
	}
}

func injectTeamRoleContext(tool string, ec *pipeline.ExecutionContext, m team.ResolvedMember, opts teamLaunchOpts) {
	toolStageDir := filepath.Join(ec.HomeDir, ".agent-workspace", tool)
	roleData := roles.TemplateData{
		TeamName:  opts.scope,
		AgentName: m.AgentName,
		Task:      opts.task,
	}
	for _, mi := range opts.memberInfos {
		roleData.Members = append(roleData.Members, roles.MemberData{
			AgentName: mi.AgentName,
			Role:      mi.Role,
			IsSelf:    mi.AgentName == m.AgentName,
		})
	}
	if roleText, rerr := roles.Render(m.Role, roleData); rerr == nil {
		appendRoleContext(toolStageDir, tool, roleText)
		if ec.WorkDir != "" && ec.WorkDir != toolStageDir {
			appendRoleContext(ec.WorkDir, tool, roleText)
		}
	}
}

// toolContextFile returns the context filename for the given tool,
// or empty string if the tool doesn't support context injection.
func toolContextFile(tool string) string {
	switch tool {
	case "claude", "cursor":
		return "CLAUDE.md"
	case "codex", "opencode":
		return "AGENTS.md"
	default:
		return ""
	}
}

func appendRoleContext(toolStageDir, tool, roleText string) {
	filename := toolContextFile(tool)
	if filename == "" {
		return
	}

	path := filepath.Join(toolStageDir, filename)
	existing, _ := os.ReadFile(path)
	content := string(existing)
	content += "\n" + roleText
	_ = os.WriteFile(path, []byte(content), 0644)
}

func ensureWorktree(repoRoot, branchName, worktreePath, base string, resume bool) error {
	if _, err := os.Stat(worktreePath); err == nil {
		if resume {
			return nil
		}
		return fmt.Errorf("worktree already exists at %s (use --resume to reuse)", worktreePath)
	}
	if err := os.MkdirAll(filepath.Dir(worktreePath), 0o755); err != nil {
		return fmt.Errorf("creating worktree parent dir: %w", err)
	}
	cmd := exec.Command("git", "-C", repoRoot, "worktree", "add", "-b", branchName, worktreePath, base)
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func resolveToolSessionID(prevSessions map[string]string, agentName string) string {
	if prev, ok := prevSessions[agentName]; ok && prev != "" {
		return prev
	}
	return uuid.New().String()
}

func teamContainerName(teamName, agentName string) string {
	return fmt.Sprintf("aw-%s-%s-%d", teamName, agentName, time.Now().UnixNano())
}

func teamBranchName(teamName, agentName string) string {
	return fmt.Sprintf("aw/%s/%s", teamName, agentName)
}

func teamWorktreePath(repoRoot, teamName, agentName string) string {
	return filepath.Join(repoRoot, "worktrees", fmt.Sprintf("aw-%s-%s", teamName, agentName))
}

func closeReaperFds(fds []*os.File) {
	for _, f := range fds {
		_ = f.Close()
	}
}
