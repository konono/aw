package launcher

import (
	"testing"
)

func TestAppendResumeFlags_Claude(t *testing.T) {
	cmd := []string{"claude", "--permission-mode", "bypassPermissions"}
	got := AppendResumeFlags("claude", cmd)
	if len(got) != 4 || got[3] != "--continue" {
		t.Errorf("got %v, want [..., --continue]", got)
	}
}

func TestAppendResumeFlags_Cursor(t *testing.T) {
	cmd := []string{"agent", "--force"}
	got := AppendResumeFlags("cursor", cmd)
	if len(got) != 3 || got[2] != "--continue" {
		t.Errorf("got %v, want [..., --continue]", got)
	}
}

func TestAppendResumeFlags_OpenCode(t *testing.T) {
	cmd := []string{"opencode"}
	got := AppendResumeFlags("opencode", cmd)
	if len(got) != 2 || got[1] != "--continue" {
		t.Errorf("got %v, want [opencode, --continue]", got)
	}
}

func TestAppendResumeFlags_Unknown(t *testing.T) {
	cmd := []string{"vim"}
	got := AppendResumeFlags("vim", cmd)
	if len(got) != 1 {
		t.Errorf("unknown tool should not add resume flags, got %v", got)
	}
}

func TestToolPrintCommand(t *testing.T) {
	tests := []struct {
		tool    string
		wantNil bool
		wantCmd string
	}{
		{"claude", false, "claude"},
		{"cursor", false, "agent"},
		{"codex", true, ""},
		{"opencode", true, ""},
		{"unknown", true, ""},
	}
	for _, tt := range tests {
		t.Run(tt.tool, func(t *testing.T) {
			cmd := ToolPrintCommand(tt.tool)
			if tt.wantNil {
				if cmd != nil {
					t.Errorf("expected nil for %q, got %v", tt.tool, cmd)
				}
				return
			}
			if cmd == nil {
				t.Fatalf("expected non-nil for %q", tt.tool)
			}
			if cmd[0] != tt.wantCmd {
				t.Errorf("got %v, want first element %q", cmd, tt.wantCmd)
			}
			// Must include -p flag
			hasP := false
			for _, arg := range cmd {
				if arg == "-p" {
					hasP = true
				}
			}
			if !hasP {
				t.Errorf("print command for %q missing -p flag: %v", tt.tool, cmd)
			}
		})
	}
}

func TestSupportsAgentLoop(t *testing.T) {
	if !SupportsAgentLoop("claude") {
		t.Error("claude should support agent loop")
	}
	if !SupportsAgentLoop("cursor") {
		t.Error("cursor should support agent loop")
	}
	if SupportsAgentLoop("codex") {
		t.Error("codex should not support agent loop")
	}
	if SupportsAgentLoop("opencode") {
		t.Error("opencode should not support agent loop")
	}
}

func TestToolContainerCommand(t *testing.T) {
	tests := []struct {
		tool    string
		wantNil bool
		wantCmd string
	}{
		{"claude", false, "claude"},
		{"codex", false, "codex"},
		{"opencode", false, "opencode"},
		{"cursor", false, "agent"},
		{"unknown", true, ""},
	}
	for _, tt := range tests {
		t.Run(tt.tool, func(t *testing.T) {
			cmd := ToolContainerCommand(tt.tool)
			if tt.wantNil {
				if cmd != nil {
					t.Errorf("expected nil for %q, got %v", tt.tool, cmd)
				}
				return
			}
			if cmd == nil || cmd[0] != tt.wantCmd {
				t.Errorf("got %v, want first element %q", cmd, tt.wantCmd)
			}
		})
	}
}
