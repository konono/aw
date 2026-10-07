package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/konono/aw/v4/internal/docker"
	"github.com/konono/aw/v4/internal/picker"
	"github.com/konono/aw/v4/internal/platform"
	"github.com/konono/aw/v4/internal/profile"
)

type containerEntry struct {
	docker.ContainerInfo
	Runtime string
}

func (s *SaveCmd) Run() error {
	runtimes, err := detectRuntimes(s.Runtime)
	if err != nil {
		return err
	}

	ctx := context.Background()
	entries, err := listAllAwContainers(ctx, runtimes)
	if err != nil {
		return err
	}

	entries = filterUserEntries(entries)
	if len(entries) == 0 {
		return fmt.Errorf("no aw containers found (running or recently stopped)")
	}

	items := make([]string, len(entries))
	entryMap := make(map[string]*containerEntry, len(entries))
	for i, e := range entries {
		profileName, _ := extractProfileName(e.Name)
		if profileName == "" {
			profileName = "?"
		}
		status := formatStatus(e.Status)
		items[i] = fmt.Sprintf("%s  (%s)  [%s]", e.Name, profileName, status)
		entryMap[items[i]] = &entries[i]
	}

	selected, err := picker.Pick(items, picker.Options{Prompt: "container> "})
	if err != nil {
		if errors.Is(err, picker.ErrCancelled) {
			return nil
		}
		return err
	}

	entry := entryMap[selected]
	if entry == nil {
		return fmt.Errorf("unexpected picker result")
	}

	profileName, err := extractProfileName(entry.Name)
	if err != nil {
		return err
	}

	client := docker.NewShellClient(entry.Runtime)

	rawWorkspace, err := client.InspectContainerEnv(ctx, entry.Name, "HOST_WORKSPACE")
	if err != nil {
		return fmt.Errorf("cannot determine workspace directory for container %q (HOST_WORKSPACE not set)", entry.Name)
	}
	workspace := platform.FromContainerPath(rawWorkspace)

	if _, err := os.Stat(workspace); err != nil {
		return fmt.Errorf("workspace directory %q does not exist on this host", workspace)
	}

	// Resolve the profile against the config as seen from the container's own
	// workspace before committing anything. A container name only yields a
	// candidate profile name; leftover containers from the removed "aw team"
	// command are named aw-<team>-<agent>-<n> and would otherwise resolve to a
	// bogus "<team>-<agent>" profile that gets written into the config.
	cfg, err := loadWorkspaceConfig(workspace)
	if err != nil {
		return fmt.Errorf("loading config for workspace %q: %w", workspace, err)
	}
	p, err := resolveSaveProfile(cfg, entry.Name, profileName, workspace, entry.Runtime)
	if err != nil {
		return err
	}

	configPath, err := resolveConfigPath(workspace)
	if err != nil {
		return fmt.Errorf("could not determine config path for workspace %q: %w", workspace, err)
	}

	imageName := s.ImageName
	if imageName == "" {
		imageName = computeSaveImageName(profileName)
	}

	fmt.Fprintf(os.Stderr, "Committing container '%s' as '%s'...\n", entry.Name, imageName)
	if err := client.Commit(ctx, entry.Name, imageName, commitBaseChanges); err != nil {
		return fmt.Errorf("committing container: %w", err)
	}

	pkgMgr := p.EffectivePackageManager()
	if err := applyBuildResult(configPath, profileName, imageName, pkgMgr, true); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: image '%s' was created but config update failed.\n", imageName)
		return fmt.Errorf("writing config: %w", err)
	}

	fmt.Fprintf(os.Stderr, "\nDone.\n")
	fmt.Fprintf(os.Stderr, "  Image: %s\n", imageName)
	fmt.Fprintf(os.Stderr, "  Config: %s\n", configPath)
	fmt.Fprintf(os.Stderr, "\nNext 'aw %s' from %s will use the saved image.\n", profileName, workspace)
	return nil
}

