package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/konono/aw/internal/manifest"
	"github.com/konono/aw/internal/profile"
	"github.com/konono/aw/internal/stage"
	"github.com/konono/aw/internal/toolinfo"
)

// Run generates Kubernetes manifests for a profile.
func (m *ManifestCmd) Run() error {
	cfg, err := profile.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	p, ok := cfg.Profiles[m.ProfileName]
	if !ok {
		return fmt.Errorf("profile %q not found", m.ProfileName)
	}

	if p.Environment != profile.EnvironmentContainer {
		return fmt.Errorf("profile %q uses environment: %s (manifest requires environment: container)", m.ProfileName, p.Environment)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("getting home directory: %w", err)
	}

	imageName := m.Image
	if imageName == "" {
		imageName = resolveImageName(p)
	}

	opts := manifest.Options{
		Profile:      p,
		ProfileName:  m.ProfileName,
		InstanceName: m.Name,
		ImageName:    imageName,
		HomeDir:      homeDir,
	}

	resources, err := manifest.Generate(opts)
	if err != nil {
		return fmt.Errorf("generating manifests: %w", err)
	}

	output := manifest.RenderAll(resources)

	hasSecrets := false
	for _, r := range resources {
		if r.Kind == "Secret" {
			hasSecrets = true
			break
		}
	}
	if hasSecrets {
		fmt.Fprintln(os.Stderr, "Warning: generated manifests contain credentials from this host.")
		fmt.Fprintln(os.Stderr, "         Do not commit the Secret YAML to version control.")
	}

	if m.Output != "" {
		if err := os.MkdirAll(m.Output, 0755); err != nil {
			return fmt.Errorf("creating output directory: %w", err)
		}
		for _, r := range resources {
			filename := fmt.Sprintf("%s-%s.yaml", r.Name, r.Kind)
			path := filepath.Join(m.Output, filename)
			if err := os.WriteFile(path, r.YAML, 0644); err != nil {
				return fmt.Errorf("writing %s: %w", path, err)
			}
			fmt.Fprintf(os.Stderr, "Wrote %s\n", path)
		}
		return nil
	}

	_, err = os.Stdout.Write(output)
	return err
}

func resolveImageName(p profile.Profile) string {
	if p.Image != "" {
		return p.Image
	}

	tool := toolinfo.ImageTool(p.EffectiveTool())
	official := stage.OfficialImageName(tool, p.EffectiveOS())

	if p.Kubernetes != nil && p.Kubernetes.Registry != "" {
		// Replace the default registry with the user's registry.
		// OfficialImageName returns "ghcr.io/konono/aw-<tool>:<version>-<os>".
		// Extract the image name after the registry prefix.
		parts := strings.SplitN(official, "/", 3)
		if len(parts) == 3 {
			return p.Kubernetes.Registry + "/" + parts[2]
		}
	}

	return official
}
