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
	toolFlag := flag.String("tool", "", "Tool name (base, claude, codex, opencode, cursor)")
	outputFlag := flag.String("output", "", "Output directory for build context")
	allFlag := flag.Bool("all", false, "Render all tools (and all OS if --os all)")
	flag.Parse()

	tools := toolinfo.Names()
	if !*allFlag && *toolFlag != "" {
		tools = []string{*toolFlag}
	} else if !*allFlag {
		fmt.Fprintln(os.Stderr, "Usage: render-image --tool <name> [--os <os|all>] [--output <dir>]")
		fmt.Fprintln(os.Stderr, "       render-image --all [--os <os|all>] [--output <dir>]")
		fmt.Fprintf(os.Stderr, "Tools: %s\n", strings.Join(toolinfo.Names(), ", "))
		fmt.Fprintf(os.Stderr, "OS:    %s, all\n", strings.Join(profile.OSTemplateNames(), ", "))
		os.Exit(1)
	}

	osList := profile.OSTemplateNames()
	if *osFlag != "all" {
		osList = []string{*osFlag}
	}

	explicitOutput := *outputFlag != ""
	if !explicitOutput {
		*outputFlag = "build"
	}

	// When --output is explicitly set for a single tool+OS, write directly
	// to that path (backwards-compatible with the old --tool behavior).
	if explicitOutput && len(tools) == 1 && len(osList) == 1 {
		if err := renderContext(osList[0], tools[0], *outputFlag); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Rendered: %s\n", *outputFlag)
		return
	}

	for _, osName := range osList {
		for _, tool := range tools {
			dir := filepath.Join(*outputFlag, tool+"-"+osName)
			if err := renderContext(osName, tool, dir); err != nil {
				fmt.Fprintf(os.Stderr, "Error rendering %s-%s: %v\n", tool, osName, err)
				os.Exit(1)
			}
			fmt.Fprintf(os.Stderr, "Rendered: %s\n", dir)
		}
	}
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
