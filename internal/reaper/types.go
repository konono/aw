package reaper

import (
	"encoding/json"
	"time"
)

// ReaperSpec describes the post-container tasks to execute.
// It is serialized as a single JSON line over the pipe from aw to the reaper.
type ReaperSpec struct {
	Timeout       int              `json:"timeout"`
	ContainerName string           `json:"container_name"`
	Runtime       string           `json:"runtime"`
	KeepContainer   bool             `json:"keep_container"`
	TTY             string           `json:"tty,omitempty"`
	ReportRetention int              `json:"report_retention,omitempty"`
	PodmanSSH       *PodmanSSHConfig `json:"podman_ssh,omitempty"`
	Tasks         []ReaperTask     `json:"tasks"`
}

// PodmanSSHConfig holds SSH connection info for macOS Podman VM operations.
type PodmanSSHConfig struct {
	IdentityPath   string `json:"identity_path"`
	Port           int    `json:"port"`
	RemoteUsername string `json:"remote_username"`
}

// ReaperTask is a single post-container task.
type ReaperTask struct {
	Type   string          `json:"type"`
	Label  string          `json:"label,omitempty"`
	Config json.RawMessage `json:"config"`
}

// KillProcessConfig kills a process by PID after verifying its identity.
type KillProcessConfig struct {
	PID    int `json:"pid"`
	Signal int `json:"signal"`
}

// RemoveFileConfig removes a file or socket.
// Host "podman-vm" means the file is inside the Podman VM.
type RemoveFileConfig struct {
	Path string `json:"path"`
	Host string `json:"host,omitempty"`
}

// ShellConfig runs an arbitrary shell command.
type ShellConfig struct {
	Command string `json:"command"`
	Dir     string `json:"dir,omitempty"`
}

// RunReport records the outcome of a reaper run.
type RunReport struct {
	StartedAt     time.Time      `json:"started_at"`
	FinishedAt    time.Time      `json:"finished_at"`
	ContainerName string         `json:"container_name"`
	ExitCode      int            `json:"exit_code"`
	ContainerDiag *ContainerDiag `json:"container_diag,omitempty"`
	ContainerKept bool           `json:"container_kept"`
	Tasks         []TaskResult   `json:"tasks"`
}

// ContainerDiag records detailed container exit diagnostics.
type ContainerDiag struct {
	ExitCode  int    `json:"exit_code"`
	OOMKilled bool   `json:"oom_killed"`
	VMOOMHint bool   `json:"vm_oom_hint,omitempty"`
	ExitedAt  string `json:"exited_at,omitempty"`
	Error     string `json:"error,omitempty"`
	Summary   string `json:"summary"`
}

// TaskResult records the outcome of a single reaper task.
type TaskResult struct {
	Type     string        `json:"type"`
	Label    string        `json:"label"`
	Status   string        `json:"status"` // "ok", "failed", "timeout", "skipped"
	Duration time.Duration `json:"duration"`
	Error    string        `json:"error,omitempty"`
}
