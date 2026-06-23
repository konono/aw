package cmd

import (
	"testing"
)

func TestAppendTaskFlags_Claude(t *testing.T) {
	cmd := []string{"claude", "--permission-mode", "bypassPermissions"}
	got := appendTaskFlags("claude", cmd, "implement fizzbuzz")
	want := []string{"claude", "--permission-mode", "bypassPermissions", "--prompt", "implement fizzbuzz"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestAppendTaskFlags_NonClaude(t *testing.T) {
	cmd := []string{"codex", "-a", "never"}
	got := appendTaskFlags("codex", cmd, "implement fizzbuzz")
	if len(got) != len(cmd) {
		t.Errorf("non-claude tool should not add task flags, got %v", got)
	}
}

func TestAppendResumeFlags_Claude(t *testing.T) {
	cmd := []string{"claude"}
	got := appendResumeFlags("claude", cmd)
	if len(got) != 2 || got[1] != "--continue" {
		t.Errorf("got %v, want [claude --continue]", got)
	}
}

func TestAppendResumeFlags_Unknown(t *testing.T) {
	cmd := []string{"vim"}
	got := appendResumeFlags("vim", cmd)
	if len(got) != 1 {
		t.Errorf("unknown tool should not add resume flags, got %v", got)
	}
}
