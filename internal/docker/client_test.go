package docker

import (
	"fmt"
	"os/exec"
	"testing"
)

func TestIsImageInspectNotFound(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   bool
	}{
		{
			name:   "docker no such image",
			output: "Error response from daemon: No such image: missing:latest",
			want:   true,
		},
		{
			name:   "docker no such object",
			output: "Error: No such object: missing:latest",
			want:   true,
		},
		{
			name:   "podman image not known",
			output: "Error: missing:latest: image not known",
			want:   true,
		},
		{
			name:   "invalid reference format",
			output: "Error response from daemon: invalid reference format",
			want:   false,
		},
		{
			name:   "permission denied",
			output: "permission denied while trying to connect to the Docker daemon socket",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isImageInspectNotFound([]byte(tt.output))
			if got != tt.want {
				t.Fatalf("isImageInspectNotFound(%q) = %v, want %v", tt.output, got, tt.want)
			}
		})
	}
}

func TestMountToString(t *testing.T) {
	tests := []struct {
		name    string
		mount   Mount
		wantSrc string
		wantTgt string
		wantRO  bool
		wantVol bool
	}{
		{
			name:    "bind mount",
			mount:   Mount{Source: "/host/path", Target: "/container/path"},
			wantSrc: "/host/path",
			wantTgt: "/container/path",
		},
		{
			name:    "read-only bind mount",
			mount:   Mount{Source: "/host/ssh", Target: "/home/claude/.ssh-host", ReadOnly: true},
			wantSrc: "/host/ssh",
			wantTgt: "/home/claude/.ssh-host",
			wantRO:  true,
		},
		{
			name:    "named volume",
			mount:   Mount{Source: "my-volume", Target: "/data", IsVolume: true},
			wantSrc: "my-volume",
			wantTgt: "/data",
			wantVol: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.mount.Source != tt.wantSrc {
				t.Errorf("Source = %q, want %q", tt.mount.Source, tt.wantSrc)
			}
			if tt.mount.Target != tt.wantTgt {
				t.Errorf("Target = %q, want %q", tt.mount.Target, tt.wantTgt)
			}
			if tt.mount.ReadOnly != tt.wantRO {
				t.Errorf("ReadOnly = %v, want %v", tt.mount.ReadOnly, tt.wantRO)
			}
			if tt.mount.IsVolume != tt.wantVol {
				t.Errorf("IsVolume = %v, want %v", tt.mount.IsVolume, tt.wantVol)
			}
		})
	}
}

func TestRunConfigBuildArgs(t *testing.T) {
	// Test that BuildRunArgs produces the correct docker CLI arguments
	config := RunConfig{
		ImageName: "test-image",
		Mounts: []Mount{
			{Source: "vol1", Target: "/data", IsVolume: true},
			{Source: "/host/path", Target: "/container/path"},
			{Source: "/host/ssh", Target: "/home/.ssh-host", ReadOnly: true},
		},
		EnvVars: map[string]string{
			"FOO": "bar",
		},
		WorkDir: "/workspace",
		Command: []string{"claude", "--help"},
	}

	args := BuildRunArgs(config)

	// Should start with run -it --rm --init --pids-limit 8192
	expected := []string{"run", "-it", "--rm", "--init", "--pids-limit", "8192"}
	if len(args) < len(expected) {
		t.Fatalf("expected args to start with %v, got %v", expected, args)
	}
	for i, e := range expected {
		if args[i] != e {
			t.Errorf("args[%d] = %q, want %q (full prefix: %v)", i, args[i], e, args[:len(expected)])
			break
		}
	}

	// Should contain the image name
	found := false
	for _, a := range args {
		if a == "test-image" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected args to contain image name 'test-image', got %v", args)
	}

	// Should contain -e FOO=bar
	foundEnv := false
	for i, a := range args {
		if a == "-e" && i+1 < len(args) && args[i+1] == "FOO=bar" {
			foundEnv = true
			break
		}
	}
	if !foundEnv {
		t.Errorf("expected args to contain -e FOO=bar, got %v", args)
	}

	// Should contain -v with read-only mount
	foundRO := false
	for i, a := range args {
		if a == "-v" && i+1 < len(args) && args[i+1] == "/host/ssh:/home/.ssh-host:ro" {
			foundRO = true
			break
		}
	}
	if !foundRO {
		t.Errorf("expected args to contain read-only mount, got %v", args)
	}

	// Should contain --workdir /workspace
	foundWD := false
	for i, a := range args {
		if a == "--workdir" && i+1 < len(args) && args[i+1] == "/workspace" {
			foundWD = true
			break
		}
	}
	if !foundWD {
		t.Errorf("expected args to contain --workdir /workspace, got %v", args)
	}

	// Command should be at the end, after image name
	lastTwo := args[len(args)-2:]
	if lastTwo[0] != "claude" || lastTwo[1] != "--help" {
		t.Errorf("expected args to end with [claude --help], got %v", lastTwo)
	}
}

