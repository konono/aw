package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/konono/aw/v4/internal/docker"
	"github.com/konono/aw/v4/internal/platform"
	"github.com/konono/aw/v4/internal/team"
)

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
		task:            t.Task,
		teamName:        teamName,
		repoRoot:        sess.repoRoot,
	}

	for _, m := range sess.bgMembers {
		ms, reaperFd, err := launchTeamMember(ctx, launchOpts, m, teamContainerName(teamName, m.AgentName), resolveToolSessionID(sess.prevSessions, m.AgentName), false)
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

	fgContainerName := teamContainerName(teamName, sess.fgMember.AgentName)
	fgRuntime := "docker"
	if fgProfile, ok := cfg.Profiles[sess.fgMember.Profile]; ok {
		fgRuntime = fgProfile.EffectiveContainerRuntime()
	}
	fgToolSessionID := resolveToolSessionID(sess.prevSessions, sess.fgMember.AgentName)
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
	if launchOpts.repoRoot != "" {
		fgState.BranchName = teamBranchName(teamName, sess.fgMember.AgentName)
		fgState.WorktreePath = teamWorktreePath(launchOpts.repoRoot, teamName, sess.fgMember.AgentName)
	}
	teamState.Members = append(teamState.Members, fgState)
	if err := team.SaveState(teamState); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not save team state: %v\n", err)
	}

	fmt.Fprintf(os.Stderr, "[team:%s] Starting %s (%s) in foreground...\n", teamName, sess.fgMember.AgentName, sess.fgMember.Profile)

	if _, _, err := launchTeamMember(ctx, launchOpts, *sess.fgMember, fgContainerName, fgToolSessionID, true); err != nil {
		closeReaperFds(bgReaperFds)
		stopTeamContainers(teamState)
		return fmt.Errorf("starting %s: %w", sess.fgMember.AgentName, err)
	}

	return nil
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
		rt := docker.NewShellClient(m.EffectiveRuntime())
		fmt.Printf("  Stopping %s (%s)...\n", m.AgentName, m.ContainerName)
		if err := stopContainerQuiet(rt.DockerCmd(), m.ContainerName); err != nil {
			fmt.Fprintf(os.Stderr, "  Warning: %v\n", err)
		}
	}

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

	for i, s := range states {
		printTeamState(s)
		if i < len(states)-1 {
			fmt.Println()
		}
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
	printTeamState(*state)
	return nil
}

func printTeamState(s team.TeamState) {
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

func stopTeamContainers(state team.TeamState) {
	for _, m := range state.Members {
		_ = stopContainerQuiet(docker.NewShellClient(m.EffectiveRuntime()).DockerCmd(), m.ContainerName)
	}
	_ = team.RemoveState(state.Name)
}
