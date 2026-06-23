package team

import (
	"path/filepath"
	"testing"
)

func overrideStateDir(t *testing.T) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "aw", "teams")
	old := stateDir
	stateDir = func() string { return dir }
	t.Cleanup(func() { stateDir = old })
}

func TestSaveLoadState(t *testing.T) {
	overrideStateDir(t)

	state := TeamState{
		Name:        "test-team",
		SessionID:   "abc-123",
		ProjectHash: "deadbeef1234",
		TeamScope:   "test-team-deadbeef1234-abc-123",
		StartedAt:   "2026-01-01T00:00:00Z",
		Members: []MemberState{
			{
				AgentName:     "developer-1",
				Profile:       "claude-dev",
				Role:          "developer",
				ContainerName: "aw-test-1",
				Runtime:       "podman",
				ToolSessionID: "sess-001",
				Foreground:    true,
				Status:        "running",
			},
			{
				AgentName:     "reviewer-1",
				Profile:       "cursor-review",
				Role:          "reviewer",
				ContainerName: "aw-test-2",
				Runtime:       "podman",
				ToolSessionID: "sess-002",
				Foreground:    false,
				Status:        "running",
			},
		},
	}

	if err := SaveState(state); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	loaded, err := LoadState("test-team")
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}

	if loaded.SessionID != state.SessionID {
		t.Errorf("SessionID = %q, want %q", loaded.SessionID, state.SessionID)
	}
	if loaded.ProjectHash != state.ProjectHash {
		t.Errorf("ProjectHash = %q, want %q", loaded.ProjectHash, state.ProjectHash)
	}
	if loaded.TeamScope != state.TeamScope {
		t.Errorf("TeamScope = %q, want %q", loaded.TeamScope, state.TeamScope)
	}
	if len(loaded.Members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(loaded.Members))
	}
	if loaded.Members[0].ToolSessionID != "sess-001" {
		t.Errorf("Members[0].ToolSessionID = %q, want %q", loaded.Members[0].ToolSessionID, "sess-001")
	}
	if loaded.Members[1].ToolSessionID != "sess-002" {
		t.Errorf("Members[1].ToolSessionID = %q, want %q", loaded.Members[1].ToolSessionID, "sess-002")
	}
}

func TestListStates(t *testing.T) {
	overrideStateDir(t)

	s1 := TeamState{Name: "team-a", SessionID: "id-a", StartedAt: "2026-01-01T00:00:00Z"}
	s2 := TeamState{Name: "team-b", SessionID: "id-b", StartedAt: "2026-01-02T00:00:00Z"}
	_ = SaveState(s1)
	_ = SaveState(s2)

	states, err := ListStates()
	if err != nil {
		t.Fatalf("ListStates: %v", err)
	}
	if len(states) != 2 {
		t.Fatalf("expected 2 states, got %d", len(states))
	}
}

func TestRemoveState(t *testing.T) {
	overrideStateDir(t)

	state := TeamState{Name: "rm-test", SessionID: "id-rm", StartedAt: "2026-01-01T00:00:00Z"}
	_ = SaveState(state)

	if err := RemoveState("rm-test"); err != nil {
		t.Fatalf("RemoveState: %v", err)
	}

	statePath := filepath.Join(StateDir(), "rm-test.state.json")
	if _, err := LoadState("rm-test"); err == nil {
		t.Fatalf("expected LoadState to fail after removal, file still exists at %s", statePath)
	}
}
