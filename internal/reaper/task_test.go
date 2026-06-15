package reaper

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestExecuteTask_UnknownType(t *testing.T) {
	task := ReaperTask{
		Type:   "nonexistent",
		Config: json.RawMessage(`{}`),
	}
	spec := &ReaperSpec{}
	err := executeTask(context.Background(), task, spec)
	if err != nil {
		t.Errorf("unknown task type should not error, got: %v", err)
	}
}

func TestExecuteTask_KillProcess_PIDNotExist(t *testing.T) {
	cfg, _ := json.Marshal(KillProcessConfig{PID: 999999999, Signal: 15})
	task := ReaperTask{
		Type:   "kill_process",
		Config: cfg,
	}
	spec := &ReaperSpec{}
	err := executeTask(context.Background(), task, spec)
	if err != nil {
		t.Errorf("kill_process with non-existent PID should be no-op, got: %v", err)
	}
}

func TestExecuteTask_RemoveFile_NotExist(t *testing.T) {
	cfg, _ := json.Marshal(RemoveFileConfig{Path: "/tmp/nonexistent-aw-test-file-12345"})
	task := ReaperTask{
		Type:   "remove_file",
		Config: cfg,
	}
	spec := &ReaperSpec{}
	err := executeTask(context.Background(), task, spec)
	if err != nil {
		t.Errorf("remove_file with non-existent path should be no-op, got: %v", err)
	}
}

func TestRunShell_Timeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := runShell(ctx, ShellConfig{Command: "sleep 10"})
	if err == nil {
		t.Error("expected error from timed-out shell command")
	}
}

func TestExecuteTask_Dispatch(t *testing.T) {
	tests := []struct {
		name    string
		task    ReaperTask
		wantErr bool
	}{
		{
			name: "kill_process dispatch",
			task: ReaperTask{
				Type:   "kill_process",
				Config: json.RawMessage(`{"pid": 999999999, "signal": 15}`),
			},
		},
		{
			name: "remove_file dispatch",
			task: ReaperTask{
				Type:   "remove_file",
				Config: json.RawMessage(`{"path": "/tmp/nonexistent-12345"}`),
			},
		},
		{
			name: "shell dispatch",
			task: ReaperTask{
				Type:   "shell",
				Config: json.RawMessage(`{"command": "true"}`),
			},
		},
		{
			name: "unknown type",
			task: ReaperTask{
				Type:   "future_type",
				Config: json.RawMessage(`{}`),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := &ReaperSpec{}
			err := executeTask(context.Background(), tt.task, spec)
			if tt.wantErr && err == nil {
				t.Error("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
