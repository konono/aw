package cmd

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/alecthomas/kong"
	kongcompletion "github.com/jotaen/kong-completion"
	"github.com/konono/aw/v4/internal/profile"
)

// ExitError signals a non-zero exit code without printing an extra message.
type ExitError struct{ Code int }

func (e ExitError) Error() string { return fmt.Sprintf("exit code %d", e.Code) }

func exitCode(code int) error {
	if code == 0 {
		return nil
	}
	return ExitError{Code: code}
}

// PassthroughCmd holds the passthrough args split before kong sees them.
type PassthroughCmd struct {
	Args       []string
	UsedLegacy bool
}

// ExtractRunPassthrough splits args at the first "--" or legacy "-c",
// whichever appears first. This preserves backward compatibility: when a
// user writes "aw profile -c npm run -- build", the -c at position 1
// wins and "npm run -- build" becomes the passthrough command intact.
// Returns usedLegacy=true when "-c" was the separator.
func ExtractRunPassthrough(args []string) (kongArgs, cmdArgs []string, usedLegacy bool, err error) {
	for i, a := range args {
		if a == "--" || a == "-c" {
			if i+1 >= len(args) {
				return nil, nil, a == "-c", fmt.Errorf("%s requires a command", a)
			}
			return args[:i], args[i+1:], a == "-c", nil
		}
	}
	return args, nil, false, nil
}

// CLI is the top-level kong grammar.
type CLI struct {
	// Global flags
	Version kong.VersionFlag `name:"version" help:"Show version."`

	// Subcommands
	Run               RunCmd               `cmd:"" default:"withargs" help:"Run a profile (default command)."`
	Profiles          ProfilesCmd          `cmd:"" help:"List available profiles."`
	Init              InitCmd              `cmd:"" help:"Write the built-in config to ~/.config/aw/config.yml."`
	Auth              AuthCmd              `cmd:"" help:"Run auth login/logout/status for a tool."`
	Login             LoginCmd             `cmd:"" help:"Alias for 'auth login'."`
	Build             BuildCmd             `cmd:"" help:"Build a profile's container image."`
	Save              SaveCmd              `cmd:"" help:"Save a container's state as a reusable image and update .aw.yml."`
	Manifest          ManifestCmd          `cmd:"" help:"Generate Kubernetes manifests for a profile."`
	Doctor            DoctorCmd            `cmd:"" help:"Check system environment and configuration."`
	Reaper            ReaperCmd            `cmd:"" help:"View/recover post-container cleanup reports."`
	Update            UpdateCmd            `cmd:"" help:"Update aw to the latest version."`
	DefaultDockerfile DefaultDockerfileCmd `cmd:"" name:"default-dockerfile" help:"Print the default Dockerfile."`
	DefaultInitScript DefaultInitScriptCmd `cmd:"" name:"default-init-script" help:"Print aw-init.sh."`

	// Completion support
	Completion kongcompletion.Completion `cmd:"" help:"Outputs shell code for initialising tab completions."`
}

// ProfilesCmd lists available profiles.
type ProfilesCmd struct{}

// UpdateCmd updates aw.
type UpdateCmd struct{}

// DefaultDockerfileCmd prints default Dockerfile.
type DefaultDockerfileCmd struct{}

// DefaultInitScriptCmd prints default init script.
type DefaultInitScriptCmd struct{}

// RunCmd launches a profile.
type RunCmd struct {
	Profile  string `arg:"" optional:"" help:"Profile name to run." completion-predictor:"profile"`
	Recent   bool   `short:"r" name:"recent" help:"Pick a directory from launch history." aliases:"recent-dir"`
	Query    string `name:"query" help:"Initial query for --recent picker."`
	Cwd      string `short:"C" name:"cwd" help:"Change to path before loading config."`
	NoRecord bool   `name:"no-record" help:"Don't record this launch in directory history."`
	NoCache  bool   `name:"no-cache" help:"Rebuild the container image without using cache."`
}

func (r *RunCmd) Validate() error {
	if r.Recent && r.Cwd != "" {
		return fmt.Errorf("--recent and -C/--cwd cannot be used together")
	}
	if r.Query != "" && !r.Recent {
		return fmt.Errorf("--query requires --recent")
	}
	return nil
}

// InitCmd writes default config.
type InitCmd struct {
	Force bool `name:"force" help:"Overwrite existing config file."`
}

// AuthCmd handles auth subcommands.
type AuthCmd struct {
	Login  AuthLoginCmd  `cmd:"" help:"Log in to a tool."`
	Logout AuthLogoutCmd `cmd:"" help:"Log out from a tool."`
	Status AuthStatusCmd `cmd:"" help:"Show auth status for a tool."`
}

type authFlags struct {
	Tool    string `arg:"" help:"Tool name (claude, codex, opencode, cursor) or profile name." completion-predictor:"tool"`
	Profile string `short:"p" name:"profile" help:"Explicit profile name."`
}

// AuthLoginCmd logs in.
type AuthLoginCmd struct{ authFlags }

// AuthLogoutCmd logs out.
type AuthLogoutCmd struct{ authFlags }

// AuthStatusCmd shows status.
type AuthStatusCmd struct{ authFlags }

// LoginCmd is an alias for auth login.
type LoginCmd struct{ authFlags }

