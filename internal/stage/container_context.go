package stage

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/konono/aw/v4/internal/pipeline"
	"github.com/konono/aw/v4/internal/profile"
)

const containerContextMarker = "\n# aw Container Environment\n"

func appendContainerContext(toolStageDir string, ec *pipeline.ExecutionContext) error {
	if toolStageDir == "" {
		return nil
	}
	claudeMD := filepath.Join(toolStageDir, "CLAUDE.md")

	var sections []string

	if ec.Profile.EffectivePackageManager() == profile.PackageManagerDevbox {
		sections = append(sections, `## Package Managers

- npm: Node.js package manager. Use "npm install -g <pkg>" to install global packages
- mise: polyglot runtime manager. Use "mise install" / "mise use" for language runtimes
- Both are pre-installed and available in PATH

When you install a new tool or package, record it in the workspace so it persists across container rebuilds:
- devbox packages: add via "devbox global add <pkg>" or edit devbox.json in the workspace root
- mise tools: add to mise.toml (or .mise.toml) in the workspace root
- apt/dnf system packages: add to packages.txt in the workspace root (one package per line)`)
	} else {
		sections = append(sections, `## Package Managers

- mise: polyglot runtime manager. Use "mise install" / "mise use" for language runtimes
- Pre-installed and available in PATH

When you install a new tool or package, record it in the workspace so it persists across container rebuilds:
- mise tools: add to mise.toml (or .mise.toml) in the workspace root
- apt/dnf system packages: add to packages.txt in the workspace root (one package per line)`)
	}

	if ec.ContainerSockReady {
		sections = append(sections, `## Docker / Podman (DooD)

Container runtime socket is mounted at /run/container.sock.
DOCKER_HOST and CONTAINER_HOST are pre-configured — docker/docker-compose/podman commands work directly.

- Use docker-compose / docker / podman commands directly — do NOT try to install Docker or start a daemon
- Containers created via docker-compose are sibling containers on the host

### mise-installed podman / docker-compose shim naming issue

mise installs podman and docker-compose with non-standard binary names (e.g. podman-remote-static-linux_<arch>, docker-cli-plugin-docker-compose).
The entrypoint automatically fixes shim names and installs the podman compose plugin after mise install.

If you install podman or docker-compose manually via mise later (not via mise.toml at startup), run these commands to fix the shim names:

    # podman (use subshell cd so the glob expands in the correct directory)
    PODMAN_DIR=$(mise where podman 2>/dev/null) && \
      mkdir -p "$PODMAN_DIR/bin" && \
      (cd "$PODMAN_DIR/bin" && ln -sf ../podman-remote-static-linux_* podman) && \
      mise reshim

    # docker-compose (standalone command)
    DC_DIR=$(mise where docker-compose 2>/dev/null) && \
      mkdir -p "$DC_DIR/bin" && \
      (cd "$DC_DIR/bin" && ln -sf ../docker-cli-plugin-docker-compose docker-compose) && \
      mise reshim

    # docker-compose (podman compose plugin)
    DC_DIR=$(mise where docker-compose 2>/dev/null) && \
      mkdir -p ~/.docker/cli-plugins && \
      ln -sf "$DC_DIR/docker-cli-plugin-docker-compose" ~/.docker/cli-plugins/docker-compose

If the above fails, check actual binary names with: ls $(mise where podman) or ls $(mise where docker-compose)`)
	}

	if ec.Profile.EffectiveGhToken() {
		if ec.Profile.Image != "" {
			sections = append(sections, `## GitHub Token

GITHUB_TOKEN is set. Git HTTPS operations (clone, push, fetch) to github.com work directly via credential helper.
Note: gh CLI may not be available in this pre-built image.`)
		} else {
			sections = append(sections, `## GitHub CLI

GITHUB_TOKEN is set. gh commands (gh pr, gh issue, etc.) work directly.
Git HTTPS operations (clone, push, fetch) to github.com also work directly via credential helper.`)
		}
	} else if ec.Profile.EffectiveMountGH() {
		sections = append(sections, `## GitHub CLI

Host gh configuration is mounted (read-only). gh commands (gh pr, gh issue, etc.) work directly.`)
	}

	if ec.SSHAgentReady {
		sections = append(sections, `## SSH Agent

SSH agent is forwarded. Git SSH operations (push, clone, fetch) work without additional setup.`)
	}

	suffix := "\n# aw Container Environment\n\nThis session runs inside an aw container.\n\n" +
		strings.Join(sections, "\n\n") + "\n"

	existing, _ := os.ReadFile(claudeMD)
	base := string(existing)
	if idx := strings.Index(base, containerContextMarker); idx >= 0 {
		base = base[:idx]
	}

	return os.WriteFile(claudeMD, []byte(base+suffix), 0644)
}