func detectRuntimes(explicit string) ([]string, error) {
	if explicit != "" {
		if explicit != "docker" && explicit != "podman" {
			return nil, fmt.Errorf("unsupported runtime %q (use docker or podman)", explicit)
		}
		return []string{explicit}, nil
	}
	var found []string
	for _, rt := range []string{"docker", "podman"} {
		if _, err := exec.LookPath(rt); err == nil {
			found = append(found, rt)
		}
	}
	if len(found) == 0 {
		return nil, fmt.Errorf("no container runtime found; install docker or podman, or use --runtime")
	}
	return found, nil
}

func listAllAwContainers(ctx context.Context, runtimes []string) ([]containerEntry, error) {
	seen := make(map[string]bool)
	var entries []containerEntry
	var lastErr error
	for _, rt := range runtimes {
		client := docker.NewShellClient(rt)
		containers, err := client.ListAwContainers(ctx)
		if err != nil {
			lastErr = fmt.Errorf("%s: %w", rt, err)
			continue
		}
		for _, c := range containers {
			if seen[c.Name] {
				continue
			}
			seen[c.Name] = true
			entries = append(entries, containerEntry{ContainerInfo: c, Runtime: rt})
		}
	}
	if len(entries) == 0 && lastErr != nil {
		return nil, fmt.Errorf("listing containers: %w", lastErr)
	}
	return entries, nil
}

var profileNameRe = regexp.MustCompile(`^aw-(.+)-[0-9]+$`)

func extractProfileName(containerName string) (string, error) {
	m := profileNameRe.FindStringSubmatch(containerName)
	if m == nil {
		return "", fmt.Errorf("cannot extract profile name from container %q", containerName)
	}
	return m[1], nil
}

func computeSaveImageName(profileName string) string {
	ts := time.Now().Format("20060102-150405")
	safe := tagUnsafe.ReplaceAllString(profileName, "-")
	return fmt.Sprintf("aw-save:%s-%s", safe, ts)
}

func formatStatus(status string) string {
	lower := strings.ToLower(status)
	if strings.HasPrefix(lower, "up") {
		return "running"
	}
	if strings.HasPrefix(lower, "exited") {
		return "exited"
	}
	return status
}

var snapshotNameRe = regexp.MustCompile(`^aw-snapshot-`)

func filterUserEntries(entries []containerEntry) []containerEntry {
	var result []containerEntry
	for _, e := range entries {
		if snapshotNameRe.MatchString(e.Name) {
			continue
		}
		result = append(result, e)
	}
	return result
}

func resolveConfigPath(workspace string) (string, error) {
	origDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getting current directory: %w", err)
	}
	if err := os.Chdir(workspace); err != nil {
		return "", fmt.Errorf("changing to workspace %q: %w", workspace, err)
	}
	defer func() { _ = os.Chdir(origDir) }()
	path := profile.ProjectConfigPath()
	if path == "" {
		return "", fmt.Errorf("could not determine config path for workspace %q", workspace)
	}
	return path, nil
}

// resolveSaveProfile looks up the profile a container was launched from.
// The profile name extracted from a container name is only a candidate: any
// container matching aw-<something>-<digits> is listed, including leftovers
// from the removed "aw team" command (aw-<team>-<agent>-<n>), which would
// resolve to a bogus "<team>-<agent>" profile and get written into the config.
func resolveSaveProfile(cfg *profile.Config, containerName, profileName, workspace, runtime string) (profile.Profile, error) {
	p, ok := cfg.Profiles[profileName]
	if !ok {
		return profile.Profile{}, fmt.Errorf("container %q does not belong to a known profile: %q is not defined in the config for %s\n"+
			"If this is a leftover container from the removed 'aw team' command, remove it with '%s rm -f %s'",
			containerName, profileName, workspace, runtime, containerName)
	}
	return p, nil
}

// loadWorkspaceConfig loads the merged config (builtin -> user -> project) as
// seen from the container's workspace directory, not from the directory aw was
// invoked in.
func loadWorkspaceConfig(workspace string) (*profile.Config, error) {
	origDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("getting current directory: %w", err)
	}
	if err := os.Chdir(workspace); err != nil {
		return nil, fmt.Errorf("changing to workspace %q: %w", workspace, err)
	}
	defer func() { _ = os.Chdir(origDir) }()
	return profile.LoadQuiet()
}