// BuildCmd builds a profile's container image with snapshot.
type BuildCmd struct {
	ProfileName  string            `arg:"" help:"Profile name to build." completion-predictor:"profile"`
	Save         *string           `name:"save" help:"Save image as tar archive." placeholder:"PATH"`
	Apply        bool              `name:"apply" help:"Write image name back to config file."`
	NoCache      bool              `name:"no-cache" help:"Rebuild from template without cache."`
	Push         bool              `name:"push" help:"Push the image to a container registry."`
	Registry     string            `name:"registry" help:"Registry to push to (e.g. ghcr.io/myorg)." placeholder:"REGISTRY"`
	Include      []string          `name:"include" help:"Copy host path into image (src:dst format, repeatable)." placeholder:"src:dst"`
	Env          map[string]string `name:"env" help:"Bake env var into image (KEY=VAL, repeatable)."`
	BuildArg     map[string]string `name:"build-arg" help:"Pass a build arg to docker build (KEY=VAL, repeatable)." placeholder:"KEY=VAL"`

	preloadedConfig *profile.Config // internal: avoids double profile.Load() in export compat
}

func (b *BuildCmd) Validate() error {
	if b.Push && b.Registry == "" {
		return fmt.Errorf("--push requires --registry")
	}
	if b.Registry != "" && !b.Push {
		return fmt.Errorf("--registry requires --push")
	}
	for k := range b.BuildArg {
		if strings.HasPrefix(k, "AW_") {
			return fmt.Errorf("--build-arg key %q conflicts with reserved AW_* prefix", k)
		}
	}
	return nil
}

// SaveCmd commits a container's filesystem state to an image and updates .aw.yml.
type SaveCmd struct {
	Runtime   string `name:"runtime" help:"Container runtime (docker or podman). When omitted, queries all installed runtimes."`
	ImageName string `name:"image" help:"Override the saved image name (default: aw-save:<profile>-<timestamp>)."`
}

// ManifestCmd generates Kubernetes manifests.
type ManifestCmd struct {
	ProfileName string `arg:"" help:"Profile name." completion-predictor:"profile"`
	Output      string `short:"o" name:"output" help:"Output directory (default: stdout)."`
	Name        string `name:"name" help:"Instance name suffix for multi-instance deployments."`
	Image       string `name:"image" help:"Override image name."`
}

var dns1123Re = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)

func (m *ManifestCmd) Validate() error {
	if m.Name != "" {
		if len(m.Name) > 63 {
			return fmt.Errorf("--name must be 63 characters or less")
		}
		if !dns1123Re.MatchString(m.Name) {
			return fmt.Errorf("--name must be a valid DNS-1123 label (lowercase alphanumeric and hyphens, must start/end with alphanumeric)")
		}
	}
	return nil
}

// DoctorCmd checks system environment.
type DoctorCmd struct {
	Verbose bool `short:"v" name:"verbose" help:"Show detailed diagnostic information."`
}

// ReaperCmd handles reaper subcommands.
type ReaperCmd struct {
	Show    ReaperShowCmd    `cmd:"" default:"withargs" help:"Show report details (default: latest)."`
	List    ReaperListCmd    `cmd:"" help:"List recent reports."`
	Clear   ReaperClearCmd   `cmd:"" help:"Delete all reports and dumps."`
	Dump    ReaperDumpCmd    `cmd:"" help:"Show diagnostic dump."`
	Recover ReaperRecoverCmd `cmd:"" help:"Re-run pending tasks."`
	Discard ReaperDiscardCmd `cmd:"" help:"Discard spec and abandon recovery."`
}

// ReaperShowCmd shows a report.
type ReaperShowCmd struct {
	File string `arg:"" optional:"" help:"Report file or container name."`
}

// ReaperListCmd lists reports.
type ReaperListCmd struct{}

// ReaperClearCmd deletes reports.
type ReaperClearCmd struct{}

// ReaperDumpCmd shows diagnostic dump.
type ReaperDumpCmd struct {
	File string `arg:"" optional:"" help:"Report file or container name."`
}

// ReaperRecoverCmd recovers from a spec.
type ReaperRecoverCmd struct {
	ContainerName string `arg:"" help:"Container name to recover."`
}

// ReaperDiscardCmd discards a spec.
type ReaperDiscardCmd struct {
	ContainerName string `arg:"" help:"Container name to discard."`
}

// IsSubcommand returns true if the first non-flag argument matches a known
// subcommand in the kong parser model, excluding the default command (run).
// The default command uses -- (or legacy -c) as a passthrough separator, so
// ExtractRunPassthrough must still run for it.
func IsSubcommand(parser *kong.Kong, args []string) bool {
	if len(args) == 0 {
		return false
	}
	first := args[0]
	if strings.HasPrefix(first, "-") {
		return false
	}
	for _, child := range parser.Model.Children {
		if child == nil {
			continue
		}
		if child == parser.Model.DefaultCmd {
			continue
		}
		if child.Name == first {
			return true
		}
		for _, alias := range child.Aliases {
			if alias == first {
				return true
			}
		}
	}
	return false
}

// parseBuildIncludes parses --include src:dst strings into profile.BuildInclude.
func parseBuildIncludes(includes []string) ([]profile.BuildInclude, error) {
	var result []profile.BuildInclude
	for _, s := range includes {
		parts := strings.SplitN(s, ":", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return nil, fmt.Errorf("--include requires format src:dst, got %q", s)
		}
		result = append(result, profile.BuildInclude{Src: parts[0], Dst: parts[1]})
	}
	return result, nil
}
