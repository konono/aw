package cmd

import (
	"fmt"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/konono/aw/internal/profile"
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

// PassthroughCmd holds the -c passthrough args split before kong sees them.
type PassthroughCmd struct{ Args []string }

// SplitAtDashC splits args at the first -c. Everything before -c goes to kong,
// everything after becomes the passthrough command for container launch.
// Returns an error if -c has no following arguments.
func SplitAtDashC(args []string) (kongArgs []string, cmdArgs []string, err error) {
	for i, a := range args {
		if a == "-c" {
			if i+1 < len(args) {
				return args[:i], args[i+1:], nil
			}
			return nil, nil, fmt.Errorf("-c requires a command")
		}
	}
	return args, nil, nil
}

// CLI is the top-level kong grammar.
type CLI struct {
	// Global flags
	Version kong.VersionFlag `name:"version" help:"Show version."`

	// Subcommands
	Run              RunCmd              `cmd:"" default:"withargs" help:"Run a profile (default command)."`
	Profiles         ProfilesCmd         `cmd:"" help:"List available profiles."`
	Init             InitCmd             `cmd:"" help:"Write the built-in config to ~/.config/aw/config.yml."`
	Auth             AuthCmd             `cmd:"" help:"Run auth login/logout/status for a tool."`
	Login            LoginCmd            `cmd:"" help:"Alias for 'auth login'."`
	Build            BuildCmd            `cmd:"" help:"Build a profile's container image."`
	Export           ExportCmd           `cmd:"" hidden:"" help:"Deprecated: use 'build' instead."`
	Doctor           DoctorCmd           `cmd:"" help:"Check system environment and configuration."`
	Reaper           ReaperCmd           `cmd:"" help:"View/recover post-container cleanup reports."`
	Team             TeamCmd             `cmd:"" help:"Manage agent teams."`
	Msg              MsgCmd              `cmd:"" help:"Inter-agent messaging."`
	Update           UpdateCmd           `cmd:"" help:"Update aw to the latest version."`
	DefaultDockerfile DefaultDockerfileCmd `cmd:"" name:"default-dockerfile" help:"Print the default Dockerfile."`
	DefaultInitScript DefaultInitScriptCmd `cmd:"" name:"default-init-script" help:"Print aw-init.sh."`

	// Internal subcommands (hidden, used by containers)
	InternalMCPMsg    InternalMCPMsgCmd    `cmd:"" name:"internal-mcp-msg" hidden:"" help:"Run MCP message server."`
	InternalCheckInbox InternalCheckInboxCmd `cmd:"" name:"internal-check-inbox" hidden:"" help:"Check inbox for unread messages."`
	InternalWatch     InternalWatchCmd     `cmd:"" name:"internal-watch" hidden:"" help:"Watch for messages."`
	InternalAgentLoop InternalAgentLoopCmd `cmd:"" name:"internal-agent-loop" hidden:"" help:"Run agent loop."`
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
	Profile  string `arg:"" optional:"" help:"Profile name to run."`
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
	Tool    string `arg:"" help:"Tool name (claude, codex, opencode, cursor) or profile name."`
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
	ProfileName  string            `arg:"" help:"Profile name to build."`
	Save         *string           `name:"save" help:"Save image as tar archive (auto-names if path omitted)."`
	FromTemplate bool              `name:"from-template" help:"Build from Dockerfile template instead of official image."`
	Apply        bool              `name:"apply" help:"Write image name back to config file."`
	NoCache      bool              `name:"no-cache" help:"Rebuild without cache (requires --from-template)."`
	Include      []string          `name:"include" help:"Copy host path into image (src:dst format, repeatable)." placeholder:"src:dst"`
	Env          map[string]string `name:"env" help:"Bake env var into image (KEY=VAL, repeatable)."`

	skipSnapshot    bool            // internal: used by deprecated export compat shim
	preloadedConfig *profile.Config // internal: avoids double profile.Load() in export compat
}

func (b *BuildCmd) Validate() error {
	if b.NoCache && !b.FromTemplate {
		return fmt.Errorf("--no-cache requires --from-template")
	}
	return nil
}

// ExportCmd is a deprecated alias for BuildCmd.
type ExportCmd struct {
	ProfileName string            `arg:"" help:"Profile name to export."`
	Output      string            `short:"o" name:"output" help:"Output file path." type:"path"`
	Snapshot    bool              `name:"snapshot" help:"(Deprecated, now always on) Run setup and commit the result."`
	Apply       bool              `name:"apply" help:"Write image name back to config file."`
	NoCache     bool              `name:"no-cache" help:"Rebuild without cache."`
	Include     []string          `name:"include" help:"Copy host path into image (src:dst format, repeatable)." placeholder:"src:dst"`
	Env         map[string]string `name:"env" help:"Bake env var into image (KEY=VAL, repeatable)."`
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

// TeamCmd handles team subcommands.
type TeamCmd struct {
	Start  TeamStartCmd  `cmd:"" help:"Start all team members."`
	Stop   TeamStopCmd   `cmd:"" help:"Stop all team members."`
	Status TeamStatusCmd `cmd:"" help:"Show team status."`
	Scope  TeamScopeCmd  `cmd:"" help:"Print team scope."`
}

// TeamStartCmd starts a team.
type TeamStartCmd struct {
	TeamName string `arg:"" help:"Team name to start."`
	Resume   bool   `name:"resume" help:"Resume a previous session."`
	Task     string `name:"task" help:"Task description for the team."`
}

// TeamStopCmd stops a team.
type TeamStopCmd struct {
	TeamName string `arg:"" help:"Team name to stop."`
}

// TeamStatusCmd shows team status.
type TeamStatusCmd struct {
	TeamName string `arg:"" optional:"" help:"Team name (shows all if omitted)."`
}

// TeamScopeCmd prints team scope.
type TeamScopeCmd struct {
	TeamName string `arg:"" help:"Team name."`
}

// MsgCmd handles messaging subcommands.
type MsgCmd struct {
	Send    MsgSendCmd    `cmd:"" help:"Send a message."`
	Inbox   MsgInboxCmd   `cmd:"" help:"Show unread messages."`
	History MsgHistoryCmd `cmd:"" help:"Show message history."`
	Watch   MsgWatchCmd   `cmd:"" help:"Watch all messages in real-time."`
	Clear   MsgClearCmd   `cmd:"" help:"Delete messages."`
}

// MsgSendCmd sends a message.
type MsgSendCmd struct {
	Team string `name:"team" required:"" help:"Team scope."`
	From string `arg:"" help:"Sender agent name."`
	To   string `arg:"" help:"Recipient agent name."`
	Body string `arg:"" help:"Message body."`
}

// MsgInboxCmd shows inbox.
type MsgInboxCmd struct {
	Team  string `name:"team" required:"" help:"Team scope."`
	Agent string `arg:"" help:"Agent name."`
}

// MsgHistoryCmd shows history.
type MsgHistoryCmd struct {
	Team  string `name:"team" required:"" help:"Team scope."`
	Agent string `name:"agent" help:"Filter by agent name."`
	Limit int    `name:"limit" default:"50" help:"Maximum messages to show."`
}

// MsgWatchCmd watches messages.
type MsgWatchCmd struct {
	Team string `name:"team" required:"" help:"Team scope."`
}

// MsgClearCmd deletes messages.
type MsgClearCmd struct {
	Team   string `name:"team" help:"Delete messages for a specific team scope."`
	All    bool   `name:"all" help:"Delete all messages."`
	Before string `name:"before" help:"Delete messages older than duration (e.g. 7d, 24h, 30m)."`
	List   bool   `name:"list" help:"Show available team scopes."`
}

func (c *MsgClearCmd) Validate() error {
	if c.All && (c.Team != "" || c.Before != "") {
		return fmt.Errorf("--all cannot be combined with --team or --before")
	}
	return nil
}

// InternalMCPMsgCmd runs the MCP message server.
type InternalMCPMsgCmd struct {
	DB    string `name:"db" env:"AW_MSG_DB" help:"Path to messages database."`
	Agent string `name:"agent" env:"AW_AGENT_NAME" help:"Agent name."`
	Team  string `name:"team" env:"AW_TEAM_NAME" help:"Team scope."`
}

// InternalCheckInboxCmd checks inbox for unread messages.
type InternalCheckInboxCmd struct {
	DB    string `name:"db" env:"AW_MSG_DB" help:"Path to messages database."`
	Agent string `name:"agent" env:"AW_AGENT_NAME" help:"Agent name."`
	Team  string `name:"team" env:"AW_TEAM_NAME" help:"Team scope."`
}

// InternalWatchCmd watches for messages.
type InternalWatchCmd struct {
	DB    string `name:"db" env:"AW_MSG_DB" help:"Path to messages database."`
	Agent string `name:"agent" env:"AW_AGENT_NAME" help:"Agent name."`
	Team  string `name:"team" env:"AW_TEAM_NAME" help:"Team scope."`
}

// InternalAgentLoopCmd runs agent loop.
type InternalAgentLoopCmd struct {
	DB    string `name:"db" env:"AW_MSG_DB" help:"Path to messages database."`
	Agent string `name:"agent" env:"AW_AGENT_NAME" help:"Agent name."`
	Team  string `name:"team" env:"AW_TEAM_NAME" help:"Team scope."`
	Tool  string `name:"tool" env:"AW_TOOL" help:"Tool name."`
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
