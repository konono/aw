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

func detectRuntime() string {
	if r := os.Getenv("CONTAINER_RUNTIME"); r != "" {
		return r
	}
	if _, err := exec.LookPath("podman"); err == nil {
		return "podman"
	}
	return "docker"
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
	if err := cmd.Run(); err != nil {
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
	if err := cmd.Run(); err != nil {
		t.Fatalf("run in %s failed: %v\noutput: %s", imageName, err, out.String())
	}
	return out.String()
}

func removeImage(runtime, imageName string) {
	_ = exec.Command(runtime, "rmi", "-f", imageName).Run()
}

func TestIntegration_ShellPerOS(t *testing.T) {
	runtime := detectRuntime()

	for osTemplate := range dockerfiles {
		osTemplate := osTemplate
		t.Run(string(osTemplate), func(t *testing.T) {
			t.Parallel()
			imageName := fmt.Sprintf("aw-inttest-shell-%s", osTemplate)
			t.Cleanup(func() { removeImage(runtime, imageName) })

			buildDir, cleanup, err := PrepareBuildContext("", osTemplate)
			if err != nil {
				t.Fatalf("PrepareBuildContext: %v", err)
			}
			defer cleanup()

			buildImage(t, runtime, imageName, buildDir, nil)

			out := runInContainer(t, runtime, imageName, `
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
		})
	}
}

func TestIntegration_ToolPerOS(t *testing.T) {
	runtime := detectRuntime()

	tools := []string{"claude", "codex"}

	for osTemplate := range dockerfiles {
		for _, tool := range tools {
			osTemplate := osTemplate
			tool := tool
			pkg := toolinfo.DevboxPkg(tool)
			if pkg == "" {
				continue
			}

			testName := fmt.Sprintf("%s/%s", osTemplate, tool)
			t.Run(testName, func(t *testing.T) {
				t.Parallel()
				imageName := fmt.Sprintf("aw-inttest-%s-%s", tool, osTemplate)
				t.Cleanup(func() { removeImage(runtime, imageName) })

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
			})
		}
	}
}

func TestIntegration_AllOSTemplatesHaveDockerfile(t *testing.T) {
	allOS := []profile.OSTemplate{
		profile.OSDebian12,
		profile.OSUBI9,
		profile.OSUBI10,
		profile.OSUbuntu2604,
	}

	for _, os := range allOS {
		if _, ok := dockerfiles[os]; !ok {
			t.Errorf("no embedded Dockerfile for OS template %q", os)
		}
	}
}
