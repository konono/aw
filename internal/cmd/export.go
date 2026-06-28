package cmd

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/konono/aw/internal/containerenv"
	"github.com/konono/aw/internal/docker"
	"github.com/konono/aw/internal/pipeline"
	"github.com/konono/aw/internal/profile"
	"github.com/konono/aw/internal/stage"
	"gopkg.in/yaml.v3"
)

//go:embed embed/snapshot.sh.tmpl
var snapshotScriptTmpl string

// Run handles the export command.
func (e *ExportCmd) Run() error {
	includes, err := parseExportIncludes(e.Include)
	if err != nil {
		return err
	}

	cfg, err := profile.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	p, ok := cfg.Profiles[e.ProfileName]
	if !ok {
		return fmt.Errorf("profile %q not found", e.ProfileName)
	}

	if p.Environment != profile.EnvironmentContainer {
		return fmt.Errorf("profile %q uses environment: %s (export requires environment: container)", e.ProfileName, p.Environment)
	}

	p.Image = ""

	ec, err := buildExecutionContext(e.ProfileName, p)
	if err != nil {
		return err
	}
	ec.NoCache = e.NoCache

	dockerStage := stage.NewDockerStage()
	if err := dockerStage.Run(context.Background(), ec); err != nil {
		return err
	}

	snapshot, incl, envVars := mergeExportFields(e.Snapshot, includes, e.Env, p.Export)

	runtime := p.EffectiveContainerRuntime()
	client := docker.NewShellClient(runtime)

	cenv := containerenv.FromUser(p.EffectiveContainerUser())

	if snapshot {
		if err := runSnapshot(client, ec, p, incl, envVars, cenv); err != nil {
			return err
		}
	}

	saveTar := !e.Apply || e.Output != ""

	outputPath := e.Output
	if saveTar && outputPath == "" {
		safe := strings.NewReplacer(":", "-", "/", "-").Replace(ec.DockerImage)
		outputPath = safe + ".tar"
	}

	if saveTar {
		fmt.Fprintf(os.Stderr, "Exporting image '%s' to %s...\n", ec.DockerImage, outputPath)
		if err := client.Save(context.Background(), ec.DockerImage, outputPath); err != nil {
			return fmt.Errorf("saving image: %w", err)
		}
	}

	fmt.Fprintf(os.Stderr, "\nDone.\n\n")

	if e.Apply {
		targetFile := profile.FindProfileSource(e.ProfileName)
		if targetFile == "" {
			targetFile = cfg.Source.FilePath
		}
		if targetFile == "" {
			return fmt.Errorf("--apply requires a config file. Run `aw init` first")
		}
		if err := applyExportResult(targetFile, e.ProfileName, ec.DockerImage, snapshot); err != nil {
			return fmt.Errorf("applying export result: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Applied image '%s' to profile '%s' in %s\n", ec.DockerImage, e.ProfileName, targetFile)
		if saveTar {
			fmt.Fprintf(os.Stderr, "# Load on target machine:\n")
			fmt.Fprintf(os.Stderr, "#   %s load -i %s\n", runtime, outputPath)
		}
	} else {
		printConfigSnippet(ec.DockerImage, runtime, string(p.Launch), outputPath, snapshot)
	}

	return nil
}

func runSnapshot(client docker.Client, ec *pipeline.ExecutionContext, p profile.Profile, includes []profile.ExportInclude, envVars map[string]string, cenv containerenv.Config) error {
	fmt.Fprintf(os.Stderr, "Snapshotting image '%s'...\n", ec.DockerImage)

	script, err := renderSnapshotScript(cenv)
	if err != nil {
		return err
	}

	userns := ""
	if p.EffectiveContainerRuntime() == "podman" {
		userns = "keep-id"
	}

	rc := docker.RunConfig{
		ImageName:  ec.DockerImage,
		Entrypoint: "/bin/bash",
		Command:    []string{"-c", script},
		EnvVars:    make(map[string]string),
		GroupAdd:   docker.RootGroupAdd(),
		User:       docker.HostUserID(),
		Userns:     userns,
	}

	rc.Mounts = append(rc.Mounts, docker.Mount{
		Source:   ec.OrigWorkDir,
		Target:   cenv.Workspace,
		ReadOnly: true,
	})

	// skip_*_install flags are for the runtime entrypoint only, not for snapshot.
	for i, inc := range includes {
		absSrc, err := filepath.Abs(inc.Src)
		if err != nil {
			return fmt.Errorf("resolving include path %q: %w", inc.Src, err)
		}
		if _, err := os.Stat(absSrc); err != nil {
			return fmt.Errorf("include source %q does not exist: %w", inc.Src, err)
		}
		target := fmt.Sprintf("/tmp/aw-include-%d", i)
		rc.Mounts = append(rc.Mounts, docker.Mount{
			Source:   absSrc,
			Target:   target,
			ReadOnly: true,
		})
		rc.EnvVars[fmt.Sprintf("AW_INCLUDE_%d_DST", i)] = inc.Dst
	}

	ctx := context.Background()
	containerID, err := client.RunOneShot(ctx, rc)
	if err != nil {
		_ = client.RemoveContainer(ctx, containerID)
		return fmt.Errorf("snapshot container failed: %w", err)
	}

	changes := []string{
		`ENTRYPOINT ["/entrypoint.sh"]`,
		`CMD ["bash"]`,
	}
	for k, v := range envVars {
		changes = append(changes, fmt.Sprintf("ENV %s=%s", k, v))
	}

	if err := client.Commit(ctx, containerID, ec.DockerImage, changes); err != nil {
		_ = client.RemoveContainer(ctx, containerID)
		return fmt.Errorf("committing snapshot: %w", err)
	}

	_ = client.RemoveContainer(ctx, containerID)
	return nil
}

func mergeExportFields(flagSnapshot bool, flagIncludes []profile.ExportInclude, flagEnv map[string]string, profileExport *profile.ExportConfig) (snapshot bool, includes []profile.ExportInclude, envVars map[string]string) {
	if profileExport != nil {
		snapshot = profileExport.Snapshot
		includes = append(includes, profileExport.Include...)
		if len(profileExport.Env) > 0 {
			envVars = make(map[string]string, len(profileExport.Env))
			for k, v := range profileExport.Env {
				envVars[k] = v
			}
		}
	}

	if flagSnapshot {
		snapshot = true
	}
	includes = append(includes, flagIncludes...)
	for k, v := range flagEnv {
		if envVars == nil {
			envVars = make(map[string]string)
		}
		envVars[k] = v
	}

	if len(includes) > 0 || len(envVars) > 0 {
		snapshot = true
	}

	return
}

func printConfigSnippet(imageName, runtime, launch, tarPath string, snapshot bool) {
	fmt.Fprintf(os.Stderr, "# Load on target machine:\n")
	fmt.Fprintf(os.Stderr, "#   %s load -i %s\n", runtime, tarPath)
	fmt.Fprintf(os.Stderr, "#\n")
	fmt.Fprintf(os.Stderr, "# Add to ~/.config/aw/config.yml:\n")
	fmt.Fprintf(os.Stderr, "#   profiles:\n")
	fmt.Fprintf(os.Stderr, "#     airgap:\n")
	fmt.Fprintf(os.Stderr, "#       environment: container\n")
	fmt.Fprintf(os.Stderr, "#       launch: %s\n", launch)
	fmt.Fprintf(os.Stderr, "#       image: '%s'\n", imageName)
	if snapshot {
		fmt.Fprintf(os.Stderr, "#       skip_devbox_install: true\n")
		fmt.Fprintf(os.Stderr, "#       skip_mise_install: true\n")
	} else {
		fmt.Fprintf(os.Stderr, "#       # For offline/air-gapped environments, add:\n")
		fmt.Fprintf(os.Stderr, "#       # skip_devbox_install: true\n")
		fmt.Fprintf(os.Stderr, "#       # skip_mise_install: true\n")
	}
}

func applyExportResult(configPath, profileName, imageName string, snapshot bool) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("reading config file: %w", err)
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("parsing config file: %w", err)
	}

	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return fmt.Errorf("config file has unexpected structure")
	}

	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return fmt.Errorf("config root is not a mapping")
	}

	profileNode := findYAMLMapValue(root, "profiles")
	if profileNode == nil {
		profileNode = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		root.Content = append(root.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "profiles", Tag: "!!str"},
			profileNode,
		)
	}

	targetProfile := findYAMLMapValue(profileNode, profileName)
	if targetProfile == nil {
		targetProfile = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		profileNode.Content = append(profileNode.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: profileName, Tag: "!!str"},
			targetProfile,
		)
	}

	setYAMLMapValue(targetProfile, "image", imageName)
	setYAMLMapBool(targetProfile, "skip_devbox_install", snapshot)
	setYAMLMapBool(targetProfile, "skip_mise_install", snapshot)

	var yamlBuf bytes.Buffer
	enc := yaml.NewEncoder(&yamlBuf)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}
	_ = enc.Close()

	if err := os.WriteFile(configPath, yamlBuf.Bytes(), 0644); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}

	return nil
}

