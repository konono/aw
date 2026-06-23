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
	"github.com/konono/aw/internal/docker"
	"github.com/konono/aw/internal/launcher"
	"github.com/konono/aw/internal/linuxbin"
	"github.com/konono/aw/internal/messaging"
	"github.com/konono/aw/internal/messaging/inject"
	"github.com/konono/aw/internal/messaging/roles"
	"github.com/konono/aw/internal/pipeline"
	"github.com/konono/aw/internal/platform"
	"github.com/konono/aw/internal/profile"
	"github.com/konono/aw/internal/reaper"
	"github.com/konono/aw/internal/stage"
	"github.com/konono/aw/internal/team"
)

func runTeam(args []string) int {
	if len(args) == 0 {
		printTeamHelp()
		return 1
	}

	switch args[0] {
	case "start":
		return runTeamStart(args[1:])
	case "stop":
		return runTeamStop(args[1:])
	case "status":
		return runTeamStatus(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "Unknown team command: %s\n", args[0])
		printTeamHelp()
		return 1
	}
}

func printTeamHelp() {
	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprintln(os.Stderr, "  aw team start [--resume] [--task <desc>] <team-name>  Start all team members")
	fmt.Fprintln(os.Stderr, "  aw team stop <team-name>               Stop all team members")
	fmt.Fprintln(os.Stderr, "  aw team status [team-name]             Show team status")
}

func projectHash(dir string) string {
	h := sha256.Sum256([]byte(dir))
	return fmt.Sprintf("%x", h)[:12]
}

func teamScope(teamName, projHash, sessionID string) string {
	return fmt.Sprintf("%s-%s-%s", teamName, projHash, sessionID[:12])
}

