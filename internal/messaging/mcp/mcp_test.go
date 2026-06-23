package mcp

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/konono/aw/internal/messaging"
)

const testTeam = "test-team-abc123"

func testDB(t *testing.T) string {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "messages.db")
	store, err := messaging.OpenStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	_ = store.Close()
	return dbPath
}

func callTool(t *testing.T, srv *Server, dbPath, name string, args interface{}) json.RawMessage {
	t.Helper()
	argsJSON, _ := json.Marshal(args)
	store, err := messaging.OpenStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	result, err := srv.callTool(store, name, argsJSON)
	if err != nil {
		t.Fatalf("callTool(%s): %v", name, err)
	}
	data, _ := json.Marshal(result)
	return data
}

func TestSendAndReadInbox(t *testing.T) {
	dbPath := testDB(t)
	srv := NewServer(dbPath, "alice", testTeam)

	// Send a message
	result := callTool(t, srv, dbPath, "send_message", map[string]string{
		"to":   "bob",
		"body": "hello bob",
	})

	var sendResult map[string]interface{}
	if err := json.Unmarshal(result, &sendResult); err != nil {
		t.Fatal(err)
	}
	if sendResult["id"] == nil {
		t.Fatal("send_message should return id")
	}

	// Read inbox as bob
	bobSrv := NewServer(dbPath, "bob", testTeam)
	result = callTool(t, bobSrv, dbPath, "read_inbox", nil)

	var inbox []map[string]interface{}
	if err := json.Unmarshal(result, &inbox); err != nil {
		t.Fatal(err)
	}
	if len(inbox) != 1 {
		t.Fatalf("expected 1 inbox item, got %d", len(inbox))
	}
	if inbox[0]["from"] != "alice" {
		t.Errorf("from = %v, want alice", inbox[0]["from"])
	}
}

func TestReadMessageNoAutoMark(t *testing.T) {
	dbPath := testDB(t)
	srv := NewServer(dbPath, "alice", testTeam)

	callTool(t, srv, dbPath, "send_message", map[string]string{
		"to":   "bob",
		"body": "test",
	})

	bobSrv := NewServer(dbPath, "bob", testTeam)

	// read_message should not mark as read
	callTool(t, bobSrv, dbPath, "read_message", map[string]float64{"id": 1})

	// inbox should still show the message
	result := callTool(t, bobSrv, dbPath, "read_inbox", nil)
	var inbox []map[string]interface{}
	_ = json.Unmarshal(result, &inbox)
	if len(inbox) != 1 {
		t.Fatalf("read_message should not auto-mark; inbox has %d items, want 1", len(inbox))
	}

	// explicit mark_read
	callTool(t, bobSrv, dbPath, "mark_read", map[string]float64{"id": 1})

	result = callTool(t, bobSrv, dbPath, "read_inbox", nil)
	_ = json.Unmarshal(result, &inbox)
	if len(inbox) != 0 {
		t.Fatalf("after mark_read, inbox should be empty, got %d", len(inbox))
	}
}

func TestListAgentsTeamScoped(t *testing.T) {
	dbPath := testDB(t)

	// Messages in team A
	srvA := NewServer(dbPath, "alice", "team-a")
	callTool(t, srvA, dbPath, "send_message", map[string]string{"to": "bob", "body": "hi"})

	// Messages in team B
	srvB := NewServer(dbPath, "charlie", "team-b")
	callTool(t, srvB, dbPath, "send_message", map[string]string{"to": "dave", "body": "hi"})

	// list_agents in team A should only show alice and bob
	result := callTool(t, srvA, dbPath, "list_agents", nil)
	var agents []map[string]interface{}
	_ = json.Unmarshal(result, &agents)
	if len(agents) != 2 {
		t.Fatalf("team-a should have 2 agents, got %d", len(agents))
	}
	for _, a := range agents {
		name := a["name"].(string)
		if name != "alice" && name != "bob" {
			t.Errorf("unexpected agent %q in team-a", name)
		}
	}
}

func TestPerRequestDBConnection(t *testing.T) {
	dbPath := testDB(t)
	srv := NewServer(dbPath, "writer", testTeam)

	// Send via MCP server
	callTool(t, srv, dbPath, "send_message", map[string]string{"to": "reader", "body": "msg1"})

	// Write directly to DB from another "process" (simulated)
	store, err := messaging.OpenStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = store.Send(testTeam, "external", "reader", "msg2")
	if err != nil {
		t.Fatal(err)
	}
	_ = store.Close()

	// MCP server should see both messages immediately (per-request open)
	readerSrv := NewServer(dbPath, "reader", testTeam)
	result := callTool(t, readerSrv, dbPath, "read_inbox", nil)
	var inbox []map[string]interface{}
	_ = json.Unmarshal(result, &inbox)
	if len(inbox) != 2 {
		t.Fatalf("expected 2 unread (per-request DB open), got %d", len(inbox))
	}
}