func findYAMLMapValue(mapping *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1]
		}
	}
	return nil
}

func setYAMLMapValue(mapping *yaml.Node, key, value string) {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			mapping.Content[i+1].Value = value
			mapping.Content[i+1].Tag = "!!str"
			mapping.Content[i+1].Style = 0
			return
		}
	}
	mapping.Content = append(mapping.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: key, Tag: "!!str"},
		&yaml.Node{Kind: yaml.ScalarNode, Value: value, Tag: "!!str"},
	)
}

func setYAMLMapBool(mapping *yaml.Node, key string, value bool) {
	v := "false"
	if value {
		v = "true"
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			mapping.Content[i+1].Value = v
			mapping.Content[i+1].Tag = "!!bool"
			mapping.Content[i+1].Style = 0
			return
		}
	}
	mapping.Content = append(mapping.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: key, Tag: "!!str"},
		&yaml.Node{Kind: yaml.ScalarNode, Value: v, Tag: "!!bool"},
	)
}

func renderSnapshotScript(cenv containerenv.Config) (string, error) {
	tmpl := strings.ReplaceAll(snapshotScriptTmpl, "\r", "")
	t, err := template.New("snapshot").Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("parsing snapshot script template: %w", err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, cenv); err != nil {
		return "", fmt.Errorf("rendering snapshot script: %w", err)
	}
	return buf.String(), nil
}
