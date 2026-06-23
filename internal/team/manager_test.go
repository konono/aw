package team

import (
	"testing"

	"github.com/konono/aw/internal/profile"
)

func TestResolveMembers(t *testing.T) {
	mgr := NewManager()
	tm := profile.Team{
		Members: []profile.TeamMember{
			{Profile: "claude-dev", Role: "developer", Foreground: true},
			{Profile: "cursor-review", Role: "reviewer"},
			{Profile: "claude-dev2", Role: "developer"},
		},
	}

	members := mgr.ResolveMembers("test-team", tm)

	if len(members) != 3 {
		t.Fatalf("expected 3 members, got %d", len(members))
	}

	expected := []struct {
		agentName  string
		profile    string
		role       string
		foreground bool
	}{
		{"developer-1", "claude-dev", "developer", true},
		{"reviewer-1", "cursor-review", "reviewer", false},
		{"developer-2", "claude-dev2", "developer", false},
	}

	for i, e := range expected {
		if members[i].AgentName != e.agentName {
			t.Errorf("member[%d].AgentName = %q, want %q", i, members[i].AgentName, e.agentName)
		}
		if members[i].Profile != e.profile {
			t.Errorf("member[%d].Profile = %q, want %q", i, members[i].Profile, e.profile)
		}
		if members[i].Role != e.role {
			t.Errorf("member[%d].Role = %q, want %q", i, members[i].Role, e.role)
		}
		if members[i].Foreground != e.foreground {
			t.Errorf("member[%d].Foreground = %v, want %v", i, members[i].Foreground, e.foreground)
		}
	}
}
