package profile

import (
	"fmt"
	"strings"
	"testing"
)

func TestMigrate_ShowBeforeAfter(t *testing.T) {
	cases := []struct {
		name   string
		before string
	}{
		{
			name: "top-level scalar changes (runtime, mount_ssh, default)",
			before: `default: shell
environment: container
os: debian12
container_runtime: docker
mount_ssh: true
ssh_agent_forwarding: true
profiles:
  shell:
    launch: shell
  claude:
    launch: claude
`,
		},
		{
			name: "builtin profile customized with auth",
			before: `default: claude
environment: container
os: debian12
container_runtime: podman
profiles:
  claude:
    launch: claude
    auth:
      on_launch:
        check: warn
      claude:
        login_mode: sso
  shell:
    launch: shell
  codex:
    launch: codex
    auth:
      on_launch:
        check: require
      codex:
        login_mode: device
        credentials_store: file
        seed_from_host: if_missing
`,
		},
		{
			name: "user-defined custom profiles preserved",
			before: `default: claude
environment: container
os: debian12
container_runtime: podman
profiles:
  claude:
    launch: claude
  shell:
    launch: shell
  my-vertex:
    launch: claude
    env:
      CLAUDE_CODE_USE_VERTEX: "1"
      CLOUD_ML_REGION: us-east5
    mounts:
      - source: "~/.config/gcloud"
        target: "/home/agent/.config/gcloud"
        mode: ro
  my-host-shell:
    environment: host
    launch: shell
    env:
      EDITOR: vim
`,
		},
		{
			name: "top-level env and mount_gh",
			before: `default: claude
environment: container
os: debian12
container_runtime: podman
mount_gh: true
env:
  CLAUDE_CODE_USE_VERTEX: "1"
  ANTHROPIC_VERTEX_PROJECT_ID: my-project
profiles:
  claude:
    launch: claude
  shell:
    launch: shell
`,
		},
		{
			name: "minimal config (only changed default)",
			before: `default: codex
profiles:
  codex:
    launch: codex
`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			userCfg, err := Parse([]byte(tc.before))
			if err != nil {
				t.Fatalf("parse before: %v", err)
			}

			after, err := Migrate(userCfg)
			if err != nil {
				t.Fatalf("Migrate: %v", err)
			}

			fmt.Printf("\n%s\n", strings.Repeat("═", 120))
			fmt.Printf("  CASE: %s\n", tc.name)
			fmt.Printf("%s\n", strings.Repeat("═", 120))
			printSideBySide(tc.before, string(after), 58)
			fmt.Println()
		})
	}
}

func printSideBySide(left, right string, colWidth int) {
	leftLines := strings.Split(strings.TrimRight(left, "\n"), "\n")
	rightLines := strings.Split(strings.TrimRight(right, "\n"), "\n")

	maxLines := len(leftLines)
	if len(rightLines) > maxLines {
		maxLines = len(rightLines)
	}

	header := fmt.Sprintf("  %-*s │ %s", colWidth, "BEFORE", "AFTER")
	fmt.Println(header)
	fmt.Printf("  %s─┼─%s\n", strings.Repeat("─", colWidth), strings.Repeat("─", colWidth))

	for i := 0; i < maxLines; i++ {
		l := ""
		if i < len(leftLines) {
			l = leftLines[i]
		}
		r := ""
		if i < len(rightLines) {
			r = rightLines[i]
		}

		marker := " "
		if l != r {
			marker = "!"
		}
		if i >= len(leftLines) {
			marker = "+"
		}
		if i >= len(rightLines) {
			marker = "-"
		}

		fmt.Printf("%s %-*s │ %s\n", marker, colWidth, truncate(l, colWidth), truncate(r, colWidth))
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}
