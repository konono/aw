package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/konono/aw/internal/reaper"
)

func runReaperCommand(args []string) int {
	if len(args) == 0 {
		return reaperShow(nil)
	}

	switch args[0] {
	case "show":
		return reaperShow(args[1:])
	case "list":
		return reaperList()
	case "clear":
		return reaperClear()
	case "dump":
		return reaperDump(args[1:])
	case "recover":
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "Usage: aw reaper recover <container-name>\n")
			return 1
		}
		return reaperRecover(args[1])
	case "discard":
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "Usage: aw reaper discard <container-name>\n")
			return 1
		}
		return reaperDiscard(args[1])
	case "-h", "--help":
		reaperHelp()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "Unknown reaper command: %s\n", args[0])
		reaperHelp()
		return 1
	}
}

func reaperShow(args []string) int {
	dir := reaper.ReaperDir()
	var reportPath string

	if len(args) > 0 {
		arg := args[0]
		if filepath.IsAbs(arg) {
			reportPath = arg
		} else if strings.HasSuffix(arg, ".json") {
			reportPath = filepath.Join(dir, arg)
		} else {
			reports := reaper.ListReports()
			for i := len(reports) - 1; i >= 0; i-- {
				if strings.Contains(filepath.Base(reports[i]), arg) {
					reportPath = reports[i]
					break
				}
			}
			if reportPath == "" {
				fmt.Fprintf(os.Stderr, "No report matching %q\n", arg)
				return 1
			}
		}
	} else {
		reports := reaper.ListReports()
		if len(reports) == 0 {
			fmt.Println("No reaper reports found.")
			return 0
		}
		reportPath = reports[len(reports)-1]
	}

	data, err := os.ReadFile(reportPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	var report reaper.RunReport
	if err := json.Unmarshal(data, &report); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing report: %v\n", err)
		return 1
	}

	ago := time.Since(report.StartedAt).Round(time.Minute)
	summary := fmt.Sprintf("exit %d", report.ExitCode)
	if report.ContainerDiag != nil && report.ContainerDiag.Summary != "" {
		summary = report.ContainerDiag.Summary
	}

	fmt.Printf("  Container:  %s\n", report.ContainerName)
	fmt.Printf("  Exit:       %d — %s\n", report.ExitCode, summary)
	fmt.Printf("  Time:       %s (%s ago)\n", report.StartedAt.Format("2006-01-02 15:04:05"), ago)
	if report.ContainerKept {
		fmt.Printf("  Kept:       yes (debug mode)\n")
	}
	if len(report.Tasks) > 0 {
		fmt.Printf("  Tasks:\n")
		for _, t := range report.Tasks {
			icon := "✓"
			switch t.Status {
			case "ok":
			case "skipped":
				icon = "-"
			default:
				icon = "✗"
			}
			line := fmt.Sprintf("    %s %s (%s)  %s", icon, t.Label, t.Type, t.Duration)
			if t.Error != "" {
				line += fmt.Sprintf(": %s", t.Error)
			}
			fmt.Println(line)
		}
	}
	if report.DumpPath != "" {
		fmt.Printf("  Dump:       %s\n", report.DumpPath)
	}
	fmt.Printf("  Report:     %s\n", reportPath)

	if report.ContainerDiag != nil {
		if report.ContainerDiag.VMOOMHint {
			fmt.Printf("  Hint:       consider increasing VM memory:\n")
			fmt.Printf("              podman machine set --memory 16384\n")
		}
		if report.ExitCode == 143 {
			fmt.Printf("  Hint:       possibly caused by terminal close or external process\n")
		}
	}

	return 0
}

func reaperList() int {
	reports := reaper.ListReports()
	if len(reports) == 0 {
		fmt.Println("No reaper reports found.")
		return 0
	}

	for _, r := range reports {
		data, err := os.ReadFile(r)
		if err != nil {
			continue
		}
		var report reaper.RunReport
		if json.Unmarshal(data, &report) != nil {
			continue
		}
		summary := fmt.Sprintf("exit %d", report.ExitCode)
		if report.ContainerDiag != nil && report.ContainerDiag.Summary != "" {
			summary = report.ContainerDiag.Summary
		}
		fmt.Printf("  %s  %-30s  %s\n", report.StartedAt.Format("2006-01-02 15:04"), report.ContainerName, summary)
	}
	return 0
}

func reaperClear() int {
	reports := reaper.ListReports()
	for _, r := range reports {
		_ = os.Remove(r)
	}
	reaper.ClearDumps()
	fmt.Printf("Cleared %d report(s).\n", len(reports))
	return 0
}

func reaperRecover(containerName string) int {
	specPath := filepath.Join(reaper.ReaperDir(), containerName+".spec.json")
	if _, err := os.Stat(specPath); err != nil {
		fmt.Fprintf(os.Stderr, "No spec found for %s\n", containerName)
		return 1
	}
	// Full recovery (including shell tasks)
	if err := reaper.RecoverFromSpec(specPath, containerName); err != nil {
		fmt.Fprintf(os.Stderr, "Recovery failed: %v\n", err)
		return 1
	}
	fmt.Printf("Recovery complete for %s\n", containerName)
	return 0
}

func reaperDiscard(containerName string) int {
	specPath := filepath.Join(reaper.ReaperDir(), containerName+".spec.json")
	if err := os.Remove(specPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	fmt.Printf("Discarded spec for %s\n", containerName)
	return 0
}

func reaperDump(args []string) int {
	dir := reaper.ReaperDir()
	var reportPath string

	if len(args) > 0 {
		arg := args[0]
		if filepath.IsAbs(arg) {
			return reaper.ShowDump(arg)
		} else if strings.HasSuffix(arg, ".json") {
			reportPath = filepath.Join(dir, arg)
		} else {
			reports := reaper.ListReports()
			for i := len(reports) - 1; i >= 0; i-- {
				if strings.Contains(filepath.Base(reports[i]), arg) {
					reportPath = reports[i]
					break
				}
			}
			if reportPath == "" {
				fmt.Fprintf(os.Stderr, "No report matching %q\n", arg)
				return 1
			}
		}
	} else {
		reports := reaper.ListReports()
		if len(reports) == 0 {
			fmt.Println("No reaper reports found.")
			return 0
		}
		reportPath = reports[len(reports)-1]
	}

	data, err := os.ReadFile(reportPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	var report reaper.RunReport
	if err := json.Unmarshal(data, &report); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing report: %v\n", err)
		return 1
	}

	if report.DumpPath == "" {
		fmt.Println("No dump collected for this session.")
		return 0
	}

	return reaper.ShowDump(report.DumpPath)
}

func reaperHelp() {
	fmt.Println("Usage: aw reaper <command>")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  show [file]      Show report details (default: latest)")
	fmt.Println("  list             List recent reports")
	fmt.Println("  dump [file]      Show diagnostic dump (logs, inspect, dmesg)")
	fmt.Println("  clear            Delete all reports and dumps")
	fmt.Println("  recover <name>   Re-run pending tasks (skips already-succeeded)")
	fmt.Println("  discard <name>   Discard spec and abandon recovery")
}
