package cmd

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/konono/aw/v4/internal/docker"
	"github.com/konono/aw/v4/internal/launcher"
	"github.com/konono/aw/v4/internal/linuxbin"
	"github.com/konono/aw/v4/internal/messaging"
	"github.com/konono/aw/v4/internal/messaging/inject"
	"github.com/konono/aw/v4/internal/messaging/roles"
	"github.com/konono/aw/v4/internal/pipeline"
	"github.com/konono/aw/v4/internal/platform"
	"github.com/konono/aw/v4/internal/profile"
	"github.com/konono/aw/v4/internal/reaper"
	"github.com/konono/aw/v4/internal/stage"
	"github.com/konono/aw/v4/internal/team"
)

func projectHash(dir string) string {
	h := sha256.Sum256([]byte(dir))
	return fmt.Sprintf("%x", h)[:12]
}

func teamScope(teamName, projHash, sessionID string) string {
	return fmt.Sprintf("%s-%s-%s", teamName, projHash, sessionID[:12])
}

type teamSession struct {
	sessionID       string
	scope           string
	projHash        string
	members         []team.ResolvedMember
	fgMember        *team.ResolvedMember
	bgMembers       []team.ResolvedMember
	memberInfos     []roles.MemberData
	injectMembers   []inject.MemberInfo
	msgDBDir        string
	prevSessions    map[string]string
	repoRoot        string
	branchIsolation bool
}

func initTeamSession(teamName string, t profile.Team, resume bool) (*teamSession, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	projHash := projectHash(cwd)

	var sessionID string
	var prevState *team.TeamState
	if resume {
		prevState, err = team.LoadState(teamName)
		if err != nil {
			return nil, fmt.Errorf("no previous session to resume: %w", err)
		}
		sessionID = prevState.SessionID
		fmt.Fprintf(os.Stderr, "[team:%s] Resuming session %s\n", teamName, sessionID[:12])
		for _, m := range prevState.Members {
			rt := m.Runtime
			if rt == "" {
				rt = "docker"
			}
			_ = stopContainerQuiet(docker.NewShellClient(rt).DockerCmd(), m.ContainerName)
		}
	} else {
		sessionID = uuid.New().String()
	}

	scope := teamScope(teamName, projHash, sessionID)
	mgr := team.NewManager()
	members := mgr.ResolveMembers(teamName, t)

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	msgDBDir := filepath.Join(homeDir, ".config", "aw", "messaging")
	msgDBPath := filepath.Join(msgDBDir, "messages.db")
	store, err := messaging.OpenStore(msgDBPath)
	if err != nil {
		return nil, fmt.Errorf("initializing messaging DB: %w", err)
	}
	_ = store.Close()

	memberInfos := make([]roles.MemberData, len(members))
	injectMembers := make([]inject.MemberInfo, len(members))
	for i, m := range members {
		memberInfos[i] = roles.MemberData{AgentName: m.AgentName, Role: m.Role}
		injectMembers[i] = inject.MemberInfo{AgentName: m.AgentName, Role: m.Role}
	}

	var fgMember *team.ResolvedMember
	var bgMembers []team.ResolvedMember
	for i := range members {
		if members[i].Foreground {
			fgMember = &members[i]
		} else {
			bgMembers = append(bgMembers, members[i])
		}
	}
	if fgMember == nil {
		fgMember = &members[0]
		bgMembers = members[1:]
	}

	prevToolSessions := map[string]string{}
	if prevState != nil {
		for _, m := range prevState.Members {
			prevToolSessions[m.AgentName] = m.ToolSessionID
		}
	}

	var repoRoot string
	if out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output(); err == nil {
		repoRoot = strings.TrimSpace(string(out))
	}

	return &teamSession{
		sessionID:       sessionID,
		scope:           scope,
		projHash:        projHash,
		members:         members,
		fgMember:        fgMember,
		bgMembers:       bgMembers,
		memberInfos:     memberInfos,
		injectMembers:   injectMembers,
		msgDBDir:        msgDBDir,
		prevSessions:    prevToolSessions,
		repoRoot:        repoRoot,
		branchIsolation: repoRoot != "",
	}, nil
}

