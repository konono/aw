package image

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The entrypoint decides whether to run mise install at launch. These tests
// drive the real script under bash with the surrounding helpers stubbed out, so
// the four startup paths are covered without building an image. Only the
// `. /aw-init.sh` line is rewritten -- that path is absolute and owned by the
// container -- everything the tests assert on runs verbatim.

const miseInstallMarker = "STUB-MISE-INSTALL-RAN"

// awInitStub stands in for /aw-init.sh. It provides the helpers the entrypoint
// expects and reports an install instead of performing one.
const awInitStub = `
aw_log() { echo "$@"; }
run_as_user() {
  case "$1" in
    *"command -v mise"*) return 0 ;;
    *"mise install"*) echo "` + miseInstallMarker + `" ;;
  esac
  return 0
}
aw_fix_mise_shims() { :; }
aw_exec() { :; }
`

type entrypointRun struct {
	// miseToml is written to the workspace; empty means no mise config.
	miseToml string
	// imageFingerprint is baked into the image; empty means none, which is
	// what a pulled official image looks like.
	imageFingerprint string
	// env is the container environment the aw CLI would pass in.
	env map[string]string
}

func runEntrypoint(t *testing.T, r entrypointRun) string {
	t.Helper()

	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	workspace := filepath.Join(dir, "workspace")
	for _, d := range []string{home, workspace} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}

	if r.miseToml != "" {
		if err := os.WriteFile(filepath.Join(workspace, "mise.toml"), []byte(r.miseToml), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if r.imageFingerprint != "" {
		if err := os.WriteFile(filepath.Join(home, ".aw_mise_fingerprint"), []byte(r.imageFingerprint+"\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	initPath := filepath.Join(dir, "aw-init.sh")
	if err := os.WriteFile(initPath, []byte(awInitStub), 0644); err != nil {
		t.Fatal(err)
	}

	script := string(Entrypoint())
	const initLine = ". /aw-init.sh"
	if !strings.Contains(script, initLine) {
		t.Fatalf("entrypoint no longer sources %s; update this test's stubbing", initLine)
	}
	script = strings.Replace(script, initLine, ". \""+initPath+"\"", 1)

	entrypointPath := filepath.Join(dir, "entrypoint.sh")
	if err := os.WriteFile(entrypointPath, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("bash", entrypointPath)
	cmd.Env = append(os.Environ(),
		"AW_HOME="+home,
		"AW_WORKSPACE="+workspace,
	)
	for k, v := range r.env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("entrypoint failed: %v\n%s", err, out)
	}
	return string(out)
}

func TestEntrypointMiseInstallDecision(t *testing.T) {
	const fingerprint = "sha256:deadbeef"
	const miseToml = "[tools]\njq = \"latest\"\n"

	t.Run("unchanged inputs skip the install", func(t *testing.T) {
		out := runEntrypoint(t, entrypointRun{
			miseToml:         miseToml,
			imageFingerprint: fingerprint,
			env:              map[string]string{"AW_MISE_FINGERPRINT": fingerprint},
		})
		if strings.Contains(out, miseInstallMarker) {
			t.Errorf("mise install ran even though the image matches the workspace:\n%s", out)
		}
		if !strings.Contains(out, "Skipping mise install") {
			t.Errorf("expected a skip message, got:\n%s", out)
		}
	})

	t.Run("changed inputs run the install", func(t *testing.T) {
		out := runEntrypoint(t, entrypointRun{
			miseToml:         miseToml + "fd = \"latest\"\n",
			imageFingerprint: fingerprint,
			env:              map[string]string{"AW_MISE_FINGERPRINT": "sha256:something-else"},
		})
		if !strings.Contains(out, miseInstallMarker) {
			t.Errorf("mise install should run after the workspace changed:\n%s", out)
		}
	})

	t.Run("unknown inputs run the install", func(t *testing.T) {
		// The CLI leaves AW_MISE_FINGERPRINT unset when it cannot account for
		// every mise input, e.g. a workspace carrying mise.lock.
		out := runEntrypoint(t, entrypointRun{
			miseToml:         miseToml,
			imageFingerprint: fingerprint,
		})
		if !strings.Contains(out, miseInstallMarker) {
			t.Errorf("mise install should run when the CLI sends no fingerprint:\n%s", out)
		}
	})

	t.Run("official image runs the install", func(t *testing.T) {
		// A pulled official image has no baked-in tools and no fingerprint.
		out := runEntrypoint(t, entrypointRun{
			miseToml: miseToml,
			env:      map[string]string{"AW_MISE_FINGERPRINT": fingerprint},
		})
		if !strings.Contains(out, miseInstallMarker) {
			t.Errorf("mise install should run when the image has no fingerprint:\n%s", out)
		}
	})

	t.Run("explicit opt-out wins over a mismatch", func(t *testing.T) {
		out := runEntrypoint(t, entrypointRun{
			miseToml:         miseToml + "fd = \"latest\"\n",
			imageFingerprint: fingerprint,
			env: map[string]string{
				"AW_SKIP_MISE_INSTALL": "1",
				"AW_MISE_FINGERPRINT":  "sha256:something-else",
			},
		})
		if strings.Contains(out, miseInstallMarker) {
			t.Errorf("mise_install: false must hold even when the inputs changed:\n%s", out)
		}
		if !strings.Contains(out, "skip_mise_install is enabled") {
			t.Errorf("expected the opt-out message, got:\n%s", out)
		}
	})

	t.Run("no mise config installs nothing", func(t *testing.T) {
		out := runEntrypoint(t, entrypointRun{})
		if strings.Contains(out, miseInstallMarker) {
			t.Errorf("nothing to install without a mise config:\n%s", out)
		}
		if !strings.Contains(out, "No mise.toml found") {
			t.Errorf("expected the no-config message, got:\n%s", out)
		}
	})
}