func runTeamStart(args []string) int {
	var resume bool
	var teamName string
	var task string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--resume":
			resume = true
		case "--task":
			if i+1 < len(args) {
				task = args[i+1]
				i++
			}
		default:
			if teamName == "" {
				teamName = args[i]
			}
		}
	}
	if teamName == "" {
		fmt.Fprintln(os.Stderr, "Usage: aw team start [--resume] [--task <description>] <team-name>")
		return 1
	}

	if platform.IsRunningAsRoot() {
		fmt.Fprintln(os.Stderr, "Error: aw must not be run as root")
		return 1
	}

	cfg, err := profile.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		return 1
	}
	if err := profile.ValidateConfig(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	t, ok := cfg.Teams[teamName]
	if !ok {
		fmt.Fprintf(os.Stderr, "Error: team %q not found\n", teamName)
		if len(cfg.Teams) > 0 {
			fmt.Fprintln(os.Stderr, "Available teams:")
			for name := range cfg.Teams {
				fmt.Fprintf(os.Stderr, "  %s\n", name)
			}
		}
		return 1
	}

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	projHash := projectHash(cwd)

	// Session ID: resume from previous state or generate new
	var sessionID string
	var prevState *team.TeamState
	if resume {
		prevState, err = team.LoadState(teamName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: no previous session to resume: %v\n", err)
			return 1
		}
		sessionID = prevState.SessionID
		fmt.Fprintf(os.Stderr, "[team:%s] Resuming session %s\n", teamName, sessionID[:12])
	} else {
		sessionID = uuid.New().String()
	}

	scope := teamScope(teamName, projHash, sessionID)

	mgr := team.NewManager()
	members := mgr.ResolveMembers(teamName, t)

	// Initialize messaging DB
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	msgDBDir := filepath.Join(homeDir, ".config", "aw", "messaging")
	msgDBPath := filepath.Join(msgDBDir, "messages.db")
	store, err := messaging.OpenStore(msgDBPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing messaging DB: %v\n", err)
		return 1
	}
	_ = store.Close()

	// Build member info for role templates
	memberInfos := make([]roles.MemberData, len(members))
	injectMembers := make([]inject.MemberInfo, len(members))
	for i, m := range members {
		memberInfos[i] = roles.MemberData{AgentName: m.AgentName, Role: m.Role}
		injectMembers[i] = inject.MemberInfo{AgentName: m.AgentName, Role: m.Role}
	}

	// Separate foreground and background members
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

	// Build previous tool session ID lookup for --resume
	prevToolSessions := map[string]string{}
	if prevState != nil {
		for _, m := range prevState.Members {
			prevToolSessions[m.AgentName] = m.ToolSessionID
		}
	}

	teamState := team.TeamState{
		Name:        teamName,
		SessionID:   sessionID,
		ProjectHash: projHash,
		TeamScope:   scope,
		StartedAt:   time.Now().UTC().Format(time.RFC3339),
	}

	ctx := context.Background()
	var bgReaperFds []*os.File

	// Detect git root for branch isolation
	var repoRoot string
	if out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output(); err == nil {
		repoRoot = strings.TrimSpace(string(out))
	}

	launchOpts := teamLaunchOpts{
		cfg:             cfg,
		scope:           scope,
		msgDBDir:        msgDBDir,
		memberInfos:     memberInfos,
		injectMembers:   injectMembers,
		resume:          resume,
		prevSessions:    prevToolSessions,
		task:            task,
		teamName:        teamName,
		branchIsolation: repoRoot != "",
		repoRoot:        repoRoot,
	}

	// Launch background members first
	for _, m := range bgMembers {
		containerName := fmt.Sprintf("aw-%s-%s-%d", teamName, m.AgentName, time.Now().UnixNano())
		ms, reaperFd, err := launchTeamMember(ctx, launchOpts, m, containerName, false)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error starting %s: %v\n", m.AgentName, err)
			closeReaperFds(bgReaperFds)
			stopTeamContainers(teamState)
			return 1
		}
		if reaperFd != nil {
			bgReaperFds = append(bgReaperFds, reaperFd)
		}
		teamState.Members = append(teamState.Members, ms)
		fmt.Fprintf(os.Stderr, "[team:%s] Started %s (%s) in background\n", teamName, m.AgentName, m.Profile)
	}

	// Save state before launching foreground (which won't return)
	fgContainerName := fmt.Sprintf("aw-%s-%s-%d", teamName, fgMember.AgentName, time.Now().UnixNano())
	fgRuntime := "docker"
	if fgProfile, ok := cfg.Profiles[fgMember.Profile]; ok {
		fgRuntime = fgProfile.EffectiveContainerRuntime()
	}
	fgToolSessionID := uuid.New().String()
	if prev, ok := prevToolSessions[fgMember.AgentName]; ok && prev != "" {
		fgToolSessionID = prev
	}
	fgState := team.MemberState{
		AgentName:     fgMember.AgentName,
		Profile:       fgMember.Profile,
		Role:          fgMember.Role,
		ContainerName: fgContainerName,
		Runtime:       fgRuntime,
		ToolSessionID: fgToolSessionID,
		Foreground:    true,
		Status:        "running",
	}
	if launchOpts.branchIsolation {
		fgState.BranchName = fmt.Sprintf("aw/%s/%s", teamName, fgMember.AgentName)
		fgState.WorktreePath = filepath.Join(launchOpts.repoRoot, "worktrees", fmt.Sprintf("aw-%s-%s", teamName, fgMember.AgentName))
	}
	teamState.Members = append(teamState.Members, fgState)
	if err := team.SaveState(teamState); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not save team state: %v\n", err)
	}

	fmt.Fprintf(os.Stderr, "[team:%s] Starting %s (%s) in foreground...\n", teamName, fgMember.AgentName, fgMember.Profile)

	if _, _, err := launchTeamMember(ctx, launchOpts, *fgMember, fgContainerName, true); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting %s: %v\n", fgMember.AgentName, err)
		closeReaperFds(bgReaperFds)
		stopTeamContainers(teamState)
		return 1
	}

	return 0
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

	// Branch isolation: create per-member worktree
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

	deliveryMode := inject.DeliveryMode(p.EffectiveDelivery(tool))

	toolStageDir := filepath.Join(ec.HomeDir, ".agent-workspace", tool)
	injector, err := inject.ForTool(tool)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: messaging injection not supported for %s: %v\n", tool, err)
	} else {
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

	// Inject role context
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
	}

	// Mount aw binary
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

	// Tool session ID for resume support
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

	runConfig := pipeline.ToolRunConfig(ec, runtime, tool, command)
	runConfig.EnvVars["AW_AGENT_NAME"] = m.AgentName
	runConfig.EnvVars["AW_MSG_DB"] = "/home/agent/.aw-msg/messages.db"
	runConfig.EnvVars["AW_TEAM_NAME"] = opts.scope
	runConfig.EnvVars["AW_SESSION_ID"] = toolSessionID

	var reviewers []string
	for _, mi := range opts.injectMembers {
		if mi.Role == "reviewer" && mi.AgentName != m.AgentName {
			reviewers = append(reviewers, mi.AgentName)
		}
	}
	if len(reviewers) > 0 {
		runConfig.EnvVars["AW_REVIEWERS"] = strings.Join(reviewers, ",")
	}

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
		client := docker.NewShellClient(runtime)
		_ = client.StopContainer(m.ContainerName)
	}
	_ = team.RemoveState(state.Name)
}

func runTeamStop(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: aw team stop <team-name>")
		return 1
	}
	teamName := args[0]

	state, err := team.LoadState(teamName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: team %q is not running: %v\n", teamName, err)
		return 1
	}

	fmt.Printf("[team:%s] Stopping %d members...\n", teamName, len(state.Members))
	for _, m := range state.Members {
		runtime := m.Runtime
		if runtime == "" {
			runtime = "docker"
		}
		client := docker.NewShellClient(runtime)
		fmt.Printf("  Stopping %s (%s)...\n", m.AgentName, m.ContainerName)
		if err := client.StopContainer(m.ContainerName); err != nil {
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
	return 0
}

func runTeamStatus(args []string) int {
	if len(args) > 0 {
		return runTeamStatusOne(args[0])
	}

	states, err := team.ListStates()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	if len(states) == 0 {
		fmt.Println("No running teams")
		return 0
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
	return 0
}

func runTeamStatusOne(teamName string) int {
	state, err := team.LoadState(teamName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Team %q is not running\n", teamName)
		return 1
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
	return 0
}
