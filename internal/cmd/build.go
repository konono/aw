package cmd

import (
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"text/template"

	"github.com/distribution/reference"
	"github.com/konono/aw/v4/internal/containerenv"
	"github.com/konono/aw/v4/internal/docker"
	"github.com/konono/aw/v4/internal/mise"
	"github.com/konono/aw/v4/internal/pipeline"
	"github.com/konono/aw/v4/internal/profile"
	"github.com/konono/aw/v4/internal/stage"
	"github.com/konono/aw/v4/internal/toolinfo"
	"gopkg.in/yaml.v3"
)

//go:embed embed/snapshot.sh.tmpl
var snapshotScriptTmpl string

// Run handles the build command.
func (b *BuildCmd) Run() error {
	includes, err := parseBuildIncludes(b.Include)
	if err != nil {
		return err
	}

	cfg, err := profile.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	p, ok := cfg.Profiles[b.ProfileName]
	if !ok {
		return fmt.Errorf("profile %q not found", b.ProfileName)
	}

	if p.Environment != profile.EnvironmentContainer {
		return fmt.Errorf("profile %q uses environment: %s (build requires environment: container)", b.ProfileName, p.Environment)
	}

	// aw run validates the whole config up front; build and manifest are
	// entered directly, so validate the one profile they act on. Doing it
	// per profile rather than config-wide keeps an unrelated broken profile
	// from blocking a build.
	if err := profile.Validate(p); err != nil {
		return fmt.Errorf("profile %q: %w", b.ProfileName, err)
	}

	prepareBuildProfile(&p, b.NoCache)

	ec, err := buildExecutionContext(b.ProfileName, p)
	if err != nil {
		return err
	}
	ec.NoCache = b.NoCache

	if len(b.BuildArg) > 0 {
		if ec.Profile.BuildEnv == nil {
			ec.Profile.BuildEnv = make(map[string]string)
		}
		for k, v := range b.BuildArg {
			ec.Profile.BuildEnv[k] = v
		}
	}

	prepareBuildImageSelection(ec)

	incl, envVars := mergeBuildFields(includes, b.Env, p.Build)
	workDir := ec.OrigWorkDir

	runtime := p.EffectiveContainerRuntime()
	client := docker.NewShellClient(runtime)

	// reusedPinnedImage records that this run produced nothing new: the profile
	// already pins the image aw would use. Writing the same name back would
	// only reformat the config through the YAML encoder, so it is left
	// untouched unless a later step (--push) renames the image.
	reusedPinnedImage := false
	snapshot := true
	var resultImage string

	if hasBuildInputs(ec, incl, envVars) {
		dockerStage := stage.NewDockerStage()
		if err := dockerStage.Run(context.Background(), ec); err != nil {
			return err
		}

		cenv := containerenv.FromUser(p.EffectiveContainerUser())
		if p.Kubernetes != nil && p.Kubernetes.SessionLog {
			cenv.SessionLog = true
		}

		commitImage := computeBuildImageName(b.ProfileName, ec.DockerImage, incl, envVars, workDir)
		if err := runSnapshot(client, ec, p, incl, envVars, cenv, commitImage); err != nil {
			return err
		}
		resultImage = commitImage
	} else {
		// Nothing to bake in, so there is no snapshot layer on the result.
		snapshot = false
		resultImage, reusedPinnedImage, err = b.imageWithoutBuildInputs(context.Background(), ec.Profile, client)
		if err != nil {
			return err
		}
		if resultImage == "" {
			return nil
		}
	}

	saveTar := b.Save != nil
	outputPath := ""
	if saveTar {
		outputPath = *b.Save
		if outputPath == "" {
			safe := strings.NewReplacer(":", "-", "/", "-").Replace(resultImage)
			outputPath = safe + ".tar"
		}
		fmt.Fprintf(os.Stderr, "Saving image '%s' to %s...\n", resultImage, outputPath)
		if err := client.Save(context.Background(), resultImage, outputPath); err != nil {
			return fmt.Errorf("saving image: %w", err)
		}
	}

	if b.Push {
		pushImage := replaceImageRegistry(resultImage, b.Registry)
		fmt.Fprintf(os.Stderr, "Tagging image '%s' as '%s'...\n", resultImage, pushImage)
		if err := client.Tag(context.Background(), resultImage, pushImage); err != nil {
			return fmt.Errorf("tagging image: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Pushing image '%s'...\n", pushImage)
		if err := client.Push(context.Background(), pushImage); err != nil {
			return fmt.Errorf("pushing image: %w", err)
		}
		resultImage = pushImage
		// The pushed reference is a new name, so it is worth recording.
		reusedPinnedImage = false
	}

	fmt.Fprintf(os.Stderr, "\nDone.\n\n")

	if b.Apply && !reusedPinnedImage {
		var targetFile string
		if hasWorkspaceFiles(workDir) {
			targetFile = profile.ProjectConfigPath()
		} else {
			targetFile = profile.FindProfileSource(b.ProfileName)
			if targetFile == "" {
				targetFile = cfg.Source.FilePath
			}
		}
		if targetFile == "" {
			return fmt.Errorf("writing the image name back needs a config file. Run `aw init` first, or pass --no-apply")
		}
		// aw build writes the image only. The entrypoint decides whether to
		// run mise install by comparing the fingerprint baked into the image
		// against the workspace, so pinning a toggle here would override a
		// decision aw can now make correctly on its own. An explicit
		// mise_install / skip_mise_install stays untouched as the user's
		// opt-out.
		if err := applyBuildResult(targetFile, b.ProfileName, resultImage, nil); err != nil {
			return fmt.Errorf("applying build result: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Applied image '%s' to profile '%s' in %s\n", resultImage, b.ProfileName, targetFile)
		if saveTar {
			fmt.Fprintf(os.Stderr, "# Load on target machine:\n")
			fmt.Fprintf(os.Stderr, "#   %s load -i %s\n", runtime, outputPath)
		}
	} else if saveTar {
		printConfigSnippet(resultImage, runtime, string(p.Launch), outputPath, snapshot)
	} else {
		fmt.Fprintf(os.Stderr, "Built image '%s'\n", resultImage)
	}

	return nil
}

// imageWithoutBuildInputs resolves the image to operate on when the profile has
// nothing to bake in. It returns the image name, whether that image is the one
// the profile already pins, and an empty name when there is nothing to do at
// all. --save, --push and the config write-back all run on the result, so the
// no-input path supports the same flags as a real build.
func (b *BuildCmd) imageWithoutBuildInputs(ctx context.Context, p profile.Profile, client docker.Client) (string, bool, error) {
	fmt.Fprintln(os.Stderr, "Warning: No build inputs found (no dockerfile, mise.toml, packages.txt, packages, ca_cert, --include, --env, --build-arg, or build_env).")

	// A pinned image is already the image aw would run. Replacing it with the
	// official one would throw away the user's choice, so reuse it instead.
	if p.Image != "" {
		fmt.Fprintf(os.Stderr, "  Profile pins image '%s'. Using it as-is.\n", p.Image)
		// --save and --push operate on the local image store, so the
		// reference has to be there. Nothing else in this branch touches the
		// image, and pulling for a no-op build would only cost time.
		if b.Save != nil || b.Push {
			if err := resolvePinnedImage(ctx, p, client); err != nil {
				return "", false, err
			}
		}
		return p.Image, true, nil
	}

	if !b.Apply && !b.Push && b.Save == nil {
		fmt.Fprintln(os.Stderr, "  The official image will be used as-is. Skipping build.")
		return "", false, nil
	}

	imageName := stage.OfficialImageName(toolinfo.ImageTool(p.EffectiveTool()), p.EffectiveOS())
	fmt.Fprintf(os.Stderr, "Pulling official image '%s'...\n", imageName)
	if err := client.Pull(ctx, imageName); err != nil {
		return "", false, fmt.Errorf("pulling official image: %w", err)
	}
	return imageName, false, nil
}

// resolvePinnedImage makes the profile's `image:` available in the local image
// store, pulling it when image_pull_policy allows.
//
// Unlike the launch path, which falls back to the official image so that an
// interactive session keeps working, a failure here is fatal: --save and
// --push name one specific image, and quietly substituting a different one
// would write the wrong thing to a tar or a registry.
func resolvePinnedImage(ctx context.Context, p profile.Profile, client docker.Client) error {
	imageName := p.Image
	policy := p.EffectiveImagePullPolicy()

	if policy != profile.ImagePullPolicyAlways {
		exists, err := client.ImageExists(ctx, imageName)
		if err != nil {
			return fmt.Errorf("checking image %q: %w", imageName, err)
		}
		if exists {
			fmt.Fprintf(os.Stderr, "  Using local image '%s'.\n", imageName)
			return nil
		}
		if policy == profile.ImagePullPolicyNever {
			return fmt.Errorf("image %q not found locally (image_pull_policy: never)", imageName)
		}
		if _, err := reference.ParseNormalizedNamed(imageName); err != nil {
			return fmt.Errorf("image %q not found locally and is not a valid image reference: %w", imageName, err)
		}
	}

	fmt.Fprintf(os.Stderr, "  Pulling image '%s'...\n", imageName)
	if err := client.Pull(ctx, imageName); err != nil {
		return fmt.Errorf("pulling image %q: %w", imageName, err)
	}
	return nil
}

var workspaceFileNames = []string{"mise.toml", ".mise.toml", "packages.txt"}

func hasWorkspaceFiles(dir string) bool {
	for _, name := range workspaceFileNames {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return true
		}
	}
	return false
}

func prepareBuildProfile(p *profile.Profile, fromTemplate bool) {
	if p.Dockerfile != "" || fromTemplate {
		p.Image = ""
	}
	p.SkipMiseInstall = nil
	p.MiseInstall = nil
	if fromTemplate {
		p.ImagePullPolicy = profile.ImagePullPolicyBuild
	}
}

// prepareBuildImageSelection keeps explicit builds on the template path for
// image customizations, including OS packages. Normal launches can install
// packages.txt through aw-init on an existing image instead.
func prepareBuildImageSelection(ec *pipeline.ExecutionContext) {
	if !stage.HasBuildCustomizations(ec) {
		return
	}
	ec.Profile.Image = ""
	ec.Profile.ImagePullPolicy = profile.ImagePullPolicyBuild
}

func hasBuildInputs(ec *pipeline.ExecutionContext, includes []profile.BuildInclude, envVars map[string]string) bool {
	if ec.Profile.Dockerfile != "" {
		return true
	}
	if hasWorkspaceFiles(ec.OrigWorkDir) {
		return true
	}
	if len(includes) > 0 || len(envVars) > 0 {
		return true
	}
	// Everything that forces a template build is a build input too. Deferring
	// to the same predicate the image resolution uses keeps ca_cert,
	// container_user and mount_zellij profiles from falling through to the
	// official image, which has none of those customizations.
	return stage.HasBuildCustomizations(ec)
}

var commitBaseChanges = []string{
	`ENTRYPOINT ["/entrypoint.sh"]`,
	`CMD ["bash"]`,
}

var tagUnsafe = regexp.MustCompile(`[^a-zA-Z0-9._-]`)

func computeBuildImageName(profileName, baseImage string, includes []profile.BuildInclude, envVars map[string]string, workDir string) string {
	hashInput := baseImage + "\n" + profileName
	for _, inc := range includes {
		hashInput += "\ninclude:" + inc.Src + ":" + inc.Dst
	}
	for _, k := range slices.Sorted(maps.Keys(envVars)) {
		hashInput += "\nenv:" + k + "=" + envVars[k]
	}
	for _, name := range workspaceFileNames {
		if data, err := os.ReadFile(filepath.Join(workDir, name)); err == nil {
			hashInput += "\nworkspace:" + name + ":" + string(data)
		}
	}
	hash := sha256.Sum256([]byte(hashInput))
	safe := tagUnsafe.ReplaceAllString(profileName, "-")
	return fmt.Sprintf("aw-build:%s-%x", safe, hash[:6])
}

func runSnapshot(client docker.Client, ec *pipeline.ExecutionContext, p profile.Profile, includes []profile.BuildInclude, envVars map[string]string, cenv containerenv.Config, commitImage string) error {
	fmt.Fprintf(os.Stderr, "Snapshotting image '%s'...\n", ec.DockerImage)

	// Empty when the workspace has mise inputs the snapshot does not copy, so
	// the image goes without a fingerprint and the entrypoint keeps installing.
	miseFingerprint, _ := mise.Fingerprint(ec.OrigWorkDir)

	script, err := renderSnapshotScript(cenv, miseFingerprint)
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

	// Snapshot bind mounts bypass mount.go's :z labeling; spc_t avoids
	// SELinux AVC denials (especially for home directory mounts on Podman).
	rc.SecurityOpts = append(rc.SecurityOpts, "label=type:spc_t")

	rc.Mounts = append(rc.Mounts, docker.Mount{
		Source:   ec.OrigWorkDir,
		Target:   cenv.Workspace,
		ReadOnly: true,
	})

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

	changes := append([]string{}, commitBaseChanges...)
	for _, k := range slices.Sorted(maps.Keys(envVars)) {
		changes = append(changes, fmt.Sprintf("ENV %s=%s", k, envVars[k]))
	}

	if err := client.Commit(ctx, containerID, commitImage, changes); err != nil {
		_ = client.RemoveContainer(ctx, containerID)
		return fmt.Errorf("committing snapshot: %w", err)
	}

	_ = client.RemoveContainer(ctx, containerID)
	return nil
}

func mergeBuildFields(flagIncludes []profile.BuildInclude, flagEnv map[string]string, profileBuild *profile.BuildConfig) (includes []profile.BuildInclude, envVars map[string]string) {
	if profileBuild != nil {
		includes = append(includes, profileBuild.Include...)
		if len(profileBuild.Env) > 0 {
			envVars = make(map[string]string, len(profileBuild.Env))
			for k, v := range profileBuild.Env {
				envVars[k] = v
			}
		}
	}

	includes = append(includes, flagIncludes...)
	for k, v := range flagEnv {
		if envVars == nil {
			envVars = make(map[string]string)
		}
		envVars[k] = v
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
		// The tar already carries the tools, and an air-gapped target cannot
		// reinstall them, so suggest opting out of the startup install there.
		fmt.Fprintf(os.Stderr, "#       mise_install: false\n")
	}
}

// applyBuildResult writes imageName into the named profile.
//
// skipMiseInstall controls the deprecated entrypoint install toggle: nil leaves
// whatever the profile already has. aw build passes nil because it no longer
// manages the toggle; aw save passes a value because the container it commits
// has its tools installed already.
func applyBuildResult(configPath, profileName, imageName string, skipMiseInstall *bool) error {
	data, err := os.ReadFile(configPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading config file: %w", err)
	}
	if data == nil {
		data = []byte("{}\n")
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("parsing config file: %w", err)
	}

	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return fmt.Errorf("config file has unexpected structure")
	}

	root := doc.Content[0]
	root.Style = 0
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
	if skipMiseInstall != nil {
		setMiseInstallToggle(root, targetProfile, *skipMiseInstall)
	}
	// Drop the key left behind by the removed devbox package manager.
	removeYAMLMapKey(targetProfile, "skip_devbox_install")

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

// setMiseInstallToggle records whether the entrypoint should skip mise install.
//
// New writes use mise_install; skip_mise_install is kept readable but is no
// longer introduced. A profile that already carries the deprecated key keeps
// using it, because the two are mutually exclusive (profile.Validate) and
// adding the new key beside the old one would break the next aw command.
func setMiseInstallToggle(root, targetProfile *yaml.Node, skip bool) {
	if findYAMLMapValue(targetProfile, "skip_mise_install") != nil ||
		findYAMLMapValue(root, "skip_mise_install") != nil {
		setYAMLMapBool(targetProfile, "skip_mise_install", skip)
		return
	}
	setYAMLMapBool(targetProfile, "mise_install", !skip)
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

func removeYAMLMapKey(mapping *yaml.Node, key string) {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			mapping.Content = append(mapping.Content[:i], mapping.Content[i+2:]...)
			return
		}
	}
}

// snapshotData is the template input for snapshot.sh.tmpl: the container's own
// layout plus the mise fingerprint to bake into the image. An empty
// MiseFingerprint leaves the image without one, which makes the entrypoint
// install at every launch.
type snapshotData struct {
	containerenv.Config
	MiseFingerprint string
}

func renderSnapshotScript(cenv containerenv.Config, miseFingerprint string) (string, error) {
	tmpl := strings.ReplaceAll(snapshotScriptTmpl, "\r", "")
	t, err := template.New("snapshot").Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("parsing snapshot script template: %w", err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, snapshotData{Config: cenv, MiseFingerprint: miseFingerprint}); err != nil {
		return "", fmt.Errorf("rendering snapshot script: %w", err)
	}
	return buf.String(), nil
}
