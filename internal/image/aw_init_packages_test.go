package image

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Exercise the package installer from the embedded init script with fake OS
// package commands. This covers the actual startup branch without touching the
// host's package database.
func TestInitScript_InstallsOnlyMissingOSPackages(t *testing.T) {
	src := string(InitScript())
	start := strings.Index(src, "# Extra package installation\n")
	end := strings.Index(src, "# Tool config symlinks\n")
	if start < 0 || end <= start {
		t.Fatal("could not locate the OS package installer in aw-init.sh")
	}
	block := src[start:end]

	for _, tt := range []struct {
		name      string
		packages  string
		want      string
		forbid    string
		wantEmpty bool
		fail      string
	}{
		{name: "debian installed and held", packages: "installed,held", wantEmpty: true},
		{name: "debian removed and missing", packages: "installed,removed,missing", want: "apt-get install -y --no-install-recommends removed missing"},
		{name: "debian update failure stops startup", packages: "missing", fail: "apt-update", want: "apt-get update -qq", forbid: "apt-get install"},
		{name: "debian install failure stops startup", packages: "missing", fail: "apt-install", want: "apt-get install -y --no-install-recommends missing"},
		{name: "ubi installed", packages: "installed", wantEmpty: true},
		{name: "ubi missing", packages: "installed,missing", want: "dnf install -y missing"},
		{name: "ubi install failure stops startup", packages: "missing", fail: "dnf-install", want: "dnf install -y missing"},
		{name: "unsupported OS stops startup", packages: "missing", fail: "no-manager", wantEmpty: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			logPath := filepath.Join(dir, "commands.log")
			writeCommand := func(name, body string) {
				t.Helper()
				if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/bash\n"+body), 0755); err != nil {
					t.Fatal(err)
				}
			}
			writeCommand("sudo", "case \"$1\" in apt-get|dnf) \"$@\" ;; esac\n")
			writeCommand("apt-get", "echo \"apt-get $*\" >> \"$AW_TEST_LOG\"\nif [ \"$AW_TEST_FAIL\" = \"apt-$1\" ]; then exit 42; fi\n")
			writeCommand("dnf", "echo \"dnf $*\" >> \"$AW_TEST_LOG\"\nif [ \"$AW_TEST_FAIL\" = \"dnf-$1\" ]; then exit 42; fi\n")
			writeCommand("dpkg-query", "case \"${@: -1}\" in installed) echo 'install ok installed';; held) echo 'hold ok installed';; removed) echo 'deinstall ok config-files';; *) exit 1;; esac\n")
			writeCommand("rpm", "[ \"$2\" = installed ]\n")

			prefix := "set -e\naw_log() { echo \"$1\" >&2; }\n"
			if strings.HasPrefix(tt.name, "ubi") {
				// Simulate a UBI image even when tests run on a Debian host.
				prefix += "command() { if [ \"$1\" = -v ] && [ \"$2\" = apt-get ]; then return 1; fi; builtin command \"$@\"; }\n"
			} else if tt.fail == "no-manager" {
				prefix += "command() { if [ \"$1\" = -v ] && { [ \"$2\" = apt-get ] || [ \"$2\" = dnf ]; }; then return 1; fi; builtin command \"$@\"; }\n"
			}
			cmd := exec.Command("bash", "-c", prefix+block)
			cmd.Env = append(os.Environ(),
				"PATH="+dir+":"+os.Getenv("PATH"),
				"AW_PACKAGES="+tt.packages,
				"AW_TEST_LOG="+logPath,
				"AW_TEST_FAIL="+tt.fail,
			)
			out, runErr := cmd.CombinedOutput()
			if tt.fail != "" && runErr == nil {
				t.Fatalf("package installer should stop startup after %s failure:\n%s", tt.fail, out)
			}
			if tt.fail == "" && runErr != nil {
				t.Fatalf("package installer failed: %v\n%s", runErr, out)
			}
			if tt.fail == "no-manager" {
				for _, message := range []string{"missing", "Dockerfile", "packages.txt", "packages:"} {
					if !strings.Contains(string(out), message) {
						t.Errorf("unsupported OS error should include %q, got:\n%s", message, out)
					}
				}
			}
			log, err := os.ReadFile(logPath)
			if err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
			if tt.wantEmpty && len(log) != 0 {
				t.Errorf("package manager should not run for installed packages, got:\n%s", log)
			}
			if tt.want != "" && !strings.Contains(string(log), tt.want) {
				t.Errorf("missing %q in commands:\n%s", tt.want, log)
			}
			if tt.forbid != "" && strings.Contains(string(log), tt.forbid) {
				t.Errorf("unexpected %q in commands:\n%s", tt.forbid, log)
			}
		})
	}
}
