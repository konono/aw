package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/konono/aw/internal/docker"
	"github.com/konono/aw/internal/launcher"
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
	fmt.Fprintln(os.Stderr, "  aw team start <team-name>   Start all team members")
	fmt.Fprintln(os.Stderr, "  aw team stop <team-name>    Stop all team members")
	fmt.Fprintln(os.Stderr, "  aw team status [team-name]  Show team status")
}

func runTeamStart(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: aw team start <team-name>")
		return 1
	}
	teamName := args[0]

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

	teamState := team.TeamState{
		Name:      teamName,
		StartedAt: time.Now().UTC().Format(time.RFC3339),
	}

	ctx := context.Background()

	var bgReaperFds []*os.File

	// Launch background members first
	for _, m := range bgMembers {
		containerName := fmt.Sprintf("aw-%s-%s-%d", teamName, m.AgentName, time.Now().UnixNano())
		ms, reaperFd, err := launchTeamMember(ctx, cfg, teamName, m, containerName, msgDBDir, memberInfos, injectMembers, false)
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
	teamState.Members = append(teamState.Members, team.MemberState{
		AgentName:     fgMember.AgentName,
		Profile:       fgMember.Profile,
		Role:          fgMember.Role,
		ContainerName: fgContainerName,
		Runtime:       fgRuntime,
		Foreground:    true,
		Status:        "running",
	})
	if err := team.SaveState(teamState); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not save team state: %v\n", err)
	}

	fmt.Fprintf(os.Stderr, "[team:%s] Starting %s (%s) in foreground...\n", teamName, fgMember.AgentName, fgMember.Profile)

	// Launch foreground member (this calls os.Exit and does not return).
	// Background reaper fds are inherited by the exec'd process and closed
	// when it exits, which correctly signals the reapers.
	if _, _, err := launchTeamMember(ctx, cfg, teamName, *fgMember, fgContainerName, msgDBDir, memberInfos, injectMembers, true); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting %s: %v\n", fgMember.AgentName, err)
		closeReaperFds(bgReaperFds)
		stopTeamContainers(teamState)
		return 1
	}

	return 0
}

func closeReaperFds(fds []*os.File) {
	for _, f := range fds {
		_ = f.Close()
	}
}

func launchTeamMember(
	ctx context.Context,
	cfg *profile.Config,
	teamName string,
	m team.ResolvedMember,
	containerName string,
	msgDBDir string,
	memberInfos []roles.MemberData,
	injectMembers []inject.MemberInfo,
	foreground bool,
) (team.MemberState, *os.File, error) {
	p, ok := cfg.Profiles[m.Profile]
	if !ok {
		return team.MemberState{}, nil, fmt.Errorf("profile %q not found", m.Profile)
	}

	ec, err := buildExecutionContext(m.Profile, p)
	if err != nil {
		return team.MemberState{}, nil, fmt.Errorf("building execution context: %w", err)
	}
	ec.ContainerName = containerName

	// Run DockerStage to build image, sync config, setup mounts
	dockerStage := stage.NewDockerStage()
	if err := dockerStage.Run(ctx, ec); err != nil {
		return team.MemberState{}, nil, fmt.Errorf("docker stage: %w", err)
	}

	// Run EnvStage to load custom env vars
	envStage := &stage.EnvStage{}
	if err := envStage.Run(ctx, ec); err != nil {
		return team.MemberState{}, nil, fmt.Errorf("env stage: %w", err)
	}

	tool := p.EffectiveTool()

	// Inject messaging MCP + hook
	toolStageDir := filepath.Join(ec.HomeDir, ".agent-workspace", tool)
	injector, err := inject.ForTool(tool)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: messaging injection not supported for %s: %v\n", tool, err)
	} else {
		injectCfg := inject.InjectorConfig{
			AgentName:  m.AgentName,
			TeamName:   teamName,
			Role:       m.Role,
			Members:    injectMembers,
			StagingDir: toolStageDir,
			WorkDir:    ec.WorkDir,
			MCPBinary:  "/home/agent/.aw-msg/bin/aw",
			DBPath:     "/home/agent/.aw-msg/messages.db",
		}
		if ierr := injector.InjectMCP(injectCfg); ierr != nil {
			fmt.Fprintf(os.Stderr, "Warning: MCP injection: %v\n", ierr)
		}
		if ierr := injector.InjectHook(injectCfg); ierr != nil {
			fmt.Fprintf(os.Stderr, "Warning: hook injection: %v\n", ierr)
		}
	}

	// Inject role context into instruction file
	roleData := roles.TemplateData{
		TeamName:  teamName,
		AgentName: m.AgentName,
	}
	for _, mi := range memberInfos {
		roleData.Members = append(roleData.Members, roles.MemberData{
			AgentName: mi.AgentName,
			Role:      mi.Role,
			IsSelf:    mi.AgentName == m.AgentName,
		})
	}
	if roleText, rerr := roles.Render(m.Role, roleData); rerr == nil {
		appendRoleContext(toolStageDir, tool, roleText)
	}

	// Mount aw binary into container for MCP server and check-inbox
	awBin, err := os.Executable()
	if err != nil {
		return team.MemberState{}, nil, fmt.Errorf("resolving aw binary path: %w", err)
	}
	awBin, err = filepath.EvalSymlinks(awBin)
	if err != nil {
		return team.MemberState{}, nil, fmt.Errorf("resolving aw binary symlink: %w", err)
	}

	ec.DockerMounts = append(ec.DockerMounts,
		docker.Mount{
			Source:   msgDBDir,
			Target:   "/home/agent/.aw-msg",
			ReadOnly: false,
		},
		docker.Mount{
			Source:   awBin,
			Target:   "/home/agent/.aw-msg/bin/aw",
			ReadOnly: true,
		},
	)

	// Add messaging env vars
	runtime := p.EffectiveContainerRuntime()
	command := ec.CommandOverride
	if len(command) == 0 {
		command = launcher.ToolContainerCommand(tool)
		if command == nil {
			command = []string{tool}
		}
	}

	runConfig := pipeline.ToolRunConfig(ec, runtime, tool, command)
	runConfig.EnvVars["AW_AGENT_NAME"] = m.AgentName
	runConfig.EnvVars["AW_MSG_DB"] = "/home/agent/.aw-msg/messages.db"
	runConfig.EnvVars["AW_TEAM_NAME"] = teamName

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
		Foreground:    false,
		Status:        "running",
	}, reaperFd, nil
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

	if err := team.RemoveState(teamName); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not remove state file: %v\n", err)
	}

	fmt.Printf("[team:%s] Stopped.\n", teamName)
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
		fmt.Printf("Team: %s (started: %s)\n", s.Name, s.StartedAt)
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

	fmt.Printf("Team: %s (started: %s)\n", state.Name, state.StartedAt)
	for _, m := range state.Members {
		fg := ""
		if m.Foreground {
			fg = " [fg]"
		}
		fmt.Printf("  %-20s %-10s %-10s %s%s\n", m.AgentName, m.Profile, m.Role, m.Status, fg)
	}
	return 0
}
