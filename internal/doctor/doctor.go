package doctor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"context"

	"github.com/konono/aw/internal/docker"
	"github.com/konono/aw/internal/mount"
	"github.com/konono/aw/internal/pathutil"
	"github.com/konono/aw/internal/profile"
	"github.com/konono/aw/internal/reaper"
	"github.com/konono/aw/internal/stage"
	"github.com/konono/aw/internal/version"
)

type result struct {
	errors int
}

func (r *result) pass(msg string) {
	fmt.Printf("  ✓ %s\n", msg)
}

func (r *result) fail(msg string) {
	fmt.Printf("  ✗ %s\n", msg)
	r.errors++
}

func (r *result) detail(msg string) {
	fmt.Printf("    %s\n", msg)
}

// RunDiagnostics executes environment diagnostics and returns an exit code.
func RunDiagnostics(verbose bool) int {
	res := &result{}

	cfg := checkConfig(res, verbose)
	if cfg == nil {
		fmt.Println()
		fmt.Printf("%d error found.\n", res.errors)
		return 1
	}

	runtimes := collectRuntimes(cfg)
	runtimeOK := checkRuntimes(res, runtimes, verbose)

	needGH, needMountGH := collectGHNeeds(cfg)
	ghOK := checkGitHub(res, needGH, needMountGH, verbose)

	needSSH, needAgent := collectSSHNeeds(cfg)
	agentOK := checkSSH(res, needSSH, needAgent, verbose)

	needContainerSock := collectContainerSockNeeds(cfg)
	sockOK := checkContainerSock(res, needContainerSock, runtimes, verbose)

	checkProfiles(res, cfg, runtimeOK, ghOK, agentOK, sockOK, verbose)
	checkOfficialImages(res, cfg, runtimeOK, verbose)
	checkReaper(res)

	if verbose {
		printSystemInfo()
	}

	fmt.Println()
	if res.errors == 0 {
		fmt.Println("All checks passed.")
		return 0
	}
	if res.errors == 1 {
		fmt.Println("1 error found. Some profiles may not work.")
	} else {
		fmt.Printf("%d errors found. Some profiles may not work.\n", res.errors)
	}
	return 1
}

func checkConfig(res *result, verbose bool) *profile.Config {
	fmt.Println("Config")

	cfg, err := profile.Load()
	if err != nil {
		res.fail(fmt.Sprintf("Failed to load config: %v", err))
		return nil
	}

	source := "built-in default"
	if !cfg.Source.IsBuiltin {
		source = cfg.Source.FilePath
	}
	res.pass(fmt.Sprintf("Loaded from %s", source))
	if verbose && !cfg.Source.IsBuiltin {
		res.detail(fmt.Sprintf("search: %s (found)", cfg.Source.FilePath))
	}

	defaultInfo := ""
	if cfg.Default != "" {
		defaultInfo = fmt.Sprintf(" (default: %s)", cfg.Default)
	}
	res.pass(fmt.Sprintf("%d profiles found%s", len(cfg.Profiles), defaultInfo))

	return cfg
}

func collectRuntimes(cfg *profile.Config) map[string]bool {
	runtimes := map[string]bool{}
	for _, p := range cfg.Profiles {
		if p.Environment == profile.EnvironmentContainer || p.Environment == "" {
			runtimes[p.EffectiveContainerRuntime()] = true
		}
	}
	return runtimes
}

func checkRuntimes(res *result, runtimes map[string]bool, verbose bool) map[string]bool {
	if len(runtimes) == 0 {
		return nil
	}

	fmt.Println()
	fmt.Println("Container Runtime")

	ok := map[string]bool{}
	for rt := range runtimes {
		client := docker.NewShellClient(rt)
		if err := client.CheckAvailable(); err != nil {
			res.fail(fmt.Sprintf("%s: %v", rt, err))
			if verbose {
				path, lookErr := exec.LookPath(rt)
				if lookErr != nil {
					res.detail(fmt.Sprintf("which %s: not found", rt))
					res.detail(fmt.Sprintf("PATH=%s", os.Getenv("PATH")))
				} else {
					res.detail(fmt.Sprintf("which %s: %s", rt, path))
					res.detail(fmt.Sprintf("%s info: failed", rt))
				}
				if rt == "podman" {
					res.detail("hint: install podman — https://podman.io/docs/installation")
				} else {
					res.detail("hint: install docker — https://docs.docker.com/get-docker/")
				}
			}
			ok[rt] = false
		} else {
			res.pass(fmt.Sprintf("%s is available and running", rt))
			if verbose {
				path, _ := exec.LookPath(rt)
				res.detail(fmt.Sprintf("which %s: %s", rt, path))
				out, _ := exec.Command(rt, "--version").Output()
				res.detail(fmt.Sprintf("%s --version: %s", rt, strings.TrimSpace(string(out))))
			}
			ok[rt] = true
		}
	}
	return ok
}

