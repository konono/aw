package team

import "github.com/konono/aw/internal/profile"

// ResolvedMember is a team member with its assigned agent name ready for launch.
type ResolvedMember struct {
	AgentName  string
	Profile    string
	Role       string
	Foreground bool
}

// Manager coordinates team lifecycle: naming, state, and (future) container launch.
type Manager struct{}

// NewManager creates a new team Manager.
func NewManager() *Manager {
	return &Manager{}
}

// ResolveMembers assigns agent names to team members based on their roles.
func (m *Manager) ResolveMembers(teamName string, team profile.Team) []ResolvedMember {
	roles := make([]MemberRole, len(team.Members))
	for i, tm := range team.Members {
		roles[i] = MemberRole{Role: string(tm.Role)}
	}
	names := AssignNames(roles)

	resolved := make([]ResolvedMember, len(team.Members))
	for i, tm := range team.Members {
		resolved[i] = ResolvedMember{
			AgentName:  names[i],
			Profile:    tm.Profile,
			Role:       string(tm.Role),
			Foreground: tm.Foreground,
		}
	}
	return resolved
}
