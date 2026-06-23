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
