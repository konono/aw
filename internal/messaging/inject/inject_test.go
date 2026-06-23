package inject

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testConfig(t *testing.T) InjectorConfig {
	t.Helper()
	dir := t.TempDir()
	return InjectorConfig{
		AgentName:    "developer-1",
		TeamName:     "team-abc123-uuid1",
		Role:         "developer",
		Members:      []MemberInfo{{AgentName: "developer-1", Role: "developer"}, {AgentName: "reviewer-1", Role: "reviewer"}},
		StagingDir:   dir,
		WorkDir:      dir,
		MCPBinary:    "/home/agent/.aw-msg/bin/aw",
		DBPath:       "/home/agent/.aw-msg/messages.db",
		DeliveryMode: DeliveryTurn,
	}
}

func TestClaudeInjectMCP(t *testing.T) {
	cfg := testConfig(t)
	injector := &ClaudeInjector{}

	if err := injector.InjectMCP(cfg); err != nil {
		t.Fatalf("InjectMCP: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(cfg.WorkDir, ".mcp.json"))
	if err != nil {
		t.Fatalf("reading .mcp.json: %v", err)
	}

	var root map[string]interface{}
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatalf("parsing .mcp.json: %v", err)
	}

	servers, _ := root["mcpServers"].(map[string]interface{})
	awMsg, _ := servers["aw-msg"].(map[string]interface{})
	if awMsg == nil {
		t.Fatal("aw-msg server not found in .mcp.json")
	}

	args, _ := awMsg["args"].([]interface{})
	argStr := make([]string, len(args))
	for i, a := range args {
		argStr[i], _ = a.(string)
	}
	joined := strings.Join(argStr, " ")
	if !strings.Contains(joined, "--team") {
		t.Errorf("MCP args missing --team flag: %v", argStr)
	}
	if !strings.Contains(joined, cfg.TeamName) {
		t.Errorf("MCP args missing team name: %v", argStr)
	}
}

func TestClaudeInjectHookTurnMode(t *testing.T) {
	cfg := testConfig(t)
	cfg.DeliveryMode = DeliveryTurn
	injector := &ClaudeInjector{}

	if err := injector.InjectHook(cfg); err != nil {
		t.Fatalf("InjectHook: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(cfg.StagingDir, "settings.json"))
	if err != nil {
		t.Fatalf("reading settings.json: %v", err)
	}
	if !strings.Contains(string(data), "--internal-check-inbox") {
		t.Error("settings.json should contain check-inbox hook")
	}
}

func TestClaudeInjectHookOffMode(t *testing.T) {
	cfg := testConfig(t)
	cfg.DeliveryMode = DeliveryOff
	injector := &ClaudeInjector{}

	if err := injector.InjectHook(cfg); err != nil {
		t.Fatalf("InjectHook: %v", err)
	}

	if _, err := os.Stat(filepath.Join(cfg.StagingDir, "settings.json")); err == nil {
		t.Error("settings.json should not be created when DeliveryMode is off")
	}
}

func TestClaudeInjectHookIdempotent(t *testing.T) {
	cfg := testConfig(t)
	injector := &ClaudeInjector{}

	_ = injector.InjectHook(cfg)
	_ = injector.InjectHook(cfg)

	data, _ := os.ReadFile(filepath.Join(cfg.StagingDir, "settings.json"))
	count := strings.Count(string(data), "--internal-check-inbox")
	if count != 1 {
		t.Errorf("hook should appear exactly once, found %d times", count)
	}
}

func TestCodexInjectMCP(t *testing.T) {
	cfg := testConfig(t)
	injector := &CodexInjector{}

	if err := injector.InjectMCP(cfg); err != nil {
		t.Fatalf("InjectMCP: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(cfg.StagingDir, "config.toml"))
	if err != nil {
		t.Fatalf("reading config.toml: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "--team") {
		t.Error("config.toml should contain --team flag")
	}
	if !strings.Contains(content, cfg.TeamName) {
		t.Error("config.toml should contain team name")
	}
}

func TestCodexInjectHookOffMode(t *testing.T) {
	cfg := testConfig(t)
	cfg.DeliveryMode = DeliveryOff
	injector := &CodexInjector{}

	if err := injector.InjectHook(cfg); err != nil {
		t.Fatalf("InjectHook: %v", err)
	}

	if _, err := os.Stat(filepath.Join(cfg.StagingDir, "config.toml")); err == nil {
		data, _ := os.ReadFile(filepath.Join(cfg.StagingDir, "config.toml"))
		if strings.Contains(string(data), "hooks.aw-msg") {
			t.Error("hook should not be injected when DeliveryMode is off")
		}
	}
}

func TestCursorInjectMCP(t *testing.T) {
	cfg := testConfig(t)
	injector := &CursorInjector{}

	if err := injector.InjectMCP(cfg); err != nil {
		t.Fatalf("InjectMCP: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(cfg.StagingDir, "mcp.json"))
	if err != nil {
		t.Fatalf("reading mcp.json: %v", err)
	}
	if !strings.Contains(string(data), "--team") {
		t.Error("mcp.json should contain --team flag")
	}
}

func TestOpenCodeInjectMCP(t *testing.T) {
	cfg := testConfig(t)
	injector := &OpenCodeInjector{}

	if err := injector.InjectMCP(cfg); err != nil {
		t.Fatalf("InjectMCP: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(cfg.StagingDir, "opencode.json"))
	if err != nil {
		t.Fatalf("reading opencode.json: %v", err)
	}
	if !strings.Contains(string(data), "--team") {
		t.Error("opencode.json should contain --team flag")
	}
}

func TestForTool(t *testing.T) {
	tools := []string{"claude", "codex", "opencode", "cursor"}
	for _, tool := range tools {
		t.Run(tool, func(t *testing.T) {
			inj, err := ForTool(tool)
			if err != nil {
				t.Fatalf("ForTool(%q): %v", tool, err)
			}
			if inj == nil {
				t.Fatalf("ForTool(%q) returned nil", tool)
			}
		})
	}

	_, err := ForTool("unknown")
	if err == nil {
		t.Fatal("expected error for unknown tool")
	}
}
