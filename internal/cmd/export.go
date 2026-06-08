package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/konono/aw/internal/docker"
	"github.com/konono/aw/internal/pipeline"
	"github.com/konono/aw/internal/profile"
	"github.com/konono/aw/internal/stage"
)

type exportOptions struct {
	ProfileName string
	OutputPath  string
	Snapshot    bool
	Include     []profile.ExportInclude
	Env         map[string]string
}

var errExportHelp = errors.New("export help requested")

func runExport(args []string) int {
	opts, err := parseExportArgs(args)
	if err != nil {
		if errors.Is(err, errExportHelp) {
			printExportHelp()
			return 0
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		printExportHelp()
		return 1
	}

	cfg, err := profile.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		return 1
	}

	p, ok := cfg.Profiles[opts.ProfileName]
	if !ok {
		fmt.Fprintf(os.Stderr, "Error: profile %q not found\n", opts.ProfileName)
		return 1
	}

	if p.Environment != profile.EnvironmentContainer {
		fmt.Fprintf(os.Stderr, "Error: profile %q uses environment: %s (export requires environment: container)\n", opts.ProfileName, p.Environment)
		return 1
	}

	if p.Image != "" {
		fmt.Fprintf(os.Stderr, "Error: profile %q uses a pre-built image (%s); there is nothing to build and export\n", opts.ProfileName, p.Image)
		return 1
	}

	ec, err := buildExecutionContext(opts.ProfileName, p)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	dockerStage := stage.NewDockerStage()
	if err := dockerStage.Run(context.Background(), ec); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	snapshot, includes, envVars := mergeExportOptions(opts, p.Export)

	runtime := p.EffectiveContainerRuntime()
	client := docker.NewShellClient(runtime)

	if snapshot {
		if err := runSnapshot(client, ec, p, includes, envVars); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}
	}

	outputPath := opts.OutputPath
	if outputPath == "" {
		safe := strings.NewReplacer(":", "-", "/", "-").Replace(ec.DockerImage)
		outputPath = safe + ".tar"
	}

	fmt.Fprintf(os.Stderr, "Exporting image '%s' to %s...\n", ec.DockerImage, outputPath)
	if err := client.Save(context.Background(), ec.DockerImage, outputPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving image: %v\n", err)
		return 1
	}

	fmt.Fprintf(os.Stderr, "\nDone.\n\n")
	printConfigSnippet(ec.DockerImage, runtime, string(p.Launch), outputPath, snapshot)

	return 0
}

