package launcher

import (
	"testing"
)

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