func TestBuildRunArgsNoOptionalFields(t *testing.T) {
	config := RunConfig{
		ImageName: "test-image",
		Command:   []string{"echo"},
	}

	args := BuildRunArgs(config)

	// Should not contain --workdir
	for _, a := range args {
		if a == "--workdir" {
			t.Error("expected no --workdir when WorkDir is empty")
		}
	}

	// Should not contain -v
	for _, a := range args {
		if a == "-v" {
			t.Error("expected no -v when Mounts is empty")
		}
	}

	// Should not contain -e
	for _, a := range args {
		if a == "-e" {
			t.Error("expected no -e when EnvVars is empty/nil")
		}
	}

	// Should not contain --security-opt
	for _, a := range args {
		if a == "--security-opt" {
			t.Error("expected no --security-opt when SecurityOpts is empty")
		}
	}
}

func TestBuildRunArgs_SecurityOpts(t *testing.T) {
	config := RunConfig{
		ImageName:    "test-image",
		Command:      []string{"sh"},
		SecurityOpts: []string{"label=type:spc_t"},
	}

	args := BuildRunArgs(config)

	found := false
	for i, a := range args {
		if a == "--security-opt" && i+1 < len(args) && args[i+1] == "label=type:spc_t" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected --security-opt label=type:spc_t, got args: %v", args)
	}
}

func TestBuildOneShotRunArgs(t *testing.T) {
	config := RunConfig{
		ImageName: "test-image",
		Mounts: []Mount{
			{Source: "/host/ws", Target: "/workspace", ReadOnly: true},
		},
		EnvVars: map[string]string{
			"FOO": "bar",
		},
		Command: []string{"/bin/bash", "-c", "echo hello"},
	}

	args := BuildOneShotRunArgs("aw-snapshot-abcd", config)

	expected := []string{"run", "--name", "aw-snapshot-abcd", "--pids-limit", "8192"}
	if len(args) < len(expected) {
		t.Fatalf("expected args to start with %v, got %v", expected, args)
	}
	for i, e := range expected {
		if args[i] != e {
			t.Errorf("args[%d] = %q, want %q", i, args[i], e)
		}
	}

	// Should NOT contain -it or --rm
	for _, a := range args {
		if a == "-it" {
			t.Error("one-shot args should not contain -it")
		}
		if a == "--rm" {
			t.Error("one-shot args should not contain --rm")
		}
	}

	// Should contain env var
	foundEnv := false
	for i, a := range args {
		if a == "-e" && i+1 < len(args) && args[i+1] == "FOO=bar" {
			foundEnv = true
			break
		}
	}
	if !foundEnv {
		t.Errorf("expected -e FOO=bar, got %v", args)
	}

	// Should contain mount
	foundMount := false
	for i, a := range args {
		if a == "-v" && i+1 < len(args) && args[i+1] == "/host/ws:/workspace:ro" {
			foundMount = true
			break
		}
	}
	if !foundMount {
		t.Errorf("expected mount /host/ws:/workspace:ro, got %v", args)
	}

	// Should NOT contain --entrypoint (not set)
	for _, a := range args {
		if a == "--entrypoint" {
			t.Error("one-shot args should not contain --entrypoint when Entrypoint is empty")
		}
	}
}

func TestBuildOneShotRunArgs_Entrypoint(t *testing.T) {
	config := RunConfig{
		ImageName:  "test-image",
		Entrypoint: "/bin/bash",
		Command:    []string{"-c", "echo hello"},
	}

	args := BuildOneShotRunArgs("aw-snapshot-1234", config)

	foundEntrypoint := false
	for i, a := range args {
		if a == "--entrypoint" && i+1 < len(args) && args[i+1] == "/bin/bash" {
			foundEntrypoint = true
			break
		}
	}
	if !foundEntrypoint {
		t.Errorf("expected --entrypoint /bin/bash, got %v", args)
	}

	// --entrypoint must appear before image name
	var entrypointIdx, imageIdx int
	for i, a := range args {
		if a == "--entrypoint" {
			entrypointIdx = i
		}
		if a == "test-image" {
			imageIdx = i
		}
	}
	if entrypointIdx > imageIdx {
		t.Errorf("--entrypoint (idx %d) must appear before image name (idx %d)", entrypointIdx, imageIdx)
	}
}

