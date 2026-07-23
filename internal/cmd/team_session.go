package cmd

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/konono/aw/v4/internal/docker"
	"github.com/konono/aw/v4/internal/messaging"
	"github.com/konono/aw/v4/internal/messaging/inject"
	"github.com/konono/aw/v4/internal/messaging/roles"
	"github.com/konono/aw/v4/internal/profile"
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
	fgMember        *team.ResolvedMember
	bgMembers       []team.ResolvedMember
	memberInfos     []roles.MemberData
	injectMembers   []inject.MemberInfo
	msgDBDir        string
	prevSessions    map[string]string
	repoRoot        string
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
			_ = stopContainerQuiet(docker.NewShellClient(m.EffectiveRuntime()).DockerCmd(), m.ContainerName)
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
		fgMember:        fgMember,
		bgMembers:       bgMembers,
		memberInfos:     memberInfos,
		injectMembers:   injectMembers,
		msgDBDir:        msgDBDir,
		prevSessions:    prevToolSessions,
		repoRoot:        repoRoot,
	}, nil
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