func collectGHNeeds(cfg *profile.Config) (needGHToken, needMountGH bool) {
	for _, p := range cfg.Profiles {
		if p.EffectiveGhToken() {
			needGHToken = true
		}
		if p.EffectiveMountGH() {
			needMountGH = true
		}
	}
	return
}

func checkGitHub(res *result, needGHToken, needMountGH bool, verbose bool) bool {
	if !needGHToken && !needMountGH {
		return true
	}

	fmt.Println()
	fmt.Println("GitHub")

	ok := true

	if needGHToken {
		ghPath, err := exec.LookPath("gh")
		if err != nil {
			res.fail("gh CLI not found")
			if verbose {
				res.detail("which gh: not found")
				res.detail("hint: install gh — https://cli.github.com/")
			}
			ok = false
		} else {
			res.pass("gh CLI found")
			if verbose {
				res.detail(fmt.Sprintf("which gh: %s", ghPath))
				out, _ := exec.Command("gh", "--version").Output()
				res.detail(fmt.Sprintf("gh --version: %s", strings.TrimSpace(strings.Split(string(out), "\n")[0])))
			}

			token, err := stage.DetectGhToken()
			if err != nil {
				res.fail("Authentication failed")
				if verbose {
					res.detail(fmt.Sprintf("gh auth token: %v", err))
					res.detail("hint: run 'gh auth login' to authenticate")
				}
				ok = false
			} else {
				scopeInfo := checkTokenScopes(token, verbose)
				res.pass(fmt.Sprintf("Authenticated%s", scopeInfo))
			}
		}
	}

	if needMountGH {
		homeDir, _ := os.UserHomeDir()
		ghDir := filepath.Join(homeDir, ".config", "gh")
		if info, err := os.Stat(ghDir); err != nil || !info.IsDir() {
			res.fail("~/.config/gh directory not found (mount_gh)")
			if verbose {
				res.detail(fmt.Sprintf("stat %s: %v", ghDir, err))
			}
			ok = false
		} else {
			res.pass("~/.config/gh directory exists")
			if verbose {
				res.detail(fmt.Sprintf("stat %s: directory", ghDir))
			}
		}
	}

	return ok
}

func checkTokenScopes(token string, verbose bool) string {
	cmd := exec.Command("curl", "-sI", "-H", fmt.Sprintf("Authorization: token %s", token), "https://api.github.com/")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(strings.ToLower(line), "x-oauth-scopes:") {
			scopes := strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
			if scopes != "" {
				return fmt.Sprintf(" (scopes: %s)", scopes)
			}
		}
	}
	return ""
}

func collectSSHNeeds(cfg *profile.Config) (needSSH, needAgent bool) {
	for _, p := range cfg.Profiles {
		if p.EffectiveMountSSH() {
			needSSH = true
		}
		if p.EffectiveSSHAgentForwarding() {
			needAgent = true
		}
	}
	return
}

func checkSSH(res *result, needSSH, needAgent bool, verbose bool) bool {
	if !needSSH && !needAgent {
		return true
	}

	fmt.Println()
	fmt.Println("SSH")

	ok := true

	if needSSH {
		homeDir, _ := os.UserHomeDir()
		sshDir := filepath.Join(homeDir, ".ssh")
		if info, err := os.Stat(sshDir); err != nil || !info.IsDir() {
			res.fail("~/.ssh directory not found (mount_ssh)")
			if verbose {
				res.detail(fmt.Sprintf("stat %s: %v", sshDir, err))
			}
			ok = false
		} else {
			res.pass("~/.ssh directory exists")
			if verbose {
				res.detail(fmt.Sprintf("stat %s: directory", sshDir))
			}
		}
	}

	if needAgent {
		sock := os.Getenv("SSH_AUTH_SOCK")
		if sock == "" {
			res.fail("SSH_AUTH_SOCK is not set (ssh_agent_forwarding)")
			if verbose {
				res.detail("env SSH_AUTH_SOCK: (empty)")
				if runtime.GOOS == "windows" {
					res.detail("hint: set SSH_AUTH_SOCK in Git Bash, or use Docker Desktop")
				} else {
					res.detail("hint: start ssh-agent — eval \"$(ssh-agent -s)\"")
				}
			}
			ok = false
		} else {
			if runtime.GOOS != "windows" {
				if _, err := os.Stat(sock); err != nil {
					res.fail(fmt.Sprintf("SSH_AUTH_SOCK=%s (socket not found)", sock))
					if verbose {
						res.detail(fmt.Sprintf("stat %s: %v", sock, err))
					}
					ok = false
				} else {
					res.pass("SSH_AUTH_SOCK is set")
					if verbose {
						res.detail(fmt.Sprintf("env SSH_AUTH_SOCK: %s", sock))
					}
				}
			} else {
				res.pass("SSH_AUTH_SOCK is set")
				if verbose {
					res.detail(fmt.Sprintf("env SSH_AUTH_SOCK: %s", sock))
				}
			}
		}
	}

	return ok
}

