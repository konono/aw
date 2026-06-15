package reaper

import "testing"

func TestSummarizeExit(t *testing.T) {
	tests := []struct {
		name string
		diag ContainerDiag
		want string
	}{
		{"normal exit", ContainerDiag{ExitCode: 0}, "exited normally"},
		{"OOM killed", ContainerDiag{ExitCode: 137, OOMKilled: true}, "container memory limit exceeded (OOM killed)"},
		{"VM OOM", ContainerDiag{ExitCode: 137, VMOOMHint: true}, "possible Podman VM memory exhaustion (VM OOM)"},
		{"SIGKILL", ContainerDiag{ExitCode: 137}, "killed by SIGKILL (exit 137)"},
		{"SIGTERM", ContainerDiag{ExitCode: 143}, "terminated by SIGTERM (exit 143)"},
		{"error", ContainerDiag{ExitCode: 1}, "exited with error (exit 1)"},
		{"not found", ContainerDiag{ExitCode: 127}, "command not found (exit 127)"},
		{"permission", ContainerDiag{ExitCode: 126}, "permission denied (exit 126)"},
		{"signal 6", ContainerDiag{ExitCode: 134}, "killed by signal 6 (exit 134)"},
		{"exit 42", ContainerDiag{ExitCode: 42}, "exited with code 42"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := summarizeExit(tt.diag)
			if got != tt.want {
				t.Errorf("summarizeExit() = %q, want %q", got, tt.want)
			}
		})
	}
}
