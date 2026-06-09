//go:build integration

package image

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/konono/aw/internal/containerenv"
	"github.com/konono/aw/internal/profile"
	"github.com/konono/aw/internal/toolinfo"
)

var allOSTemplates = []profile.OSTemplate{
	profile.OSDebian12,
	profile.OSUBI9,
	profile.OSUBI10,
	profile.OSUbuntu2604,
}

var integrationTools = []string{"claude", "codex", "opencode"}

const progressLogInterval = 30 * time.Second

func detectRuntime() string {
	if r := os.Getenv("CONTAINER_RUNTIME"); r != "" {
		return r
	}
	if _, err := exec.LookPath("podman"); err == nil {
		return "podman"
	}
	return "docker"
}

func runCommandWithProgress(t *testing.T, cmd *exec.Cmd, label string) error {
	t.Helper()

	start := time.Now()
	t.Logf("%s: starting", label)
	if err := cmd.Start(); err != nil {
		return err
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	ticker := time.NewTicker(progressLogInterval)
	defer ticker.Stop()

	for {
		select {
		case err := <-done:
			if err == nil {
				t.Logf("%s: completed in %s", label, time.Since(start).Round(time.Second))
			}
			return err
		case <-ticker.C:
			t.Logf("%s: still running after %s", label, time.Since(start).Round(time.Second))
		}
	}
}

func buildImage(t *testing.T, runtime, imageName, contextDir string, buildArgs map[string]string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	args := []string{"build", "-t", imageName}
	for k, v := range buildArgs {
		args = append(args, "--build-arg", fmt.Sprintf("%s=%s", k, v))
	}
	args = append(args, contextDir)

	cmd := exec.CommandContext(ctx, runtime, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := runCommandWithProgress(t, cmd, fmt.Sprintf("build %s", imageName)); err != nil {
		t.Fatalf("build %s failed: %v", imageName, err)
	}
}

func runInContainer(t *testing.T, runtime, imageName, script string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, runtime, "run", "--rm", "-i", imageName)
	cmd.Stdin = strings.NewReader(script)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := runCommandWithProgress(t, cmd, fmt.Sprintf("run %s", imageName)); err != nil {
		t.Fatalf("run in %s failed: %v\noutput: %s", imageName, err, out.String())
	}
	return out.String()
}

func runDetachedContainer(t *testing.T, runtime, imageName string, command ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	args := []string{"run", "--rm", "-d", imageName}
	args = append(args, command...)
	cmd := exec.CommandContext(ctx, runtime, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := runCommandWithProgress(t, cmd, fmt.Sprintf("run detached %s", imageName)); err != nil {
		t.Fatalf("run detached %s failed: %v\noutput: %s", imageName, err, out.String())
	}

	return strings.TrimSpace(out.String())
}

func execInContainer(t *testing.T, runtime, containerID string, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cmdArgs := append([]string{"exec", containerID}, args...)
	cmd := exec.CommandContext(ctx, runtime, cmdArgs...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := runCommandWithProgress(t, cmd, fmt.Sprintf("exec %s", strings.Join(args, " "))); err != nil {
		t.Fatalf("exec in %s failed: %v\noutput: %s", containerID, err, out.String())
	}
	return out.String()
}

func removeImage(runtime, imageName string) {
	_ = exec.Command(runtime, "rmi", "-f", imageName).Run()
}

func removeContainer(runtime, containerID string) {
	_ = exec.Command(runtime, "rm", "-f", containerID).Run()
}

func TestIntegration_ShellPerOS(t *testing.T) {
	runtime := detectRuntime()
	t.Logf("container runtime: %s", runtime)

	// Build-heavy integration tests run sequentially so progress logs stay readable
	// and the local container runtime is not overloaded.
	for _, osTemplate := range allOSTemplates {
		t.Run(string(osTemplate), func(t *testing.T) {
			imageName := fmt.Sprintf("aw-inttest-shell-%s", osTemplate)
			t.Cleanup(func() { removeImage(runtime, imageName) })

			t.Logf("preparing shell image for os=%s", osTemplate)
			buildDir, cleanup, err := PrepareBuildContext("", osTemplate, containerenv.Default())
			if err != nil {
				t.Fatalf("PrepareBuildContext: %v", err)
			}
			defer cleanup()

			buildImage(t, runtime, imageName, buildDir, nil)

			out := runInContainer(t, runtime, imageName, `
id -un
whoami
which git
which curl
which devbox
nix --version
echo "SHELL_OK"
`)
			if !strings.Contains(out, "agent") {
				t.Error("expected user 'agent'")
			}
			if !strings.Contains(out, "SHELL_OK") {
				t.Error("shell test did not complete")
			}

			containerID := runDetachedContainer(t, runtime, imageName, "sleep", "600")
			t.Cleanup(func() { removeContainer(runtime, containerID) })

			execUser := strings.TrimSpace(execInContainer(t, runtime, containerID, "id", "-un"))
			if execUser != "agent" {
				t.Errorf("exec user = %q, want %q", execUser, "agent")
			}

			loginOut := execInContainer(t, runtime, containerID, "bash", "-lc", `
command -v devbox >/dev/null
case ":$PATH:" in
  *":/home/agent/.local/share/mise/shims:"*) ;;
  *) echo PATH_MISSING; exit 1 ;;
esac
echo LOGIN_OK
`)
			if !strings.Contains(loginOut, "LOGIN_OK") {
				t.Error("bash -lc did not complete with expected PATH")
			}

			interactiveOut := execInContainer(t, runtime, containerID, "bash", "-ic", `
command -v devbox >/dev/null
case ":$PATH:" in
  *":/home/agent/.local/share/mise/shims:"*) ;;
  *) echo PATH_MISSING; exit 1 ;;
esac
echo INTERACTIVE_OK
`)
			if !strings.Contains(interactiveOut, "INTERACTIVE_OK") {
				t.Error("bash -ic did not complete with expected PATH")
			}

			rcOut := execInContainer(t, runtime, containerID, "bash", "-lc", `
test -f /home/agent/.aw_env.sh
grep -q '/home/agent/.aw_env.sh' /home/agent/.bashrc
grep -q '/home/agent/.bashrc' /home/agent/.bash_profile
echo RC_OK
`)
			if !strings.Contains(rcOut, "RC_OK") {
				t.Error("bash startup files were not wired as expected")
			}
		})
	}
}

func TestIntegration_ToolPerOS(t *testing.T) {
	runtime := detectRuntime()
	t.Logf("container runtime: %s", runtime)

	for _, osTemplate := range allOSTemplates {
		for _, tool := range integrationTools {
			pkg := toolinfo.DevboxPkg(tool)
			if pkg == "" {
				continue
			}

			testName := fmt.Sprintf("%s/%s", osTemplate, tool)
			t.Run(testName, func(t *testing.T) {
				imageName := fmt.Sprintf("aw-inttest-%s-%s", tool, osTemplate)
				t.Cleanup(func() { removeImage(runtime, imageName) })

				t.Logf("preparing tool image for os=%s tool=%s pkg=%s", osTemplate, tool, pkg)
				buildDir, cleanup, err := PrepareBuildContext("", osTemplate, containerenv.Default())
				if err != nil {
					t.Fatalf("PrepareBuildContext: %v", err)
				}
				defer cleanup()

				buildImage(t, runtime, imageName, buildDir, map[string]string{
					"AW_TOOL_PKG": pkg,
				})

				script := fmt.Sprintf("which %s && %s --version && echo TOOL_OK", tool, tool)
				out := runInContainer(t, runtime, imageName, script)

				if !strings.Contains(out, "TOOL_OK") {
					t.Errorf("%s --version did not succeed on %s", tool, osTemplate)
				}

				containerID := runDetachedContainer(t, runtime, imageName, "sleep", "600")
				t.Cleanup(func() { removeContainer(runtime, containerID) })

				loginOut := execInContainer(t, runtime, containerID, "bash", "-lc", fmt.Sprintf("command -v %s >/dev/null && echo TOOL_LOGIN_OK", tool))
				if !strings.Contains(loginOut, "TOOL_LOGIN_OK") {
					t.Errorf("%s was not available from bash -lc on %s", tool, osTemplate)
				}

				interactiveOut := execInContainer(t, runtime, containerID, "bash", "-ic", fmt.Sprintf("command -v %s >/dev/null && echo TOOL_INTERACTIVE_OK", tool))
				if !strings.Contains(interactiveOut, "TOOL_INTERACTIVE_OK") {
					t.Errorf("%s was not available from bash -ic on %s", tool, osTemplate)
				}
			})
		}
	}
}

// TestIntegration_Smoke is a quick sanity check using a single debian12 image.
// It verifies shell basics, the pre-installed tool (claude), and dynamically
// installs codex to confirm devbox-based tool installation works at runtime.
//
//	go test -v -tags integration -timeout 10m ./internal/image/ -run TestIntegration_Smoke
func TestIntegration_Smoke(t *testing.T) {
	runtime := detectRuntime()
	t.Logf("container runtime: %s", runtime)

	imageName := "aw-inttest-smoke"
	t.Cleanup(func() { removeImage(runtime, imageName) })

	buildDir, cleanup, err := PrepareBuildContext("", profile.OSDebian12, containerenv.Default())
	if err != nil {
		t.Fatalf("PrepareBuildContext: %v", err)
	}
	defer cleanup()

	buildImage(t, runtime, imageName, buildDir, map[string]string{
		"AW_TOOL_PKG": toolinfo.DevboxPkg("claude"),
	})

	containerID := runDetachedContainer(t, runtime, imageName, "sleep", "600")
	t.Cleanup(func() { removeContainer(runtime, containerID) })

	t.Run("shell", func(t *testing.T) {
		out := execInContainer(t, runtime, containerID, "bash", "-lc", `
id -un
which git
which curl
which devbox
case ":$PATH:" in
  *":/home/agent/.local/share/mise/shims:"*) ;;
  *) echo PATH_MISSING; exit 1 ;;
esac
test -f /home/agent/.aw_env.sh
grep -q '/home/agent/.aw_env.sh' /home/agent/.bashrc
grep -q '/home/agent/.bashrc' /home/agent/.bash_profile
echo SHELL_OK
`)
		if !strings.Contains(out, "agent") {
			t.Error("expected user 'agent'")
		}
		if !strings.Contains(out, "SHELL_OK") {
			t.Error("shell environment check did not complete")
		}
	})

	t.Run("claude", func(t *testing.T) {
		out := execInContainer(t, runtime, containerID, "bash", "-lc",
			"command -v claude >/dev/null && claude --version && echo CLAUDE_OK")
		if !strings.Contains(out, "CLAUDE_OK") {
			t.Error("claude was not available")
		}
	})

	t.Run("codex", func(t *testing.T) {
		execInContainer(t, runtime, containerID, "bash", "-lc",
			"devbox global add "+toolinfo.DevboxPkg("codex"))
		out := execInContainer(t, runtime, containerID, "bash", "-lc",
			"command -v codex >/dev/null && codex --version && echo CODEX_OK")
		if !strings.Contains(out, "CODEX_OK") {
			t.Error("codex was not available")
		}
	})
}

// TestIntegration_DevboxJSON verifies that an image with a user-provided
// devbox.json builds successfully and the declared package is installed.
// This catches regressions like missing directory creation before copying
// devbox.json into the devbox global config path.
func TestIntegration_DevboxJSON(t *testing.T) {
	runtime := detectRuntime()
	t.Logf("container runtime: %s", runtime)

	for _, osTemplate := range allOSTemplates {
		t.Run(string(osTemplate), func(t *testing.T) {
			imageName := fmt.Sprintf("aw-inttest-devbox-%s", osTemplate)
			t.Cleanup(func() { removeImage(runtime, imageName) })

			buildDir, cleanup, err := PrepareBuildContext("", osTemplate, containerenv.Default())
			if err != nil {
				t.Fatalf("PrepareBuildContext: %v", err)
			}
			defer cleanup()

			devboxJSON := []byte(`{"packages":["hello@latest"]}`)
			if err := os.WriteFile(filepath.Join(buildDir, "devbox.json"), devboxJSON, 0644); err != nil {
				t.Fatalf("writing devbox.json: %v", err)
			}

			buildImage(t, runtime, imageName, buildDir, nil)

			out := runInContainer(t, runtime, imageName, `
hello 2>&1 || true
devbox global list 2>/dev/null
echo DEVBOX_JSON_OK
`)
			if !strings.Contains(out, "DEVBOX_JSON_OK") {
				t.Error("devbox.json test did not complete")
			}
			if !strings.Contains(out, "hello") {
				t.Error("expected 'hello' package to be installed from devbox.json")
			}
		})
	}
}

// TestIntegration_DevboxAndMise builds an image with both devbox.json and
// mise.toml in the build context and verifies both tool managers work.
// This is an E2E test that catches permission errors in the Dockerfile
// templates — any root-owned directory that blocks agent-user writes will
// cause the build or the in-container checks to fail.
func TestIntegration_DevboxAndMise(t *testing.T) {
	runtime := detectRuntime()
	t.Logf("container runtime: %s", runtime)

	for _, osTemplate := range allOSTemplates {
		t.Run(string(osTemplate), func(t *testing.T) {
			imageName := fmt.Sprintf("aw-inttest-both-%s", osTemplate)
			t.Cleanup(func() { removeImage(runtime, imageName) })

			cenv := containerenv.Default()
			buildDir, cleanup, err := PrepareBuildContext("", osTemplate, cenv)
			if err != nil {
				t.Fatalf("PrepareBuildContext: %v", err)
			}
			defer cleanup()

			devboxJSON := []byte(`{"packages":["hello@latest"]}`)
			if err := os.WriteFile(filepath.Join(buildDir, "devbox.json"), devboxJSON, 0644); err != nil {
				t.Fatalf("writing devbox.json: %v", err)
			}

			miseToml := []byte("[tools]\ntiny = \"latest\"\n")
			if err := os.WriteFile(filepath.Join(buildDir, "mise.toml"), miseToml, 0644); err != nil {
				t.Fatalf("writing mise.toml: %v", err)
			}

			buildImage(t, runtime, imageName, buildDir, nil)

			out := runInContainer(t, runtime, imageName, fmt.Sprintf(`
hello 2>&1 || true
devbox global list 2>/dev/null
test -d %s && echo MISE_SHIMS_DIR_OK
test -w %s && echo LOCAL_SHARE_WRITABLE
echo DEVBOX_AND_MISE_OK
`, cenv.MiseShims(), cenv.Home+"/.local/share"))

			if !strings.Contains(out, "hello") {
				t.Error("expected 'hello' package from devbox.json")
			}
			if !strings.Contains(out, "MISE_SHIMS_DIR_OK") {
				t.Error("mise shims directory was not created")
			}
			if !strings.Contains(out, "LOCAL_SHARE_WRITABLE") {
				t.Error("~/.local/share is not writable by agent user")
			}
			if !strings.Contains(out, "DEVBOX_AND_MISE_OK") {
				t.Error("devbox+mise combined test did not complete")
			}
		})
	}
}

// TestIntegration_FilePermissions verifies that the agent user owns all
// directories it needs to write to at runtime. This catches the class of
// bugs where root-executed mkdir/cp in Dockerfile templates leaves
// directories that the agent user cannot modify.
func TestIntegration_FilePermissions(t *testing.T) {
	runtime := detectRuntime()
	t.Logf("container runtime: %s", runtime)

	for _, osTemplate := range allOSTemplates {
		t.Run(string(osTemplate), func(t *testing.T) {
			imageName := fmt.Sprintf("aw-inttest-perms-%s", osTemplate)
			t.Cleanup(func() { removeImage(runtime, imageName) })

			cenv := containerenv.Default()
			buildDir, cleanup, err := PrepareBuildContext("", osTemplate, cenv)
			if err != nil {
				t.Fatalf("PrepareBuildContext: %v", err)
			}
			defer cleanup()

			devboxJSON := []byte(`{"packages":["hello@latest"]}`)
			if err := os.WriteFile(filepath.Join(buildDir, "devbox.json"), devboxJSON, 0644); err != nil {
				t.Fatalf("writing devbox.json: %v", err)
			}

			miseToml := []byte("[tools]\ntiny = \"latest\"\n")
			if err := os.WriteFile(filepath.Join(buildDir, "mise.toml"), miseToml, 0644); err != nil {
				t.Fatalf("writing mise.toml: %v", err)
			}

			buildImage(t, runtime, imageName, buildDir, nil)

			writableDirs := []string{
				cenv.Home + "/.local/share",
				cenv.Home + "/.local/bin",
				cenv.Home + "/.config",
			}

			for _, dir := range writableDirs {
				t.Run(dir, func(t *testing.T) {
					out := runInContainer(t, runtime, imageName, fmt.Sprintf(`
owner=$(stat -c '%%U' %s 2>/dev/null || stat -f '%%Su' %s 2>/dev/null)
echo "OWNER=$owner"
test -w %s && echo WRITABLE || echo NOT_WRITABLE
`, dir, dir, dir))

					if !strings.Contains(out, "OWNER="+cenv.User) {
						t.Errorf("%s is not owned by %s: %s", dir, cenv.User, strings.TrimSpace(out))
					}
					if strings.Contains(out, "NOT_WRITABLE") {
						t.Errorf("%s is not writable by %s", dir, cenv.User)
					}
				})
			}
		})
	}
}

func TestIntegration_AllOSTemplatesHaveDockerfile(t *testing.T) {
	for _, os := range allOSTemplates {
		if _, ok := dockerfileTmpls[os]; !ok {
			t.Errorf("no embedded Dockerfile for OS template %q", os)
		}
	}
}