func collectContainerSockNeeds(cfg *profile.Config) bool {
	for _, p := range cfg.Profiles {
		if p.EffectiveMountContainerSock() {
			return true
		}
	}
	return false
}

func checkContainerSock(res *result, needed bool, runtimes map[string]bool, verbose bool) bool {
	if !needed {
		return true
	}

	fmt.Println()
	fmt.Println("Container Socket")

	for rt := range runtimes {
		sockPath, err := mount.DetectContainerSock(rt)
		if err != nil {
			res.fail(fmt.Sprintf("Socket not found for %s", rt))
			if verbose {
				res.detail(fmt.Sprintf("detect %s socket: %v", rt, err))
			}
			return false
		}
		res.pass(fmt.Sprintf("Socket detected at %s", sockPath))
		if verbose {
			if info, err := os.Stat(sockPath); err == nil {
				res.detail(fmt.Sprintf("stat %s: %s", sockPath, info.Mode()))
			}
		}
	}
	return true
}

type profileIssue struct {
	name   string
	issues []string
}

func checkProfiles(res *result, cfg *profile.Config, runtimeOK map[string]bool, ghOK, agentOK, sockOK bool, verbose bool) {
	fmt.Println()
	fmt.Println("Profiles")

	passed := 0
	total := len(cfg.Profiles)
	var failed []profileIssue

	homeDir, _ := os.UserHomeDir()

	for name, p := range cfg.Profiles {
		var issues []string

		if err := profile.Validate(p); err != nil {
			issues = append(issues, err.Error())
		}

		if p.Environment == profile.EnvironmentContainer || p.Environment == "" {
			rt := p.EffectiveContainerRuntime()
			if runtimeOK != nil && !runtimeOK[rt] {
				issues = append(issues, fmt.Sprintf("container runtime %q is not available", rt))
			}
		}

		if p.EffectiveGhToken() && !ghOK {
			issues = append(issues, "gh authentication failed")
		}

		if p.EffectiveSSHAgentForwarding() && !agentOK {
			issues = append(issues, "SSH_AUTH_SOCK is not set")
		}

		if p.EffectiveMountContainerSock() && !sockOK {
			issues = append(issues, "container socket not found")
		}

		for _, m := range p.Mounts {
			src := pathutil.ExpandTilde(m.Source, homeDir)
			if _, err := os.Stat(src); err != nil {
				issues = append(issues, fmt.Sprintf("mount source %s does not exist", m.Source))
			}
		}

		if len(issues) == 0 {
			passed++
			if verbose {
				desc := profile.Describe(p)
				features := profile.FeatureFlags(p)
				if features != "" {
					desc += " + " + features
				}
				res.pass(fmt.Sprintf("%-16s %s", name, desc))
			}
		} else {
			failed = append(failed, profileIssue{name: name, issues: issues})
		}
	}

	type issueGroup struct {
		key    string
		issues []string
		names  []string
	}
	var groups []issueGroup
	seen := map[string]int{}

	for _, f := range failed {
		key := strings.Join(f.issues, "\n")
		if idx, ok := seen[key]; ok {
			groups[idx].names = append(groups[idx].names, f.name)
		} else {
			seen[key] = len(groups)
			groups = append(groups, issueGroup{key: key, issues: f.issues, names: []string{f.name}})
		}
	}

	for _, g := range groups {
		if len(g.names) == 1 {
			res.fail(g.names[0])
		} else {
			res.fail(strings.Join(g.names, ", "))
		}
		for _, issue := range g.issues {
			res.detail(fmt.Sprintf("→ %s", issue))
		}
	}

	if passed > 0 {
		fmt.Printf("  ✓ %d/%d profiles OK\n", passed, total)
	}
}

