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
	"github.com/konono/aw/v4/internal/team"
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

	imageName := s.ImageName
	if imageName == "" {
		imageName = computeSaveImageName(profileName)
	}

	fmt.Fprintf(os.Stderr, "Committing container '%s' as '%s'...\n", entry.Name, imageName)
	if err := client.Commit(ctx, entry.Name, imageName, commitBaseChanges); err != nil {
		return fmt.Errorf("committing container: %w", err)
	}

	pkgMgr := detectPackageManager(workspace, profileName)
	configPath, err := resolveConfigPath(workspace)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: image '%s' was created but could not determine config path: %v\n", imageName, err)
		return err
	}
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
	teamNames := teamContainerNames()
	var result []containerEntry
	for _, e := range entries {
		if snapshotNameRe.MatchString(e.Name) {
			continue
		}
		if teamNames[e.Name] {
			continue
		}
		result = append(result, e)
	}
	return result
}

func teamContainerNames() map[string]bool {
	states, err := team.ListStates()
	if err != nil {
		return nil
	}
	names := make(map[string]bool)
	for _, s := range states {
		for _, m := range s.Members {
			if m.ContainerName != "" {
				names[m.ContainerName] = true
			}
		}
	}
	return names
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

func detectPackageManager(workspace, profileName string) profile.PackageManager {
	origDir, err := os.Getwd()
	if err != nil {
		return profile.PackageManagerApt
	}
	if err := os.Chdir(workspace); err != nil {
		return profile.PackageManagerApt
	}
	defer func() { _ = os.Chdir(origDir) }()

	cfg, err := profile.LoadQuiet()
	if err != nil {
		return profile.PackageManagerApt
	}
	p, ok := cfg.Profiles[profileName]
	if !ok {
		return profile.PackageManagerApt
	}
	return p.EffectivePackageManager()
}
