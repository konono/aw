package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/konono/aw/internal/containerenv"
	"github.com/konono/aw/internal/image"
	"github.com/konono/aw/internal/profile"
	"github.com/konono/aw/internal/toolinfo"
	"github.com/konono/aw/internal/version"
)

func main() {
	osFlag := flag.String("os", "debian12", "OS template (debian12, ubi9, ubi10, ubuntu2604, or 'all')")
	toolFlag := flag.String("tool", "", "Tool name (claude, codex, opencode, cursor)")
	outputFlag := flag.String("output", "", "Output directory for build context")
	allFlag := flag.Bool("all", false, "Render all tools (and all OS if --os all)")
	flag.Parse()

	if *allFlag {
		if *outputFlag == "" {
			*outputFlag = "build"
		}
		osList := profile.OSTemplateNames()
		if *osFlag != "all" {
			osList = []string{*osFlag}
		}
		for _, osName := range osList {
			for _, tool := range toolinfo.Names() {
				dir := filepath.Join(*outputFlag, tool+"-"+osName)
				if err := renderContext(osName, tool, dir); err != nil {
					fmt.Fprintf(os.Stderr, "Error rendering %s-%s: %v\n", tool, osName, err)
					os.Exit(1)
				}
				fmt.Fprintf(os.Stderr, "Rendered: %s\n", dir)
			}
		}
		return
	}

	if *toolFlag == "" {
		fmt.Fprintln(os.Stderr, "Usage: render-image --tool <name> [--os <os>] [--output <dir>]")
		fmt.Fprintln(os.Stderr, "       render-image --all [--os <os|all>] [--output <dir>]")
		fmt.Fprintf(os.Stderr, "Tools: %s\n", strings.Join(toolinfo.Names(), ", "))
		fmt.Fprintf(os.Stderr, "OS:    %s, all\n", strings.Join(profile.OSTemplateNames(), ", "))
		os.Exit(1)
	}

	if *outputFlag == "" {
		*outputFlag = filepath.Join("build", *toolFlag+"-"+*osFlag)
	}

	if err := renderContext(*osFlag, *toolFlag, *outputFlag); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "Rendered: %s\n", *outputFlag)
}

func renderContext(osName, tool, outputDir string) error {
	osTemplate := profile.OSTemplate(osName)
	cenv := containerenv.Default()

	if _, ok := toolinfo.Lookup(tool); !ok {
		return fmt.Errorf("unknown tool: %q (supported: %s)", tool, strings.Join(toolinfo.Names(), ", "))
	}

	dockerfile, err := image.RenderDockerfile(osTemplate, profile.PackageManagerApt, cenv)
	if err != nil {
		return fmt.Errorf("rendering Dockerfile: %w", err)
	}

	entrypoint := image.Entrypoint(profile.PackageManagerApt)
	initScript := image.InitScript()

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	files := map[string][]byte{
		"Dockerfile":    dockerfile,
		"entrypoint.sh": entrypoint,
		"aw-init.sh":    initScript,
	}

	for name, content := range files {
		if err := os.WriteFile(filepath.Join(outputDir, name), content, 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", name, err)
		}
	}

	installScript := toolinfo.InstallScript(tool)
	buildArgsFile := fmt.Sprintf("AW_TOOL_INSTALL_SCRIPT=%s\n", installScript)
	buildArgsFile += fmt.Sprintf("AW_GH_VERSION=%s\n", toolinfo.GhCLIVersion)
	buildArgsFile += fmt.Sprintf("AW_MISE_VERSION=%s\n", toolinfo.MiseVersion)
	buildArgsFile += "AW_OCI_SOURCE=https://github.com/konono/aw\n"
	buildArgsFile += fmt.Sprintf("AW_OCI_VERSION=%s\n", version.Version)
	buildArgsFile += fmt.Sprintf("AW_OCI_OS=%s\n", osName)
	buildArgsFile += fmt.Sprintf("AW_OCI_TOOL=%s\n", tool)

	if err := os.WriteFile(filepath.Join(outputDir, "build-args.env"), []byte(buildArgsFile), 0o644); err != nil {
		return fmt.Errorf("writing build-args.env: %w", err)
	}

	return nil
}
