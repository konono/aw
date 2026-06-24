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
		wantErr    bool
	}{
		{
			name:     "no -c",
			args:     []string{"profile", "--recent"},
			wantKong: []string{"profile", "--recent"},
		},
		{
			name:     "with -c",
			args:     []string{"host-shell", "-c", "echo", "hello"},
			wantKong: []string{"host-shell"},
			wantCmd:  []string{"echo", "hello"},
		},
		{
			name:     "-c at start",
			args:     []string{"-c", "cmd"},
			wantCmd:  []string{"cmd"},
		},
		{
			name:    "-c with no args after",
			args:    []string{"profile", "-c"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kongArgs, cmdArgs, err := SplitAtDashC(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error for -c with no args")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(kongArgs) == 0 && len(tt.wantKong) == 0 {
				// both nil/empty — ok
			} else if len(kongArgs) != len(tt.wantKong) {
				t.Errorf("kongArgs = %v, want %v", kongArgs, tt.wantKong)
			} else {
				for i := range kongArgs {
					if kongArgs[i] != tt.wantKong[i] {
						t.Errorf("kongArgs[%d] = %q, want %q", i, kongArgs[i], tt.wantKong[i])
					}
				}
			}
			if len(cmdArgs) == 0 && len(tt.wantCmd) == 0 {
				// both nil/empty — ok
			} else if len(cmdArgs) != len(tt.wantCmd) {
				t.Errorf("cmdArgs = %v, want %v", cmdArgs, tt.wantCmd)
			} else {
				for i := range cmdArgs {
					if cmdArgs[i] != tt.wantCmd[i] {
						t.Errorf("cmdArgs[%d] = %q, want %q", i, cmdArgs[i], tt.wantCmd[i])
					}
				}
			}
		})
	}
}
