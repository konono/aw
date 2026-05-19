//go:build integration

package image

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

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
			buildDir, cleanup, err := PrepareBuildContext("", osTemplate)
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
				buildDir, cleanup, err := PrepareBuildContext("", osTemplate)
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

	buildDir, cleanup, err := PrepareBuildContext("", profile.OSDebian12)
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

func TestIntegration_AllOSTemplatesHaveDockerfile(t *testing.T) {
	for _, os := range allOSTemplates {
		if _, ok := dockerfiles[os]; !ok {
			t.Errorf("no embedded Dockerfile for OS template %q", os)
		}
	}
}
