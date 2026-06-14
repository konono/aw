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

func detectGitHubToken() string {
	if t := os.Getenv("GITHUB_TOKEN"); t != "" {
		return t
	}
	out, err := exec.Command("gh", "auth", "token").Output()
	if err == nil {
		return strings.TrimSpace(string(out))
	}
	return ""
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

func runDetachedContainer(t *testing.T, runtime, imageName string, command ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	args := []string{"run", "--rm", "-d"}
	if token := detectGitHubToken(); token != "" {
		args = append(args, "-e", fmt.Sprintf("GITHUB_TOKEN=%s", token))
	}
	args = append(args, imageName)
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

// runContainerCommand runs a command through the image's entrypoint, just
// like `aw <profile> -c <command...>` in production.
func runContainerCommand(t *testing.T, runtime, imageName string, command ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	args := []string{"run", "--rm"}
	if token := detectGitHubToken(); token != "" {
		args = append(args, "-e", fmt.Sprintf("GITHUB_TOKEN=%s", token))
	}
	args = append(args, imageName)
	args = append(args, command...)
	cmd := exec.CommandContext(ctx, runtime, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := runCommandWithProgress(t, cmd, fmt.Sprintf("run %s %s", imageName, strings.Join(command, " "))); err != nil {
		t.Fatalf("run %s %v failed: %v\noutput: %s", imageName, command, err, out.String())
	}
	return out.String()
}

// toolBuildArgs returns the Docker build args for installing a tool.
func toolBuildArgs(tool string, pkgMgr profile.PackageManager) map[string]string {
	args := map[string]string{}
	if pkgMgr == profile.PackageManagerDevbox {
		if pkg := toolinfo.DevboxPkg(tool); pkg != "" {
			args["AW_TOOL_PKG"] = pkg
		}
	} else {
		if script := toolinfo.InstallScript(tool); script != "" {
			args["AW_TOOL_INSTALL_SCRIPT"] = script
		}
	}
	return args
}

// =============================================================================
// apt mode tests (default)
// =============================================================================

// TestIntegration_ShellPerOS verifies that each OS template produces a working
// shell environment with the expected user, PATH, and startup files.
//
//	go test -v -tags integration -timeout 30m ./internal/image/ -run TestIntegration_ShellPerOS
func TestIntegration_ShellPerOS(t *testing.T) {
	runtime := detectRuntime()
	t.Logf("container runtime: %s", runtime)

	for _, osTemplate := range allOSTemplates {
		t.Run(string(osTemplate), func(t *testing.T) {
			imageName := fmt.Sprintf("aw-inttest-shell-%s", osTemplate)
			t.Cleanup(func() { removeImage(runtime, imageName) })

			buildDir, cleanup, err := PrepareBuildContext("", osTemplate, profile.PackageManagerApt, containerenv.Default())
			if err != nil {
				t.Fatalf("PrepareBuildContext: %v", err)
			}
			defer cleanup()

			buildImage(t, runtime, imageName, buildDir, nil)

			// Verify basic shell via entrypoint (-c pattern)
			out := runContainerCommand(t, runtime, imageName, "bash", "-c", `
id -un
which git
which curl
echo SHELL_OK
`)
			if !strings.Contains(out, "agent") {
				t.Error("expected user 'agent'")
			}
			if !strings.Contains(out, "SHELL_OK") {
				t.Error("shell test did not complete")
			}

			// Verify login shell PATH and startup files
			out = runContainerCommand(t, runtime, imageName, "bash", "-c", `
case ":$PATH:" in
  *":/home/agent/.local/share/mise/shims:"*) ;;
  *) echo PATH_MISSING; exit 1 ;;
esac
test -f /home/agent/.aw_env.sh
grep -q '/home/agent/.aw_env.sh' /home/agent/.bashrc
grep -q '/home/agent/.bashrc' /home/agent/.bash_profile
echo RC_OK
`)
			if !strings.Contains(out, "RC_OK") {
				t.Error("startup files were not wired as expected")
			}
		})
	}
}

// TestIntegration_ToolPerOS verifies that each tool installs and runs on each
// OS template. Uses runContainerCommand to test through the entrypoint, just
// like `aw <profile> -c <tool> --version`.
//
//	go test -v -tags integration -timeout 30m ./internal/image/ -run TestIntegration_ToolPerOS
func TestIntegration_ToolPerOS(t *testing.T) {
	runtime := detectRuntime()
	t.Logf("container runtime: %s", runtime)

	for _, osTemplate := range allOSTemplates {
		for _, tool := range integrationTools {
			testName := fmt.Sprintf("%s/%s", osTemplate, tool)
			t.Run(testName, func(t *testing.T) {
				imageName := fmt.Sprintf("aw-inttest-%s-%s", tool, osTemplate)
				t.Cleanup(func() { removeImage(runtime, imageName) })

				buildDir, cleanup, err := PrepareBuildContext("", osTemplate, profile.PackageManagerApt, containerenv.Default())
				if err != nil {
					t.Fatalf("PrepareBuildContext: %v", err)
				}
				defer cleanup()

				buildArgs := toolBuildArgs(tool, profile.PackageManagerApt)
				t.Logf("building %s on %s with args: %v", tool, osTemplate, buildArgs)
				buildImage(t, runtime, imageName, buildDir, buildArgs)

				// Test tool via entrypoint (like `aw <profile> -c <tool> --version`)
				out := runContainerCommand(t, runtime, imageName, tool, "--version")
				if !strings.Contains(strings.ToLower(out), tool) {
					t.Errorf("%s --version did not produce expected output on %s:\n%s", tool, osTemplate, out)
				}
			})
		}
	}
}

// TestIntegration_Smoke is a quick sanity check using a single debian12 image.
// It verifies the pre-installed tool (claude) works and that standalone tools
// can be installed at runtime.
//
//	go test -v -tags integration -timeout 10m ./internal/image/ -run TestIntegration_Smoke
func TestIntegration_Smoke(t *testing.T) {
	runtime := detectRuntime()
	t.Logf("container runtime: %s", runtime)

	imageName := "aw-inttest-smoke"
	t.Cleanup(func() { removeImage(runtime, imageName) })

	buildDir, cleanup, err := PrepareBuildContext("", profile.OSDebian12, profile.PackageManagerApt, containerenv.Default())
	if err != nil {
		t.Fatalf("PrepareBuildContext: %v", err)
	}
	defer cleanup()

	buildImage(t, runtime, imageName, buildDir, toolBuildArgs("claude", profile.PackageManagerApt))

	// Test tool launch via entrypoint
	t.Run("claude", func(t *testing.T) {
		out := runContainerCommand(t, runtime, imageName, "claude", "--version")
		if !strings.Contains(strings.ToLower(out), "claude") {
			t.Errorf("claude --version failed:\n%s", out)
		}
	})

	// Test runtime standalone install (mise)
	t.Run("runtime_standalone_install", func(t *testing.T) {
		containerID := runDetachedContainer(t, runtime, imageName, "sleep", "600")
		t.Cleanup(func() { removeContainer(runtime, containerID) })

		execInContainer(t, runtime, containerID, "bash", "-lc",
			"curl -fsSL https://mise.jdx.dev/install.sh | sh")
		out := execInContainer(t, runtime, containerID, "bash", "-lc",
			"mise --version")
		if out == "" {
			t.Error("runtime standalone install: mise --version returned empty output")
		}
	})
}

// TestIntegration_E2E replicates the real `aw <tool>` flow: build an image with
// mise.toml and a tool package, then launch the tool through entrypoint.sh.
//
//	go test -v -tags integration -timeout 30m ./internal/image/ -run TestIntegration_E2E
func TestIntegration_E2E(t *testing.T) {
	runtime := detectRuntime()
	t.Logf("container runtime: %s", runtime)

	for _, osTemplate := range allOSTemplates {
		t.Run(string(osTemplate), func(t *testing.T) {
			imageName := fmt.Sprintf("aw-inttest-e2e-%s", osTemplate)
			t.Cleanup(func() { removeImage(runtime, imageName) })

			cenv := containerenv.Default()
			buildDir, cleanup, err := PrepareBuildContext("", osTemplate, profile.PackageManagerApt, cenv)
			if err != nil {
				t.Fatalf("PrepareBuildContext: %v", err)
			}
			defer cleanup()

			miseToml := []byte("[tools]\njq = \"latest\"\n")
			if err := os.WriteFile(filepath.Join(buildDir, "mise.toml"), miseToml, 0644); err != nil {
				t.Fatalf("writing mise.toml: %v", err)
			}

			buildImage(t, runtime, imageName, buildDir, toolBuildArgs("claude", profile.PackageManagerApt))

			// Tool launch via entrypoint (like `aw claude -c claude --version`)
			t.Run("tool_launch", func(t *testing.T) {
				out := runContainerCommand(t, runtime, imageName, "claude", "--version")
				if !strings.Contains(strings.ToLower(out), "claude") {
					t.Errorf("claude --version did not produce expected output:\n%s", out)
				}
			})

			// mise.toml tool via entrypoint
			t.Run("mise_tool", func(t *testing.T) {
				out := runContainerCommand(t, runtime, imageName, "jq", "--version")
				if !strings.Contains(out, "jq") {
					t.Errorf("jq --version (mise.toml) failed:\n%s", out)
				}
			})

			// Runtime tool install via mise
			t.Run("runtime_install", func(t *testing.T) {
				containerID := runDetachedContainer(t, runtime, imageName, "sleep", "600")
				t.Cleanup(func() { removeContainer(runtime, containerID) })

				execInContainer(t, runtime, containerID, "bash", "-lc",
					"MISE_YES=1 mise install fd@latest && mise use -g fd@latest")
				out := execInContainer(t, runtime, containerID, "bash", "-lc",
					"fd --version")
				if !strings.Contains(out, "fd") {
					t.Errorf("fd --version after runtime mise install failed:\n%s", out)
				}
			})
		})
	}
}

// =============================================================================
// devbox mode tests (deprecated)
// =============================================================================

// TestIntegration_Devbox_ShellPerOS verifies that devbox mode still produces
// working images with Nix and devbox available.
//
//	go test -v -tags integration -timeout 30m ./internal/image/ -run TestIntegration_Devbox_ShellPerOS
func TestIntegration_Devbox_ShellPerOS(t *testing.T) {
	runtime := detectRuntime()
	t.Logf("container runtime: %s", runtime)

	for _, osTemplate := range allOSTemplates {
		t.Run(string(osTemplate), func(t *testing.T) {
			imageName := fmt.Sprintf("aw-inttest-devbox-shell-%s", osTemplate)
			t.Cleanup(func() { removeImage(runtime, imageName) })

			buildDir, cleanup, err := PrepareBuildContext("", osTemplate, profile.PackageManagerDevbox, containerenv.Default())
			if err != nil {
				t.Fatalf("PrepareBuildContext: %v", err)
			}
			defer cleanup()

			buildImage(t, runtime, imageName, buildDir, nil)

			out := runContainerCommand(t, runtime, imageName, "bash", "-c", `
id -un
which devbox
nix --version
echo DEVBOX_SHELL_OK
`)
			if !strings.Contains(out, "agent") {
				t.Error("expected user 'agent'")
			}
			if !strings.Contains(out, "DEVBOX_SHELL_OK") {
				t.Error("devbox shell test did not complete")
			}
		})
	}
}

// TestIntegration_Devbox_E2E verifies the devbox mode E2E flow: build with
// devbox.json, install a tool via devbox, verify it works.
//
//	go test -v -tags integration -timeout 30m ./internal/image/ -run TestIntegration_Devbox_E2E
func TestIntegration_Devbox_E2E(t *testing.T) {
	runtime := detectRuntime()
	t.Logf("container runtime: %s", runtime)

	for _, osTemplate := range allOSTemplates {
		t.Run(string(osTemplate), func(t *testing.T) {
			imageName := fmt.Sprintf("aw-inttest-devbox-e2e-%s", osTemplate)
			t.Cleanup(func() { removeImage(runtime, imageName) })

			cenv := containerenv.Default()
			buildDir, cleanup, err := PrepareBuildContext("", osTemplate, profile.PackageManagerDevbox, cenv)
			if err != nil {
				t.Fatalf("PrepareBuildContext: %v", err)
			}
			defer cleanup()

			devboxJSON := []byte(`{"packages":["hello@latest"]}`)
			if err := os.WriteFile(filepath.Join(buildDir, "devbox.json"), devboxJSON, 0644); err != nil {
				t.Fatalf("writing devbox.json: %v", err)
			}

			buildImage(t, runtime, imageName, buildDir, toolBuildArgs("claude", profile.PackageManagerDevbox))

			// Tool launch via entrypoint
			t.Run("tool_launch", func(t *testing.T) {
				out := runContainerCommand(t, runtime, imageName, "claude", "--version")
				if !strings.Contains(strings.ToLower(out), "claude") {
					t.Errorf("claude --version did not produce expected output:\n%s", out)
				}
			})

			// devbox.json package
			t.Run("devbox_json_package", func(t *testing.T) {
				out := runContainerCommand(t, runtime, imageName, "hello")
				if !strings.Contains(out, "Hello") {
					t.Errorf("hello from devbox.json failed:\n%s", out)
				}
			})

			// Runtime devbox install
			t.Run("runtime_devbox_install", func(t *testing.T) {
				containerID := runDetachedContainer(t, runtime, imageName, "sleep", "600")
				t.Cleanup(func() { removeContainer(runtime, containerID) })

				execInContainer(t, runtime, containerID, "bash", "-lc",
					"devbox global add "+toolinfo.DevboxPkg("codex"))
				out := execInContainer(t, runtime, containerID, "bash", "-lc",
					"codex --version")
				if !strings.Contains(out, "codex") {
					t.Errorf("codex --version after devbox install failed:\n%s", out)
				}
			})
		})
	}
}

func TestIntegration_AllOSTemplatesHaveDockerfile(t *testing.T) {
	for _, os := range allOSTemplates {
		if _, ok := dockerfileTmpls[os]; !ok {
			t.Errorf("no apt Dockerfile for OS template %q", os)
		}
		if _, ok := dockerfileDevboxTmpls[os]; !ok {
			t.Errorf("no devbox Dockerfile for OS template %q", os)
		}
	}
}