func checkOfficialImages(res *result, cfg *profile.Config, runtimeOK map[string]bool, verbose bool) {
	tools := map[string]bool{}
	for _, p := range cfg.Profiles {
		if p.Environment != profile.EnvironmentContainer {
			continue
		}
		tool := p.EffectiveTool()
		if tool == "" || tools[tool] {
			continue
		}
		tools[tool] = true
	}
	if len(tools) == 0 {
		return
	}

	var rt string
	for r, ok := range runtimeOK {
		if ok {
			rt = r
			break
		}
	}
	if rt == "" {
		return
	}

	fmt.Println()
	fmt.Println("Official Images")

	client := docker.NewShellClient(rt)
	for tool := range tools {
		imageName := fmt.Sprintf("ghcr.io/konono/aw-%s:%s-%s", tool, version.Version, "debian12")
		exists, err := client.ImageExists(context.Background(), imageName)
		if err != nil {
			if verbose {
				res.detail(fmt.Sprintf("%s: check failed (%v)", tool, err))
			}
			continue
		}
		if exists {
			res.pass(fmt.Sprintf("%s: %s (local)", tool, imageName))
		} else {
			if verbose {
				res.detail(fmt.Sprintf("%s: %s not cached locally", tool, imageName))
				res.detail("  → will be pulled on first use (image_pull_policy: auto)")
			} else {
				res.pass(fmt.Sprintf("%s: will be pulled on first use", tool))
			}
		}
	}
}

func checkReaper(res *result) {
	fmt.Println()
	fmt.Println("Reaper")

	dir := reaper.ReaperDir()

	// Check for orphan spec files (skip active sessions)
	specs, _ := filepath.Glob(filepath.Join(dir, "*.spec.json"))
	var active, orphaned []string
	for _, s := range specs {
		name := strings.TrimSuffix(filepath.Base(s), ".spec.json")
		rt := reaper.RuntimeFromSpec(s)
		out, err := exec.Command(rt, "inspect", name,
			"--format", "{{.State.Running}}").Output()
		if err == nil && strings.TrimSpace(string(out)) == "true" {
			active = append(active, filepath.Base(s))
		} else {
			orphaned = append(orphaned, filepath.Base(s))
		}
	}
	if len(active) > 0 {
		res.pass(fmt.Sprintf("%d active session(s)", len(active)))
		for _, s := range active {
			res.detail(fmt.Sprintf("→ %s", s))
		}
	}
	if len(orphaned) > 0 {
		res.fail(fmt.Sprintf("%d orphaned reaper spec(s) found", len(orphaned)))
		for _, s := range orphaned {
			res.detail(fmt.Sprintf("→ %s", s))
		}
		res.detail("run: aw reaper recover <name> or aw reaper discard <name>")
		return
	}

	// Check recent reports for abnormal exits
	reports := reaper.ListReports()
	if len(reports) == 0 {
		res.pass("no issues (no reports)")
		return
	}

	start := 0
	if len(reports) > reaper.DoctorReportLookback {
		start = len(reports) - reaper.DoctorReportLookback
	}

	var abnormal int
	var latestSummary string
	for _, path := range reports[start:] {
		report, err := reaper.ReadReport(path)
		if err != nil {
			continue
		}
		if report.ExitCode == 0 {
			continue
		}
		abnormal++
		latestSummary = fmt.Sprintf("exit %d", report.ExitCode)
		if report.ContainerDiag != nil && report.ContainerDiag.Summary != "" {
			latestSummary = report.ContainerDiag.Summary
		}
	}

	if abnormal == 0 {
		res.pass("no issues")
		return
	}

	res.fail(fmt.Sprintf("%d recent session(s) exited abnormally (latest: %s)", abnormal, latestSummary))
	res.detail("→ aw reaper show")
}

func printSystemInfo() {
	fmt.Println()
	fmt.Println("System Info")
	fmt.Printf("  os: %s (%s)\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("  go: %s\n", runtime.Version())
	fmt.Printf("  aw: %s\n", version.Version)
	if home, err := os.UserHomeDir(); err == nil {
		fmt.Printf("  home: %s\n", home)
	}
	if cwd, err := os.Getwd(); err == nil {
		fmt.Printf("  cwd: %s\n", cwd)
	}
}

