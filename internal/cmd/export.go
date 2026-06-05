package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/konono/aw/internal/docker"
	"github.com/konono/aw/internal/profile"
	"github.com/konono/aw/internal/stage"
)

type exportOptions struct {
	ProfileName string
	OutputPath  string
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

	outputPath := opts.OutputPath
	if outputPath == "" {
		safe := strings.NewReplacer(":", "-", "/", "-").Replace(ec.DockerImage)
		outputPath = safe + ".tar"
	}

	runtime := p.EffectiveContainerRuntime()
	client := docker.NewShellClient(runtime)
	fmt.Fprintf(os.Stderr, "Exporting image '%s' to %s...\n", ec.DockerImage, outputPath)
	if err := client.Save(context.Background(), ec.DockerImage, outputPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving image: %v\n", err)
		return 1
	}

	fmt.Fprintf(os.Stderr, "\nDone.\n\n")
	printConfigSnippet(ec.DockerImage, runtime, string(p.Launch), outputPath)

	return 0
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

func printConfigSnippet(imageName, runtime, launch, tarPath string) {
	fmt.Fprintf(os.Stderr, "# Load on target machine:\n")
	fmt.Fprintf(os.Stderr, "#   %s load -i %s\n", runtime, tarPath)
	fmt.Fprintf(os.Stderr, "#\n")
	fmt.Fprintf(os.Stderr, "# Add to ~/.config/aw/config.yml:\n")
	fmt.Fprintf(os.Stderr, "#   profiles:\n")
	fmt.Fprintf(os.Stderr, "#     airgap:\n")
	fmt.Fprintf(os.Stderr, "#       environment: container\n")
	fmt.Fprintf(os.Stderr, "#       launch: %s\n", launch)
	fmt.Fprintf(os.Stderr, "#       image: %s\n", imageName)
	fmt.Fprintf(os.Stderr, "#       # For offline/air-gapped environments, add:\n")
	fmt.Fprintf(os.Stderr, "#       # skip_devbox_install: true\n")
	fmt.Fprintf(os.Stderr, "#       # skip_mise_install: true\n")
}

func printExportHelp() {
	fmt.Println("Usage: aw export <profile> [-o output.tar]")
	fmt.Println()
	fmt.Println("Build a profile's container image and export it as a tar archive.")
	fmt.Println("The exported image can be loaded on an air-gapped machine with 'docker load'.")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  -o <path>    Output file path (default: aw-container-<hash>.tar)")
	fmt.Println("  -h, --help   Show this help")
}