func TestBuildRunArgs_Userns(t *testing.T) {
	config := RunConfig{
		ImageName: "test-image",
		Command:   []string{"sh"},
		Userns:    "keep-id",
	}

	args := BuildRunArgs(config)

	found := false
	for i, a := range args {
		if a == "--userns" && i+1 < len(args) && args[i+1] == "keep-id" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected --userns keep-id, got args: %v", args)
	}
}

func TestBuildRunArgs_NoUserns(t *testing.T) {
	config := RunConfig{
		ImageName: "test-image",
		Command:   []string{"sh"},
	}

	args := BuildRunArgs(config)

	for _, a := range args {
		if a == "--userns" {
			t.Error("expected no --userns when Userns is empty")
		}
	}
}

func TestBuildOneShotRunArgs_Userns(t *testing.T) {
	config := RunConfig{
		ImageName: "test-image",
		Command:   []string{"sh"},
		Userns:    "keep-id",
	}

	args := BuildOneShotRunArgs("aw-snapshot-test", config)

	found := false
	for i, a := range args {
		if a == "--userns" && i+1 < len(args) && args[i+1] == "keep-id" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected --userns keep-id in one-shot args, got: %v", args)
	}
}

func TestBuildDetachedRunArgs(t *testing.T) {
	config := RunConfig{
		ImageName: "test-image",
		Command:   []string{"claude"},
	}

	args := BuildDetachedRunArgs("aw-claude-123", config)

	expected := []string{"run", "-it", "--init", "-d", "--name", "aw-claude-123", "--pids-limit", "8192"}
	if len(args) < len(expected) {
		t.Fatalf("expected args to start with %v, got %v", expected, args)
	}
	for i, e := range expected {
		if args[i] != e {
			t.Errorf("args[%d] = %q, want %q", i, args[i], e)
		}
	}

	for _, a := range args {
		if a == "--rm" {
			t.Error("detached args should not contain --rm")
		}
	}
}

func TestIsInspectNotRecoverable(t *testing.T) {
	tests := []struct {
		name   string
		output string
		err    error
		want   bool
	}{
		{
			name:   "docker no such container",
			output: "Error: No such container: aw-test-123",
			err:    fmt.Errorf("exit status 1"),
			want:   true,
		},
		{
			name:   "podman no such container",
			output: "Error: no such container aw-test-123",
			err:    fmt.Errorf("exit status 125"),
			want:   true,
		},
		{
			name:   "no such object",
			output: "Error: No such object: aw-test-123",
			err:    fmt.Errorf("exit status 1"),
			want:   true,
		},
		{
			name:   "runtime binary not found",
			output: "",
			err:    &exec.Error{Name: "podman", Err: exec.ErrNotFound},
			want:   true,
		},
		{
			name:   "transient API error",
			output: "Error: unable to connect to podman socket",
			err:    fmt.Errorf("exit status 125"),
			want:   false,
		},
		{
			name:   "connection refused",
			output: "Cannot connect to the Docker daemon",
			err:    fmt.Errorf("exit status 1"),
			want:   false,
		},
		{
			name:   "empty output with exit error",
			output: "",
			err:    fmt.Errorf("exit status 1"),
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsInspectNotRecoverable([]byte(tt.output), tt.err)
			if got != tt.want {
				t.Fatalf("IsInspectNotRecoverable(%q, %v) = %v, want %v", tt.output, tt.err, got, tt.want)
			}
		})
	}
}

func TestBuildRunArgs_GroupAdd(t *testing.T) {
	config := RunConfig{
		ImageName: "test-image",
		Command:   []string{"sh"},
		GroupAdd:  []string{"0"},
	}

	args := BuildRunArgs(config)

	found := false
	for i, a := range args {
		if a == "--group-add" && i+1 < len(args) && args[i+1] == "0" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected --group-add 0, got args: %v", args)
	}
}

func TestBuildRunArgs_GroupAddMultiple(t *testing.T) {
	config := RunConfig{
		ImageName: "test-image",
		Command:   []string{"sh"},
		GroupAdd:  []string{"0", "100"},
	}

	args := BuildRunArgs(config)

	groups := map[string]bool{}
	for i, a := range args {
		if a == "--group-add" && i+1 < len(args) {
			groups[args[i+1]] = true
		}
	}
	if !groups["0"] || !groups["100"] {
		t.Errorf("expected --group-add 0 and --group-add 100, got args: %v", args)
	}
}

