package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/konono/aw/v4/internal/messaging"
	"github.com/konono/aw/v4/internal/messaging/inject"
	"github.com/konono/aw/v4/internal/messaging/roles"
)

// runCheckInbox is a test helper that runs InternalCheckInboxCmd.Run()
// with the given db/agent/team values (falls back to env vars).
func runCheckInbox(db, agent, teamName string) error {
	cmd := &InternalCheckInboxCmd{
		internalMsgFlags: internalMsgFlags{
			DB:    db,
			Agent: agent,
			Team:  teamName,
		},
	}
	return cmd.Run()
}

// ---------------------------------------------------------------------------
// Phase 1.5a: check-inbox JSON output — full flow with DB
// ---------------------------------------------------------------------------

func TestE2E_CheckInbox_JSONOutput(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "messages.db")

	store, err := messaging.OpenStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.Send("team-scope", "reviewer-1", "developer-1", "Please fix the bug"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.Send("team-scope", "reviewer-1", "developer-1", "Also add tests"); err != nil {
		t.Fatal(err)
	}
	_ = store.Close()

	// Set env for cooldown check
	t.Setenv("AW_MSG_CHECK_INTERVAL", "0")

	// Remove any stale marker
	_ = os.Remove(filepath.Join(dir, ".lastcheck-developer-1"))

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err = runCheckInbox(dbPath, "developer-1", "team-scope")

	_ = w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("error: %v", err)
	}

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	output := strings.TrimSpace(string(buf[:n]))

	// Verify it is valid JSON with decision: block
	var resp hookResponse
	if err := json.Unmarshal([]byte(output), &resp); err != nil {
		t.Fatalf("output is not valid JSON: %q\nerror: %v", output, err)
	}
	if resp.Decision != "block" {
		t.Errorf("decision = %q, want %q", resp.Decision, "block")
	}
	if !strings.Contains(resp.Reason, "2 unread") {
		t.Errorf("reason = %q, should contain '2 unread'", resp.Reason)
	}
}

func TestE2E_CheckInbox_NoMessages(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "messages.db")

	store, err := messaging.OpenStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	_ = store.Close()

	t.Setenv("AW_MSG_CHECK_INTERVAL", "0")

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err = runCheckInbox(dbPath, "developer-1", "team-scope")

	_ = w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("error: %v", err)
	}

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	output := strings.TrimSpace(string(buf[:n]))
	if output != "" {
		t.Errorf("expected empty output for no messages, got %q", output)
	}
}

func TestE2E_CheckInbox_Cooldown(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "messages.db")

	store, err := messaging.OpenStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.Send("team-scope", "reviewer-1", "developer-1", "msg"); err != nil {
		t.Fatal(err)
	}
	_ = store.Close()

	t.Setenv("AW_MSG_CHECK_INTERVAL", "9999")

	// First call should output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	err = runCheckInbox(dbPath, "developer-1", "team-scope")
	_ = w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("first call error: %v", err)
	}
	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	first := strings.TrimSpace(string(buf[:n]))
	if first == "" {
		t.Fatal("first call should produce output")
	}

	// Second call within cooldown should produce no output
	r, w, _ = os.Pipe()
	os.Stdout = w
	err = runCheckInbox(dbPath, "developer-1", "team-scope")
	_ = w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("second call error: %v", err)
	}
	buf = make([]byte, 4096)
	n, _ = r.Read(buf)
	second := strings.TrimSpace(string(buf[:n]))
	if second != "" {
		t.Errorf("second call within cooldown should produce no output, got %q", second)
	}
}

func TestE2E_CheckInbox_TeamIsolation(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "messages.db")

	store, err := messaging.OpenStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.Send("team-A", "other", "developer-1", "for team A"); err != nil {
		t.Fatal(err)
	}
	_ = store.Close()

	t.Setenv("AW_MSG_CHECK_INTERVAL", "0")

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	_ = runCheckInbox(dbPath, "developer-1", "team-B")
	_ = w.Close()
	os.Stdout = oldStdout

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	output := strings.TrimSpace(string(buf[:n]))
	if output != "" {
		t.Errorf("should not see messages from team-A when querying team-B, got %q", output)
	}
}

// ---------------------------------------------------------------------------
// Phase 2a: --task flag — role template integration
// ---------------------------------------------------------------------------

