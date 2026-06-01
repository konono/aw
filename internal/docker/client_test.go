package docker

import (
	"testing"
)

func TestMountToString(t *testing.T) {
	tests := []struct {
		name     string
		mount    Mount
		wantSrc  string
		wantTgt  string
		wantRO   bool
		wantVol  bool
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

	// Should start with run -it --rm --pids-limit 1000
	expected := []string{"run", "-it", "--rm", "--pids-limit", "1000"}
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
		SecurityOpts: []string{"label=disable"},
	}

	args := BuildRunArgs(config)

	found := false
	for i, a := range args {
		if a == "--security-opt" && i+1 < len(args) && args[i+1] == "label=disable" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected --security-opt label=disable, got args: %v", args)
	}
}

func TestBuildRunArgs_MountOptions(t *testing.T) {
	tests := []struct {
		name     string
		mount    Mount
		wantArg  string
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