// Run handles team start.
func (t *TeamStartCmd) Run() error {
	teamName := t.TeamName

	if platform.IsRunningAsRoot() {
		return fmt.Errorf("aw must not be run as root")
	}

	cfg, _, err := loadTeamConfig(teamName)
	if err != nil {
		return err
	}

	sess, err := initTeamSession(teamName, cfg.Teams[teamName], t.Resume)
	if err != nil {
		return err
	}

	teamState := team.TeamState{
		Name:        teamName,
		SessionID:   sess.sessionID,
		ProjectHash: sess.projHash,
		TeamScope:   sess.scope,
		StartedAt:   time.Now().UTC().Format(time.RFC3339),
	}

	ctx := context.Background()
	var bgReaperFds []*os.File

	launchOpts := teamLaunchOpts{
		cfg:             cfg,
		scope:           sess.scope,
		msgDBDir:        sess.msgDBDir,
		memberInfos:     sess.memberInfos,
		injectMembers:   sess.injectMembers,
		resume:          t.Resume,
		prevSessions:    sess.prevSessions,
		task:            t.Task,
		teamName:        teamName,
		branchIsolation: sess.branchIsolation,
		repoRoot:        sess.repoRoot,
	}

	for _, m := range sess.bgMembers {
		containerName := fmt.Sprintf("aw-%s-%s-%d", teamName, m.AgentName, time.Now().UnixNano())
		ms, reaperFd, err := launchTeamMember(ctx, launchOpts, m, containerName, false)
		if err != nil {
			closeReaperFds(bgReaperFds)
			stopTeamContainers(teamState)
			return fmt.Errorf("starting %s: %w", m.AgentName, err)
		}
		if reaperFd != nil {
			bgReaperFds = append(bgReaperFds, reaperFd)
		}
		teamState.Members = append(teamState.Members, ms)
		fmt.Fprintf(os.Stderr, "[team:%s] Started %s (%s) in background\n", teamName, m.AgentName, m.Profile)
	}

	fgContainerName := fmt.Sprintf("aw-%s-%s-%d", teamName, sess.fgMember.AgentName, time.Now().UnixNano())
	fgRuntime := "docker"
	if fgProfile, ok := cfg.Profiles[sess.fgMember.Profile]; ok {
		fgRuntime = fgProfile.EffectiveContainerRuntime()
	}
	fgToolSessionID := uuid.New().String()
	if prev, ok := sess.prevSessions[sess.fgMember.AgentName]; ok && prev != "" {
		fgToolSessionID = prev
	}
	fgState := team.MemberState{
		AgentName:     sess.fgMember.AgentName,
		Profile:       sess.fgMember.Profile,
		Role:          sess.fgMember.Role,
		ContainerName: fgContainerName,
		Runtime:       fgRuntime,
		ToolSessionID: fgToolSessionID,
		Foreground:    true,
		Status:        "running",
	}
	if launchOpts.branchIsolation {
		fgState.BranchName = fmt.Sprintf("aw/%s/%s", teamName, sess.fgMember.AgentName)
		fgState.WorktreePath = filepath.Join(launchOpts.repoRoot, "worktrees", fmt.Sprintf("aw-%s-%s", teamName, sess.fgMember.AgentName))
	}
	teamState.Members = append(teamState.Members, fgState)
	if err := team.SaveState(teamState); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not save team state: %v\n", err)
	}

	fmt.Fprintf(os.Stderr, "[team:%s] Starting %s (%s) in foreground...\n", teamName, sess.fgMember.AgentName, sess.fgMember.Profile)

	if _, _, err := launchTeamMember(ctx, launchOpts, *sess.fgMember, fgContainerName, true); err != nil {
		closeReaperFds(bgReaperFds)
		stopTeamContainers(teamState)
		return fmt.Errorf("starting %s: %w", sess.fgMember.AgentName, err)
	}

	return nil
}

func loadTeamConfig(teamName string) (*profile.Config, profile.Team, error) {
	cfg, err := profile.Load()
	if err != nil {
		return nil, profile.Team{}, fmt.Errorf("loading config: %w", err)
	}
	if err := profile.ValidateConfig(cfg); err != nil {
		return nil, profile.Team{}, err
	}
	t, ok := cfg.Teams[teamName]
	if !ok {
		if len(cfg.Teams) > 0 {
			fmt.Fprintln(os.Stderr, "Available teams:")
			for name := range cfg.Teams {
				fmt.Fprintf(os.Stderr, "  %s\n", name)
			}
		}
		return nil, profile.Team{}, fmt.Errorf("team %q not found", teamName)
	}
	return cfg, t, nil
}

type teamLaunchOpts struct {
	cfg             *profile.Config
	scope           string
	msgDBDir        string
	memberInfos     []roles.MemberData
	injectMembers   []inject.MemberInfo
	resume          bool
	prevSessions    map[string]string
	task            string
	teamName        string
	branchIsolation bool
	repoRoot        string
}