func TestBuildRunArgs_NoGroupAdd(t *testing.T) {
	config := RunConfig{
		ImageName: "test-image",
		Command:   []string{"sh"},
	}

	args := BuildRunArgs(config)

	for _, a := range args {
		if a == "--group-add" {
			t.Error("expected no --group-add when GroupAdd is empty")
		}
	}
}

func TestBuildRunArgs_DockerPermissions(t *testing.T) {
	config := RunConfig{
		ImageName: "test-image",
		Command:   []string{"claude"},
		User:      "1000:1000",
		GroupAdd:  []string{"0"},
	}

	args := BuildRunArgs(config)

	foundUser := false
	foundGroup := false
	for i, a := range args {
		if a == "--user" && i+1 < len(args) && args[i+1] == "1000:1000" {
			foundUser = true
		}
		if a == "--group-add" && i+1 < len(args) && args[i+1] == "0" {
			foundGroup = true
		}
	}
	if !foundUser {
		t.Errorf("expected --user 1000:1000, got args: %v", args)
	}
	if !foundGroup {
		t.Errorf("expected --group-add 0, got args: %v", args)
	}
}

func TestBuildRunArgs_PodmanPermissions(t *testing.T) {
	config := RunConfig{
		ImageName: "test-image",
		Command:   []string{"claude"},
		User:      "1000:1000",
		GroupAdd:  []string{"0"},
		Userns:    "keep-id",
	}

	args := BuildRunArgs(config)

	foundUserns := false
	foundUser := false
	foundGroup := false
	for i, a := range args {
		if a == "--userns" && i+1 < len(args) && args[i+1] == "keep-id" {
			foundUserns = true
		}
		if a == "--user" && i+1 < len(args) && args[i+1] == "1000:1000" {
			foundUser = true
		}
		if a == "--group-add" && i+1 < len(args) && args[i+1] == "0" {
			foundGroup = true
		}
	}
	if !foundUserns {
		t.Errorf("expected --userns keep-id, got args: %v", args)
	}
	if !foundUser {
		t.Errorf("expected --user 1000:1000, got args: %v", args)
	}
	if !foundGroup {
		t.Errorf("expected --group-add 0, got args: %v", args)
	}
}

func TestBuildOneShotRunArgs_GroupAdd(t *testing.T) {
	config := RunConfig{
		ImageName: "test-image",
		Command:   []string{"sh"},
		GroupAdd:  []string{"0"},
	}

	args := BuildOneShotRunArgs("aw-snapshot-test", config)

	found := false
	for i, a := range args {
		if a == "--group-add" && i+1 < len(args) && args[i+1] == "0" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected --group-add 0 in one-shot args, got: %v", args)
	}
}

func TestBuildRunArgs_MountOptions(t *testing.T) {
	tests := []struct {
		name    string
		mount   Mount
		wantArg string
	}{
		{
			name:    "rw no options",
			mount:   Mount{Source: "/src", Target: "/dst"},
			wantArg: "/src:/dst",
		},
		{
			name:    "ro no options",
			mount:   Mount{Source: "/src", Target: "/dst", ReadOnly: true},
			wantArg: "/src:/dst:ro",
		},
		{
			name:    "rw with selinux",
			mount:   Mount{Source: "/src", Target: "/dst", Options: "z"},
			wantArg: "/src:/dst:z",
		},
		{
			name:    "ro with selinux",
			mount:   Mount{Source: "/src", Target: "/dst", ReadOnly: true, Options: "Z"},
			wantArg: "/src:/dst:ro,Z",
		},
		{
			name:    "rw with multiple options",
			mount:   Mount{Source: "/src", Target: "/dst", Options: "z,nocopy"},
			wantArg: "/src:/dst:z,nocopy",
		},
		{
			name:    "ro with multiple options",
			mount:   Mount{Source: "/src", Target: "/dst", ReadOnly: true, Options: "Z,nocopy"},
			wantArg: "/src:/dst:ro,Z,nocopy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := RunConfig{
				ImageName: "img",
				Mounts:    []Mount{tt.mount},
				Command:   []string{"sh"},
			}
			args := BuildRunArgs(config)
			found := false
			for i, a := range args {
				if a == "-v" && i+1 < len(args) && args[i+1] == tt.wantArg {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected mount arg %q, got args: %v", tt.wantArg, args)
			}
		})
	}
}
