package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/konono/aw/internal/docker"
	"github.com/konono/aw/internal/profile"
	"github.com/konono/aw/internal/stage"
)

func runExport(args []string) int {
	outputPath := ""
	profileName := ""

	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "-o" && i+1 < len(args):
			outputPath = args[i+1]
			i++
		case args[i] == "--help" || args[i] == "-h":
			printExportHelp()
			return 0
		case !strings.HasPrefix(args[i], "-"):
			profileName = args[i]
		default:
			fmt.Fprintf(os.Stderr, "Error: unknown flag %q\n", args[i])
			printExportHelp()
			return 1
		}
	}

	if profileName == "" {
		fmt.Fprintf(os.Stderr, "Error: profile name is required\n")
		printExportHelp()
		return 1
	}

	cfg, err := profile.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		return 1
	}

	p, ok := cfg.Profiles[profileName]
	if !ok {
		fmt.Fprintf(os.Stderr, "Error: profile %q not found\n", profileName)
		return 1
	}

	if p.Environment != profile.EnvironmentContainer {
		fmt.Fprintf(os.Stderr, "Error: profile %q uses environment: %s (export requires environment: container)\n", profileName, p.Environment)
		return 1
	}

	if p.Image != "" {
		fmt.Fprintf(os.Stderr, "Error: profile %q uses a pre-built image (%s); there is nothing to build and export\n", profileName, p.Image)
		return 1
	}

	ec, err := buildExecutionContext(profileName, p)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	dockerStage := stage.NewDockerStage()
	if err := dockerStage.Run(context.Background(), ec); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

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