func runSnapshot(client docker.Client, ec *pipeline.ExecutionContext, p profile.Profile, includes []profile.ExportInclude, envVars map[string]string) error {
	fmt.Fprintf(os.Stderr, "Snapshotting image '%s'...\n", ec.DockerImage)

	rc := docker.RunConfig{
		ImageName: ec.DockerImage,
		Command:   []string{"/bin/bash", "-c", snapshotScript},
		EnvVars:   make(map[string]string),
	}

	rc.Mounts = append(rc.Mounts, docker.Mount{
		Source:   ec.OrigWorkDir,
		Target:   "/workspace",
		ReadOnly: true,
	})

	if p.EffectiveSkipDevboxInstall() {
		rc.EnvVars["AW_SKIP_DEVBOX_INSTALL"] = "1"
	}
	if p.EffectiveSkipMiseInstall() {
		rc.EnvVars["AW_SKIP_MISE_INSTALL"] = "1"
	}

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

	var changes []string
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

func mergeExportOptions(opts exportOptions, profileExport *profile.ExportConfig) (snapshot bool, includes []profile.ExportInclude, envVars map[string]string) {
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

	if opts.Snapshot {
		snapshot = true
	}
	includes = append(includes, opts.Include...)
	for k, v := range opts.Env {
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

func parseExportArgs(args []string) (exportOptions, error) {
	opts := exportOptions{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "-o":
			if i+1 >= len(args) {
				return exportOptions{}, fmt.Errorf("%s requires an output path", arg)
			}
			opts.OutputPath = args[i+1]
			i++
		case "--snapshot":
			opts.Snapshot = true
		case "--include":
			if i+1 >= len(args) {
				return exportOptions{}, fmt.Errorf("--include requires src:dst argument")
			}
			parts := strings.SplitN(args[i+1], ":", 2)
			if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
				return exportOptions{}, fmt.Errorf("--include requires format src:dst")
			}
			opts.Include = append(opts.Include, profile.ExportInclude{Src: parts[0], Dst: parts[1]})
			i++
		case "--env":
			if i+1 >= len(args) {
				return exportOptions{}, fmt.Errorf("--env requires KEY=VAL argument")
			}
			kv := strings.SplitN(args[i+1], "=", 2)
			if len(kv) != 2 || kv[0] == "" {
				return exportOptions{}, fmt.Errorf("--env requires format KEY=VAL")
			}
			if opts.Env == nil {
				opts.Env = make(map[string]string)
			}
			opts.Env[kv[0]] = kv[1]
			i++
		case "--help", "-h":
			return exportOptions{}, errExportHelp
		default:
			if strings.HasPrefix(arg, "-") {
				return exportOptions{}, fmt.Errorf("unknown flag %q", arg)
			}
			if opts.ProfileName != "" {
				return exportOptions{}, fmt.Errorf("too many export targets")
			}
			opts.ProfileName = arg
		}
	}

	if opts.ProfileName == "" {
		return exportOptions{}, fmt.Errorf("profile name is required")
	}

	return opts, nil
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

func printExportHelp() {
	fmt.Println("Usage: aw export <profile> [options]")
	fmt.Println()
	fmt.Println("Build a profile's container image and export it as a tar archive.")
	fmt.Println("The exported image can be loaded on an air-gapped machine with 'docker load'.")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  -o <path>          Output file path (default: aw-container-<hash>.tar)")
	fmt.Println("  --snapshot         Run setup in a temporary container and commit the result")
	fmt.Println("  --include src:dst  Copy host path into the image (repeatable; implies --snapshot)")
	fmt.Println("  --env KEY=VAL      Bake environment variable into the image (repeatable; implies --snapshot)")
	fmt.Println("  -h, --help         Show this help")
}

const snapshotScript = `set -e

WORKSPACE="/workspace"
AW_ENV_FILE=/home/agent/.aw_env.sh
BASHRC_FILE=/home/agent/.bashrc
BASH_PROFILE_FILE=/home/agent/.bash_profile

run_as_agent() {
  local script="$1"
  if [ "$(id -un)" = "agent" ]; then
    /bin/bash -lc "$script"
  else
    su -s /bin/bash agent -c "$script"
  fi
}

NIX_ENV="export HOME=/home/agent && . /home/agent/.nix-profile/etc/profile.d/nix.sh 2>/dev/null;"
if [ -f "$WORKSPACE/devbox.json" ]; then
  if [ "${AW_SKIP_DEVBOX_INSTALL:-}" = "1" ]; then
    echo "Skipping devbox install (skip_devbox_install is enabled)"
  else
    echo "Installing packages from devbox.json..."
    run_as_agent "$NIX_ENV cd \"$WORKSPACE\" && devbox install"
  fi
fi
if [ -f "$WORKSPACE/mise.toml" ] || [ -f "$WORKSPACE/.mise.toml" ]; then
  if [ "${AW_SKIP_MISE_INSTALL:-}" = "1" ]; then
    echo "Skipping mise install (skip_mise_install is enabled)"
  else
    if ! run_as_agent 'command -v mise' > /dev/null 2>&1; then
      echo "Installing mise..."
      run_as_agent 'curl https://mise.jdx.dev/install.sh | sh'
    fi
    MISE_CMD="export HOME=/home/agent && export MISE_DATA_DIR=/home/agent/.local/share/mise && export MISE_CONFIG_DIR=/home/agent/.config/mise && export MISE_TRUSTED_CONFIG_PATHS=$WORKSPACE && export MISE_YES=1"
    mkdir -p /home/agent/.config/mise
    echo "Installing tools from mise.toml..."
    run_as_agent "$MISE_CMD && cd \"$WORKSPACE\" && mise install"
    MISE_TOML="$WORKSPACE/mise.toml"
    [ ! -f "$MISE_TOML" ] && MISE_TOML="$WORKSPACE/.mise.toml"
    if [ -f "$MISE_TOML" ]; then
      echo "Registering tools globally for snapshot..."
      cp "$MISE_TOML" /home/agent/.config/mise/config.toml
      chown agent:agent /home/agent/.config/mise/config.toml
    fi
  fi
fi

# Copy included files
i=0
while true; do
  dst_var="AW_INCLUDE_${i}_DST"
  dst="${!dst_var}"
  [ -z "$dst" ] && break
  src="/tmp/aw-include-${i}"
  if [ -d "$src" ]; then
    mkdir -p "$dst"
    cp -a "$src/." "$dst/"
    chown -R agent:agent "$dst"
    echo "Copied include $i -> $dst"
  fi
  i=$((i + 1))
done

# Generate env files
cat > "$AW_ENV_FILE" <<'ENVEOF'
if [ -n "${AW_BASH_ENV_LOADED:-}" ] || [ -n "${AW_BASH_ENV_RECURSION_GUARD:-}" ]; then
  return 0
fi
AW_BASH_ENV_LOADED=1
export HOME=/home/agent

. /home/agent/.nix-profile/etc/profile.d/nix.sh 2>/dev/null
case ":${PATH}:" in
  *:/home/agent/.nix-profile/bin:*) ;;
  *) export PATH="/home/agent/.nix-profile/bin:${PATH}" ;;
esac
case ":${PATH}:" in
  *:/home/agent/.local/bin:*) ;;
  *) export PATH="/home/agent/.local/bin:${PATH}" ;;
esac
export AW_BASH_ENV_RECURSION_GUARD=1
eval "$(devbox global shellenv --install 2>/dev/null | grep '^export ' || true)"
if [ -f "/workspace/devbox.json" ]; then
  eval "$(cd "/workspace" && devbox shellenv 2>/dev/null | grep '^export PATH=' || true)"
fi
unset AW_BASH_ENV_RECURSION_GUARD
case ":${PATH}:" in
  *:/home/agent/.local/share/mise/shims:*) ;;
  *) export PATH="/home/agent/.local/share/mise/shims:${PATH}" ;;
esac
export MISE_TRUSTED_CONFIG_PATHS="/workspace"
export MISE_YES=1
if [ -z "${DOCKER_HOST:-}" ] && [ -S /run/container.sock ]; then
  export DOCKER_HOST="unix:///run/container.sock"
fi
ENVEOF

cat > "$BASHRC_FILE" <<'BASHRC'
if [ -f /home/agent/.aw_env.sh ]; then
  . /home/agent/.aw_env.sh
fi
BASHRC

cat > "$BASH_PROFILE_FILE" <<'BASH_PROFILE'
if [ -f /home/agent/.bashrc ]; then
  . /home/agent/.bashrc
fi
BASH_PROFILE

chown agent:agent "$AW_ENV_FILE" "$BASHRC_FILE" "$BASH_PROFILE_FILE"
echo "Snapshot setup complete."
`