func TestE2E_TaskInAllRoleTemplates(t *testing.T) {
	roleNames := []string{"developer", "reviewer", "lead", "partner"}
	for _, role := range roleNames {
		t.Run(role, func(t *testing.T) {
			data := roles.TemplateData{
				TeamName:  "test-team-scope",
				AgentName: role + "-1",
				Members: []roles.MemberData{
					{AgentName: "developer-1", Role: "developer", IsSelf: role == "developer"},
					{AgentName: "reviewer-1", Role: "reviewer", IsSelf: role == "reviewer"},
				},
				Task: "Implement FizzBuzz with comprehensive tests",
			}

			out, err := roles.Render(role, data)
			if err != nil {
				t.Fatalf("Render(%q): %v", role, err)
			}

			if !strings.Contains(out, "### Task") {
				t.Error("missing ### Task section")
			}
			if !strings.Contains(out, "Implement FizzBuzz") {
				t.Error("missing task description")
			}
			if !strings.Contains(out, "test-team-scope") {
				t.Error("missing team name")
			}
			if !strings.Contains(out, role+"-1") {
				t.Error("missing agent name")
			}
			if !strings.Contains(out, "send_message") {
				t.Error("missing MCP tool documentation")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Phase 1.5b: Monitor mode — inject and verify settings.json structure
// ---------------------------------------------------------------------------

func TestE2E_MonitorMode_SettingsJSON(t *testing.T) {
	dir := t.TempDir()
	cfg := inject.InjectorConfig{
		AgentName:    "developer-1",
		TeamName:     "team-scope",
		Role:         "developer",
		StagingDir:   dir,
		WorkDir:      dir,
		MCPBinary:    "/home/agent/.aw-msg/bin/aw",
		DBPath:       "/home/agent/.aw-msg/messages.db",
		DeliveryMode: inject.DeliveryMonitor,
	}

	injector := &inject.ClaudeInjector{}
	if err := injector.InjectMCP(cfg); err != nil {
		t.Fatal(err)
	}
	if err := injector.InjectHook(cfg); err != nil {
		t.Fatal(err)
	}

	// Verify settings.json
	settingsData, err := os.ReadFile(filepath.Join(dir, "settings.json"))
	if err != nil {
		t.Fatalf("settings.json: %v", err)
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(settingsData, &settings); err != nil {
		t.Fatalf("settings.json parse: %v", err)
	}

	hooks, ok := settings["hooks"].(map[string]interface{})
	if !ok {
		t.Fatal("missing hooks in settings.json")
	}

	// Should have SessionStart, not Stop
	if _, ok := hooks["Stop"]; ok {
		t.Error("monitor mode should NOT have Stop hook")
	}
	startGroups, ok := hooks["SessionStart"].([]interface{})
	if !ok || len(startGroups) == 0 {
		t.Fatal("missing SessionStart hook")
	}

	// Verify .mcp.json
	mcpData, err := os.ReadFile(filepath.Join(dir, ".mcp.json"))
	if err != nil {
		t.Fatalf(".mcp.json: %v", err)
	}

	var mcpConfig map[string]interface{}
	if err := json.Unmarshal(mcpData, &mcpConfig); err != nil {
		t.Fatalf(".mcp.json parse: %v", err)
	}
	servers, _ := mcpConfig["mcpServers"].(map[string]interface{})
	if _, ok := servers["aw-msg"]; !ok {
		t.Error("missing aw-msg server in .mcp.json")
	}
}

func TestE2E_TurnMode_SettingsJSON(t *testing.T) {
	dir := t.TempDir()
	cfg := inject.InjectorConfig{
		AgentName:    "developer-1",
		TeamName:     "team-scope",
		Role:         "developer",
		StagingDir:   dir,
		WorkDir:      dir,
		MCPBinary:    "/home/agent/.aw-msg/bin/aw",
		DBPath:       "/home/agent/.aw-msg/messages.db",
		DeliveryMode: inject.DeliveryTurn,
	}

	injector := &inject.ClaudeInjector{}
	if err := injector.InjectHook(cfg); err != nil {
		t.Fatal(err)
	}

	settingsData, _ := os.ReadFile(filepath.Join(dir, "settings.json"))
	var settings map[string]interface{}
	_ = json.Unmarshal(settingsData, &settings)
	hooks := settings["hooks"].(map[string]interface{})

	// Should have Stop, not SessionStart
	if _, ok := hooks["SessionStart"]; ok {
		t.Error("turn mode should NOT have SessionStart hook")
	}
	stopGroups, ok := hooks["Stop"].([]interface{})
	if !ok || len(stopGroups) == 0 {
		t.Fatal("missing Stop hook")
	}

}

func TestE2E_OffMode_NoSettingsJSON(t *testing.T) {
	dir := t.TempDir()
	cfg := inject.InjectorConfig{
		AgentName:    "developer-1",
		TeamName:     "team-scope",
		Role:         "developer",
		StagingDir:   dir,
		WorkDir:      dir,
		MCPBinary:    "/home/agent/.aw-msg/bin/aw",
		DBPath:       "/home/agent/.aw-msg/messages.db",
		DeliveryMode: inject.DeliveryOff,
	}

	injector := &inject.ClaudeInjector{}
	if err := injector.InjectHook(cfg); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(dir, "settings.json")); err == nil {
		t.Error("off mode should not create settings.json")
	}
}

// ---------------------------------------------------------------------------
// Integration: Claude injector — all delivery modes x roles
// ---------------------------------------------------------------------------

func TestIntegration_ClaudeInjector_AllModes(t *testing.T) {
	tests := []struct {
		deliveryMode inject.DeliveryMode
		role         string
		wantStop     bool
		wantStart    bool
	}{
		{inject.DeliveryTurn, "developer", true, false},
		{inject.DeliveryTurn, "reviewer", true, false},
		{inject.DeliveryMonitor, "developer", false, true},
		{inject.DeliveryMonitor, "reviewer", false, true},
		{inject.DeliveryOff, "developer", false, false},
		{inject.DeliveryOff, "reviewer", false, false},
	}

	for _, tt := range tests {
		name := string(tt.deliveryMode) + "/" + tt.role
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			cfg := inject.InjectorConfig{
				AgentName:    tt.role + "-1",
				TeamName:     "team-scope",
				Role:         tt.role,
				StagingDir:   dir,
				WorkDir:      dir,
				MCPBinary:    "/usr/bin/aw",
				DBPath:       "/data/messages.db",
				DeliveryMode: tt.deliveryMode,
			}

			injector := &inject.ClaudeInjector{}
			if err := injector.InjectMCP(cfg); err != nil {
				t.Fatal(err)
			}
			if err := injector.InjectHook(cfg); err != nil {
				t.Fatal(err)
			}

			// Always verify .mcp.json
			mcpData, err := os.ReadFile(filepath.Join(dir, ".mcp.json"))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(mcpData), "aw-msg") {
				t.Error("missing aw-msg in .mcp.json")
			}

			// Check settings.json
			settingsPath := filepath.Join(dir, "settings.json")
			if tt.deliveryMode == inject.DeliveryOff {
				if _, err := os.Stat(settingsPath); err == nil {
					t.Error("off mode should not create settings.json")
				}
				return
			}

			settingsData, err := os.ReadFile(settingsPath)
			if err != nil {
				t.Fatalf("reading settings.json: %v", err)
			}

			var settings map[string]interface{}
			if err := json.Unmarshal(settingsData, &settings); err != nil {
				t.Fatalf("parse settings.json: %v\ncontent: %s", err, settingsData)
			}

			hooks := settings["hooks"].(map[string]interface{})

			_, hasStop := hooks["Stop"]
			_, hasStart := hooks["SessionStart"]

			if hasStop != tt.wantStop {
				t.Errorf("Stop hook: got %v, want %v", hasStop, tt.wantStop)
			}
			if hasStart != tt.wantStart {
				t.Errorf("SessionStart hook: got %v, want %v", hasStart, tt.wantStart)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Integration: MCP server — send + read flow
// ---------------------------------------------------------------------------

func TestIntegration_MCPFlow_SendAndRead(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "messages.db")

	store, err := messaging.OpenStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}

	// developer sends to reviewer
	id, _, err := store.Send("team-scope", "developer-1", "reviewer-1", "Please review my PR")
	if err != nil {
		t.Fatal(err)
	}

	// reviewer checks inbox
	inbox, err := store.ReadInbox("team-scope", "reviewer-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(inbox) != 1 {
		t.Fatalf("expected 1 unread, got %d", len(inbox))
	}
	if inbox[0].From != "developer-1" {
		t.Errorf("from = %q", inbox[0].From)
	}

	// reviewer reads full message
	msg, err := store.ReadMessage(id)
	if err != nil {
		t.Fatal(err)
	}
	if msg.Body != "Please review my PR" {
		t.Errorf("body = %q", msg.Body)
	}
	if msg.ReadAt != nil {
		t.Error("message should not be marked read yet")
	}

	// reviewer marks read
	if err := store.MarkRead(id); err != nil {
		t.Fatal(err)
	}

	// inbox should be empty
	inbox, err = store.ReadInbox("team-scope", "reviewer-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(inbox) != 0 {
		t.Errorf("expected 0 unread after mark_read, got %d", len(inbox))
	}

	// unread count should be 0
	count, err := store.UnreadCount("team-scope", "reviewer-1")
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("unread count = %d", count)
	}

	// list agents should show both
	agents, err := store.ListAgents("team-scope")
	if err != nil {
		t.Fatal(err)
	}
	if len(agents) != 2 {
		t.Errorf("expected 2 agents, got %d", len(agents))
	}

	_ = store.Close()
}

// ---------------------------------------------------------------------------
// Integration: Full messaging pipeline — check-inbox after conversation
// ---------------------------------------------------------------------------

func TestIntegration_FullMessagePipeline(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "messages.db")

	store, err := messaging.OpenStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}

	// 1. Developer sends review request
	_, _, _ = store.Send("team", "developer-1", "reviewer-1", "PR ready for review: feat/new-api")
	_ = store.Close()

	// 2. Reviewer's check-inbox fires (simulating Stop hook)
	t.Setenv("AW_MSG_CHECK_INTERVAL", "0")

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	err = runCheckInbox(dbPath, "reviewer-1", "team")
	_ = w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("check-inbox error: %v", err)
	}

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	output := strings.TrimSpace(string(buf[:n]))

	var resp hookResponse
	if err := json.Unmarshal([]byte(output), &resp); err != nil {
		t.Fatalf("not valid JSON: %q", output)
	}
	if resp.Decision != "block" {
		t.Errorf("decision = %q, want block", resp.Decision)
	}

	// 3. Reviewer reads and replies
	store, _ = messaging.OpenStore(dbPath)
	inbox, _ := store.ReadInbox("team", "reviewer-1")
	if len(inbox) != 1 {
		t.Fatalf("inbox len = %d", len(inbox))
	}
	_ = store.MarkRead(inbox[0].ID)
	_, _, _ = store.Send("team", "reviewer-1", "developer-1", "LGTM! Ship it.")
	_ = store.Close()

	// 4. Developer's check-inbox fires
	_ = os.Remove(filepath.Join(dir, ".lastcheck-developer-1"))

	r, w, _ = os.Pipe()
	os.Stdout = w
	err = runCheckInbox(dbPath, "developer-1", "team")
	_ = w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("developer check-inbox error: %v", err)
	}

	buf = make([]byte, 4096)
	n, _ = r.Read(buf)
	output = strings.TrimSpace(string(buf[:n]))

	if err := json.Unmarshal([]byte(output), &resp); err != nil {
		t.Fatalf("developer check-inbox not valid JSON: %q", output)
	}
	if resp.Decision != "block" {
		t.Errorf("developer should be blocked")
	}
}

// ---------------------------------------------------------------------------
// Edge case: CLI args via struct fields
// ---------------------------------------------------------------------------

func TestE2E_CheckInbox_StructFields(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "messages.db")

	store, err := messaging.OpenStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.Send("cli-team", "other", "cli-agent", "hello via CLI"); err != nil {
		t.Fatal(err)
	}
	_ = store.Close()

	t.Setenv("AW_MSG_CHECK_INTERVAL", "0")

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err = runCheckInbox(dbPath, "cli-agent", "cli-team")

	_ = w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("error: %v", err)
	}

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	output := strings.TrimSpace(string(buf[:n]))
	if !strings.Contains(output, "block") {
		t.Errorf("expected block response, got %q", output)
	}
}
