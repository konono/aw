package cmd

import (
	"testing"

	"github.com/alecthomas/kong"
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

func newTestParser(t *testing.T) *kong.Kong {
	t.Helper()
	var cli CLI
	parser, err := kong.New(&cli, kong.Name("aw"), kong.Exit(func(int) {}))
	if err != nil {
		t.Fatalf("kong.New: %v", err)
	}
	return parser
}

func TestIsSubcommand(t *testing.T) {
	parser := newTestParser(t)
	tests := []struct {
		args []string
		want bool
	}{
		{[]string{"build", "myprofile"}, true},
		{[]string{"completion", "zsh", "-c"}, true},
		{[]string{"auth", "login", "claude"}, true},
		{[]string{"profiles"}, true},
		{[]string{"run", "myprofile", "-c", "echo"}, false},
		{[]string{"myprofile", "-c", "echo"}, false},
		{[]string{"-c", "echo"}, false},
		{[]string{}, false},
		{[]string{"--version"}, false},
	}
	for _, tt := range tests {
		t.Run(joinArgs(tt.args), func(t *testing.T) {
			got := IsSubcommand(parser, tt.args)
			if got != tt.want {
				t.Errorf("IsSubcommand(%v) = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}

func joinArgs(args []string) string {
	if len(args) == 0 {
		return "(empty)"
	}
	s := args[0]
	for _, a := range args[1:] {
		s += " " + a
	}
	return s
}

func TestExtractRunPassthrough(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantKong   []string
		wantCmd    []string
		wantLegacy bool
		wantErr    bool
	}{
		{
			name:     "no separator",
			args:     []string{"profile", "--recent"},
			wantKong: []string{"profile", "--recent"},
		},
		{
			name:       "legacy -c",
			args:       []string{"host-shell", "-c", "echo", "hello"},
			wantKong:   []string{"host-shell"},
			wantCmd:    []string{"echo", "hello"},
			wantLegacy: true,
		},
		{
			name:       "legacy -c at start",
			args:       []string{"-c", "cmd"},
			wantCmd:    []string{"cmd"},
			wantLegacy: true,
		},
		{
			name:    "-c with no args after",
			args:    []string{"profile", "-c"},
			wantErr: true,
		},
		{
			name:     "double dash",
			args:     []string{"profile", "--", "echo", "hello"},
			wantKong: []string{"profile"},
			wantCmd:  []string{"echo", "hello"},
		},
		{
			name:    "double dash at start",
			args:    []string{"--", "cmd"},
			wantCmd: []string{"cmd"},
		},
		{
			name:    "double dash with no args after",
			args:    []string{"profile", "--"},
			wantErr: true,
		},
		{
			name:     "double dash wins over -c",
			args:     []string{"profile", "--", "-c", "echo"},
			wantKong: []string{"profile"},
			wantCmd:  []string{"-c", "echo"},
		},
		{
			name:     "double dash before -c position",
			args:     []string{"--", "echo", "-c", "hello"},
			wantCmd:  []string{"echo", "-c", "hello"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kongArgs, cmdArgs, usedLegacy, err := ExtractRunPassthrough(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if usedLegacy != tt.wantLegacy {
				t.Errorf("usedLegacy = %v, want %v", usedLegacy, tt.wantLegacy)
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
