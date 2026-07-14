package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/distribution/reference"
	"github.com/konono/aw/v4/internal/manifest"
	"github.com/konono/aw/v4/internal/profile"
	"github.com/konono/aw/v4/internal/stage"
	"github.com/konono/aw/v4/internal/toolinfo"
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
		var err error
		imageName, err = resolveImageName(p)
		if err != nil {
			return err
		}
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

func resolveImageName(p profile.Profile) (string, error) {
	if p.Image != "" {
		return p.Image, nil
	}

	tool := toolinfo.ImageTool(p.EffectiveTool())
	official := stage.OfficialImageName(tool, p.EffectiveOS())

	if p.Kubernetes != nil && p.Kubernetes.SessionLog {
		return "", fmt.Errorf("session_log is enabled but no custom image is set.\n" +
			"  The official image does not include pty-logger.\n" +
			"  Run 'aw build --from-template' first, then set 'image:' in the profile or use '--image'")
	}

	if p.Kubernetes != nil && p.Kubernetes.Registry != "" {
		return replaceImageRegistry(official, p.Kubernetes.Registry), nil
	}

	return official, nil
}

// replaceImageRegistry replaces the registry prefix of an image reference.
// e.g. "ghcr.io/konono/aw-claude:v1-debian12" with registry "ghcr.io/myorg"
// becomes "ghcr.io/myorg/aw-claude:v1-debian12".
func replaceImageRegistry(imageName, registry string) string {
	ref, err := reference.ParseNormalizedNamed(imageName)
	if err != nil {
		return registry + "/" + imageName
	}
	refPath := reference.Path(ref)
	if idx := strings.LastIndex(refPath, "/"); idx >= 0 {
		refPath = refPath[idx+1:]
	}
	result := registry + "/" + refPath
	if tagged, ok := ref.(reference.Tagged); ok {
		result += ":" + tagged.Tag()
	}
	return result
}
