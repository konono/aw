package messaging

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

const testTeam = "test-team"

func testStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := OpenStore(filepath.Join(dir, "messages.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestSendAndReadInbox(t *testing.T) {
	s := testStore(t)

	id, _, err := s.Send(testTeam, "alice", "bob", "hello bob")
	if err != nil {
		t.Fatal(err)
	}
	if id != 1 {
		t.Fatalf("expected id 1, got %d", id)
	}

	previews, err := s.ReadInbox(testTeam, "bob")
	if err != nil {
		t.Fatal(err)
	}
	if len(previews) != 1 {
		t.Fatalf("expected 1 preview, got %d", len(previews))
	}
	if previews[0].From != "alice" || previews[0].Preview != "hello bob" {
		t.Fatalf("unexpected preview: %+v", previews[0])
	}

	// alice should have empty inbox
	previews, err = s.ReadInbox(testTeam, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if len(previews) != 0 {
		t.Fatalf("expected 0 previews for alice, got %d", len(previews))
	}
}

func TestReadMessageDoesNotAutoMark(t *testing.T) {
	s := testStore(t)

	id, _, err := s.Send(testTeam, "alice", "bob", "test message")
	if err != nil {
		t.Fatal(err)
	}

	msg, err := s.ReadMessage(id)
	if err != nil {
		t.Fatal(err)
	}
	if msg.Body != "test message" {
		t.Fatalf("unexpected body: %q", msg.Body)
	}
	if msg.ReadAt != nil {
		t.Fatal("ReadMessage should not auto-mark as read")
	}

	count, err := s.UnreadCount(testTeam, "bob")
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected 1 unread after ReadMessage, got %d", count)
	}

	// Explicitly mark read
	if err := s.MarkRead(id); err != nil {
		t.Fatal(err)
	}
	count, err = s.UnreadCount(testTeam, "bob")
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("expected 0 unread after MarkRead, got %d", count)
	}
}

func TestListAgents(t *testing.T) {
	s := testStore(t)

	_, _, _ = s.Send(testTeam, "alice", "bob", "msg1")
	_, _, _ = s.Send(testTeam, "bob", "charlie", "msg2")

	agents, err := s.ListAgents(testTeam)
	if err != nil {
		t.Fatal(err)
	}
	if len(agents) != 3 {
		t.Fatalf("expected 3 agents, got %d", len(agents))
	}
	names := make(map[string]bool)
	for _, a := range agents {
		names[a.Name] = true
	}
	for _, name := range []string{"alice", "bob", "charlie"} {
		if !names[name] {
			t.Fatalf("expected agent %q in list", name)
		}
	}
}

func TestListAgentsTeamIsolation(t *testing.T) {
	s := testStore(t)

	_, _, _ = s.Send("team-a", "alice", "bob", "msg1")
	_, _, _ = s.Send("team-b", "charlie", "dave", "msg2")

	agents, err := s.ListAgents("team-a")
	if err != nil {
		t.Fatal(err)
	}
	if len(agents) != 2 {
		t.Fatalf("expected 2 agents in team-a, got %d", len(agents))
	}
	for _, a := range agents {
		if a.Name != "alice" && a.Name != "bob" {
			t.Fatalf("unexpected agent %q in team-a", a.Name)
		}
	}
}

func TestHistory(t *testing.T) {
	s := testStore(t)

	_, _, _ = s.Send(testTeam, "alice", "bob", "first")
	_, _, _ = s.Send(testTeam, "bob", "alice", "second")
	_, _, _ = s.Send(testTeam, "charlie", "bob", "third")

	// All history for team
	msgs, err := s.History(testTeam, "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(msgs))
	}

	// Filtered by alice
	msgs, err = s.History(testTeam, "alice", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages for alice, got %d", len(msgs))
	}
}

func TestPreviewTruncation(t *testing.T) {
	s := testStore(t)

	longBody := make([]byte, 500)
	for i := range longBody {
		longBody[i] = 'x'
	}
	_, _, _ = s.Send(testTeam, "alice", "bob", string(longBody))

	previews, err := s.ReadInbox(testTeam, "bob")
	if err != nil {
		t.Fatal(err)
	}
	if len(previews) != 1 {
		t.Fatal("expected 1 preview")
	}
	if len(previews[0].Preview) != previewLen+3 {
		t.Fatalf("expected preview length %d, got %d", previewLen+3, len(previews[0].Preview))
	}
}

func TestOpenStoreCreatesDir(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "sub", "dir", "messages.db")

	s, err := OpenStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	if _, err := os.Stat(filepath.Join(dir, "sub", "dir")); err != nil {
		t.Fatalf("directory not created: %v", err)
	}
}

func TestClearTeam(t *testing.T) {
	s := testStore(t)
	defer func() { _ = s.Close() }()

	_, _, _ = s.Send("team-a", "dev", "rev", "msg1")
	_, _, _ = s.Send("team-a", "dev", "rev", "msg2")
	_, _, _ = s.Send("team-b", "dev", "rev", "msg3")

	n, err := s.ClearTeam("team-a")
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("ClearTeam deleted %d, want 2", n)
	}

	msgs, _ := s.History("team-b", "", 10)
	if len(msgs) != 1 {
		t.Errorf("team-b should still have 1 message, got %d", len(msgs))
	}
}

func TestClearAll(t *testing.T) {
	s := testStore(t)
	defer func() { _ = s.Close() }()

	_, _, _ = s.Send("team-a", "dev", "rev", "msg1")
	_, _, _ = s.Send("team-b", "dev", "rev", "msg2")

	n, err := s.ClearAll()
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("ClearAll deleted %d, want 2", n)
	}
}

func TestClearBefore(t *testing.T) {
	s := testStore(t)
	defer func() { _ = s.Close() }()

	_, _, _ = s.Send(testTeam, "dev", "rev", "recent msg")

	// ClearBefore with 0 duration should delete nothing (all messages are "now")
	n, err := s.ClearBefore(0)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("ClearBefore(0) deleted %d, want 0", n)
	}

	// ClearBefore with future cutoff should delete everything
	n, err = s.ClearBefore(-24 * time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("ClearBefore(-24h) deleted %d, want 1", n)
	}
}

func TestListTeams(t *testing.T) {
	s := testStore(t)
	defer func() { _ = s.Close() }()

	_, _, _ = s.Send("team-b", "dev", "rev", "msg1")
	_, _, _ = s.Send("team-a", "dev", "rev", "msg2")
	_, _, _ = s.Send("team-a", "dev", "rev", "msg3")

	teams, err := s.ListTeams()
	if err != nil {
		t.Fatal(err)
	}
	if len(teams) != 2 {
		t.Fatalf("expected 2 teams, got %d", len(teams))
	}
	if teams[0] != "team-a" || teams[1] != "team-b" {
		t.Errorf("teams = %v, want [team-a, team-b]", teams)
	}
}
