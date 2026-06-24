package cmd

import (
	"testing"
)

func TestRunCmd_Validate_RecentAndCwdConflict(t *testing.T) {
	cmd := &RunCmd{Profile: "codex", Recent: true, Cwd: "/tmp"}
	err := cmd.Validate()
	if err == nil {
		t.Fatal("expected error for --recent + -C conflict")
	}
}

func TestRunCmd_Validate_QueryWithoutRecent(t *testing.T) {
	cmd := &RunCmd{Query: "test"}
	err := cmd.Validate()
	if err == nil {
		t.Fatal("expected error for --query without --recent")
	}
}

func TestRunCmd_Validate_ValidRecentWithQuery(t *testing.T) {
	cmd := &RunCmd{Profile: "claude", Recent: true, Query: "dotfiles"}
	err := cmd.Validate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunCmd_Validate_ProfileOnly(t *testing.T) {
	cmd := &RunCmd{Profile: "codex"}
	err := cmd.Validate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunCmd_Validate_Empty(t *testing.T) {
	cmd := &RunCmd{}
	err := cmd.Validate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSplitAtDashC(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantKong   []string
		wantCmd    []string
	}{
		{
			"no -c",
			[]string{"profile", "--recent"},
			[]string{"profile", "--recent"},
			nil,
		},
		{
			"with -c",
			[]string{"host-shell", "-c", "echo", "hello"},
			[]string{"host-shell"},
			[]string{"echo", "hello"},
		},
		{
			"-c at start",
			[]string{"-c", "cmd"},
			nil,
			[]string{"cmd"},
		},
		{
			"-c with no args after",
			[]string{"profile", "-c"},
			[]string{"profile"},
			nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kong, cmd := SplitAtDashC(tt.args)
			if len(kong) == 0 && len(tt.wantKong) == 0 {
				// both nil/empty — ok
			} else if len(kong) != len(tt.wantKong) {
				t.Errorf("kongArgs = %v, want %v", kong, tt.wantKong)
			} else {
				for i := range kong {
					if kong[i] != tt.wantKong[i] {
						t.Errorf("kongArgs[%d] = %q, want %q", i, kong[i], tt.wantKong[i])
					}
				}
			}
			if len(cmd) == 0 && len(tt.wantCmd) == 0 {
				// both nil/empty — ok
			} else if len(cmd) != len(tt.wantCmd) {
				t.Errorf("cmdArgs = %v, want %v", cmd, tt.wantCmd)
			} else {
				for i := range cmd {
					if cmd[i] != tt.wantCmd[i] {
						t.Errorf("cmdArgs[%d] = %q, want %q", i, cmd[i], tt.wantCmd[i])
					}
				}
			}
		})
	}
}