func closeReaperFds(fds []*os.File) {
	for _, f := range fds {
		_ = f.Close()
	}
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

func launchTeamMember(
	ctx context.Context,
	opts teamLaunchOpts,
	m team.ResolvedMember,
	containerName string,
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
	if opts.branchIsolation {
		branchName = fmt.Sprintf("aw/%s/%s", opts.teamName, m.AgentName)
		worktreePath = filepath.Join(opts.repoRoot, "worktrees", fmt.Sprintf("aw-%s-%s", opts.teamName, m.AgentName))
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

	toolSessionID := uuid.New().String()
	if prev, ok := opts.prevSessions[m.AgentName]; ok && prev != "" {
		toolSessionID = prev
	}

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

func stopContainerQuiet(dockerCmd, containerName string) error {
	cmd := exec.Command(dockerCmd, "stop", containerName)
	output, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	outStr := string(output)
	if strings.Contains(outStr, "no such container") || strings.Contains(outStr, "no container with name") {
		fmt.Printf("  Already stopped.\n")
		return nil
	}
	return fmt.Errorf("%s: %s", err, strings.TrimSpace(outStr))
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

func appendRoleContext(toolStageDir, tool, roleText string) {
	var filename string
	switch tool {
	case "claude":
		filename = "CLAUDE.md"
	case "codex", "opencode":
		filename = "AGENTS.md"
	default:
		return
	}

	path := filepath.Join(toolStageDir, filename)
	existing, _ := os.ReadFile(path)
	content := string(existing)
	content += "\n" + roleText
	_ = os.WriteFile(path, []byte(content), 0644)
}

func stopTeamContainers(state team.TeamState) {
	for _, m := range state.Members {
		runtime := m.Runtime
		if runtime == "" {
			runtime = "docker"
		}
		_ = stopContainerQuiet(docker.NewShellClient(runtime).DockerCmd(), m.ContainerName)
	}
	_ = team.RemoveState(state.Name)
}

// Run handles team stop.
func (t *TeamStopCmd) Run() error {
	teamName := t.TeamName

	state, err := team.LoadState(teamName)
	if err != nil {
		return fmt.Errorf("team %q is not running: %w", teamName, err)
	}

	fmt.Printf("[team:%s] Stopping %d members...\n", teamName, len(state.Members))
	for _, m := range state.Members {
		runtime := m.Runtime
		if runtime == "" {
			runtime = "docker"
		}
		rt := docker.NewShellClient(runtime)
		fmt.Printf("  Stopping %s (%s)...\n", m.AgentName, m.ContainerName)
		if err := stopContainerQuiet(rt.DockerCmd(), m.ContainerName); err != nil {
			fmt.Fprintf(os.Stderr, "  Warning: %v\n", err)
		}
	}

	// Print worktree info if any members had branch isolation
	var hasWorktrees bool
	for _, m := range state.Members {
		if m.WorktreePath != "" {
			if !hasWorktrees {
				fmt.Printf("\nWorktrees preserved:\n")
				hasWorktrees = true
			}
			fmt.Printf("  %s: %s (branch: %s)\n", m.AgentName, m.WorktreePath, m.BranchName)
		}
	}

	// Don't remove state on stop — preserve for --resume
	fmt.Printf("[team:%s] Stopped. Use 'aw team start --resume %s' to resume.\n", teamName, teamName)
	return nil
}

// Run handles team status.
func (t *TeamStatusCmd) Run() error {
	if t.TeamName != "" {
		return teamStatusOne(t.TeamName)
	}

	states, err := team.ListStates()
	if err != nil {
		return err
	}

	if len(states) == 0 {
		fmt.Println("No running teams")
		return nil
	}

	for _, s := range states {
		sid := s.SessionID
		if len(sid) > 12 {
			sid = sid[:12]
		}
		fmt.Printf("Team: %s (session: %s, started: %s)\n", s.Name, sid, s.StartedAt)
		for _, m := range s.Members {
			fg := ""
			if m.Foreground {
				fg = " [fg]"
			}
			fmt.Printf("  %-20s %-10s %-10s %s%s\n", m.AgentName, m.Profile, m.Role, m.Status, fg)
		}
		fmt.Println()
	}
	return nil
}

// Run handles team scope.
func (t *TeamScopeCmd) Run() error {
	state, err := team.LoadState(t.TeamName)
	if err != nil {
		return fmt.Errorf("team %q has no saved state", t.TeamName)
	}
	fmt.Println(state.TeamScope)
	return nil
}

func teamStatusOne(teamName string) error {
	state, err := team.LoadState(teamName)
	if err != nil {
		return fmt.Errorf("team %q is not running", teamName)
	}

	sid := state.SessionID
	if len(sid) > 12 {
		sid = sid[:12]
	}
	fmt.Printf("Team: %s (session: %s, started: %s)\n", state.Name, sid, state.StartedAt)
	for _, m := range state.Members {
		fg := ""
		if m.Foreground {
			fg = " [fg]"
		}
		fmt.Printf("  %-20s %-10s %-10s %s%s\n", m.AgentName, m.Profile, m.Role, m.Status, fg)
	}
	return nil
}
