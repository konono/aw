# agent-workspace (`aw`) — fork

> **This is a fork of [hiragram/agent-workspace](https://github.com/hiragram/agent-workspace)** with additional features for Podman support, custom mounts, mise integration, and quality-of-life improvements.

A CLI tool for launching agent workspaces with configurable profiles. Supports Docker/Podman containers, git worktrees, zellij sessions, and combinations thereof.

## Fork additions

| Feature | Description |
|---------|-------------|
| **Podman native support** | `container_runtime: podman` — no wrapper script needed |
| **Custom mounts** | `mounts:` field to bind-mount arbitrary host directories into the container |
| **mise integration** | Project's `mise.toml` controls tool installation inside the container |
| **Minimal base image** | Stripped Dockerfile — only OS essentials + mise; no bundled languages |
| **`volume create` Podman fix** | Handles Podman's non-idempotent `volume create` (exit 125) with `--ignore` flag |
| **Zellij tab reuse** | When already inside zellij, opens a new tab instead of nesting sessions |
| **`--help` flag** | `aw --help` / `aw -h` for usage information |
| **macOS test fix** | Fixed `/var` → `/private/var` symlink issue in worktree hook tests |

## Install

### From source

```bash
go install github.com/konono/agent-workspace@latest
```

The binary is installed as `agent-workspace`. Create a symlink for the `aw` command:

```bash
ln -sf ~/go/bin/agent-workspace ~/go/bin/aw
```

## Usage

```bash
aw                      # Run the default profile
aw <profile-name>       # Run a specific profile
aw profiles             # List available profiles
aw default-dockerfile   # Print the default Dockerfile
aw update               # Self-update
aw --version            # Show version
aw --help               # Show help
```

## Configuration

> **[Detailed Configuration Guide](docs/configuration.md)** — Full reference for all options, validation rules, and examples.

Create `.agent-workspace.yml` in your git repository root:

```yaml
default: worktree-zellij

# Top-level defaults (shared by all profiles)
container_runtime: podman        # "docker" (default) or "podman"

env:
  CLAUDE_CODE_USE_VERTEX: "1"
  CLOUD_ML_REGION: "us-east5"

mounts:
  - source: "~/.config/gcloud"
    target: "/home/claude/.config/gcloud"

profiles:
  claude:
    environment: container
    launch: claude

  worktree-shell:
    worktree:
      base: origin/main
    environment: host
    launch: shell

  worktree-zellij:
    worktree: {}
    environment: container
    launch: zellij
    zellij:
      layout: default
```

If no `.agent-workspace.yml` is found, `aw` uses a built-in default that creates a worktree and starts a zellij dev environment with Docker-based Claude.

### Profile options

- **`worktree`** (optional): Creates a git worktree.
  - `base` — base ref for the new worktree. Defaults to `origin/main`.
  - `dir` — directory under which worktrees are created. Defaults to `<repoRoot>/worktrees`. Supports `~` expansion.
  - `on-create` / `on-end` — shell hooks run after the worktree is created / after the launched process exits.
- **`environment`** (required): `"host"` or `"container"` — where the main process runs.
- **`launch`** (required): `"shell"`, `"claude"`, or `"zellij"` — what to launch.
- **`zellij`** (optional): Zellij session config. Only valid with `launch: zellij`.
- **`container_runtime`** (optional): `"docker"` or `"podman"`. Defaults to `"docker"`.
- **`mounts`** (optional): Custom bind mounts for Docker/Podman containers. Only valid with `environment: container`.
  - `source` — host path (supports `~` expansion)
  - `target` — container path
  - `readonly` — mount as read-only (default: false)
- **`env`** (optional): Environment variables passed into the container.
- **`dockerfile`** (optional): Path to a custom Dockerfile. Only valid with `environment: container`.

### Top-level defaults

Any profile field can also be declared at the top level. Top-level values act as defaults for every profile, and each profile overrides them field-by-field:

```yaml
default: worktree-zellij

container_runtime: podman
environment: container

mounts:
  - source: "~/.config/gcloud"
    target: "/home/claude/.config/gcloud"

profiles:
  shell:
    launch: shell
  claude:
    launch: claude
```

## mise integration

The container image ships with [mise](https://mise.jdx.dev/) pre-installed and a minimal OS base — no languages are bundled. Tools are installed at container startup based on the project's `mise.toml`.

### How it works

1. On `docker build`: only Debian slim + git + curl + mise are installed (fast, small image)
2. On `docker run` (entrypoint):
   - If `mise.toml` or `.mise.toml` exists in the workspace → `mise install` runs
   - If no mise config exists → only Node.js LTS is installed (required for Claude Code)
3. Installed tools are cached in the persistent volume (`claude-code-local` at `/home/claude/.local`), so subsequent startups are instant

### Example mise.toml

A `mise.toml.example` is included in this repository. It reproduces the original upstream Dockerfile's toolset:

```toml
[tools]
node = "22"
python = "3"
go = "1.23"
gh = "latest"
```

Copy it to your project as `mise.toml` and remove tools you don't need:

```bash
# Python project — only need node (for Claude Code) and python
cat > mise.toml << 'EOF'
[tools]
node = "22"
python = "3.14"
EOF
```

## What it does (Docker/Podman mode)

On first run with `environment: container`:

1. Builds a minimal container image (Debian slim + git + curl + mise)
2. Installs project tools via mise (cached in persistent volume)
3. Installs Claude Code into the persistent volume
4. Prompts you to log in via OAuth (browser-based)

On subsequent runs, it starts instantly with your existing authentication, tools, and settings.

### Zellij tab reuse

When `launch: zellij` is used and you are already inside a zellij session (`$ZELLIJ` is set), `aw` opens a **new tab** with the layout instead of creating a nested session.

## What gets synced into the container

When using `environment: container`, aw automatically handles host configuration so you don't have to set things up inside the container manually. Here's exactly what happens:

### Git — works out of the box

| Host | Container | How |
|------|-----------|-----|
| `~/.gitconfig` | `/home/claude/.gitconfig` | Bind mount (if exists) |

Your `user.name`, `user.email`, aliases, etc. are available as-is. No action needed.

### SSH — works out of the box

| Host | Container | How |
|------|-----------|-----|
| `~/.ssh/` | `/home/claude/.ssh/` | Mounted read-only → copied by entrypoint with correct permissions |

Private keys, `known_hosts`, `config` are all carried over. `git push` over SSH works immediately.

### GitHub CLI — works out of the box

| Host | Container | How |
|------|-----------|-----|
| `~/.config/gh/` | `/home/claude/.config/gh/` | Bind mount (if exists) |

`gh` commands (PR creation, issue management, etc.) work with your existing auth.

### Claude Code settings — synced (copied, not mounted)

| Host | Container | How |
|------|-----------|-----|
| `~/.claude/settings.json` | `/home/claude/.claude/settings.json` | Copied to `~/.agent-workspace/` then mounted |
| `~/.claude/CLAUDE.md` | `/home/claude/.claude/CLAUDE.md` | Same |
| `~/.claude/hooks/` | `/home/claude/.claude/hooks/` | Same |
| `~/.claude/plugins/` | `/home/claude/.claude/plugins/` | Same |
| `~/.claude/commands/` | `/home/claude/.claude/commands/` | Same |
| `~/.claude/agents/` | `/home/claude/.claude/agents/` | Same |

These are **copied** (not directly mounted) to `~/.agent-workspace/` on the host, which is then mounted into the container. This avoids conflicts with macOS Keychain-based credentials that don't work inside Linux containers.

### Claude Code authentication — separate per container

OAuth tokens are **not** synced from the host. The container's Claude Code performs its own OAuth login on first run. Credentials are stored in the persistent volume (`claude-code-local`), so you only authenticate once.

### Custom mounts — manual setup

Use the `mounts:` field in `.agent-workspace.yml` to mount additional directories (e.g., `~/.config/gcloud`). See [Profile options](#profile-options).

### Not synced

| Item | Workaround |
|------|------------|
| GPG keys (`~/.gnupg`) | Add via `mounts:` if you need signed commits |
| macOS Keychain | N/A — container uses its own OAuth flow |

### Project-level config

Your project's `.claude/settings.json` and `CLAUDE.md` are available automatically since the workspace directory is mounted into the container.

## Data storage

| Path | Purpose |
|------|---------|
| `~/.agent-workspace/` | Container-side Claude config (copied from `~/.claude/`) |
| `~/.agent-workspace.json` | Onboarding state |
| Volume `claude-code-local` | Claude Code installation + mise tool cache + OAuth credentials (persists across runs) |

## Uninstall

```bash
# Remove binary
rm ~/go/bin/agent-workspace ~/go/bin/aw

# Remove data
rm -rf ~/.agent-workspace ~/.agent-workspace.json

# Docker
docker rmi claude-code-docker
docker volume rm claude-code-local

# Or Podman
podman rmi claude-code-docker
podman volume rm claude-code-local
```

## Development

```bash
# Run tests
go test ./...

# Build
go build -o aw .

# Install locally
go install .

# Lint
golangci-lint run
```

## Requirements

### Host (required)

| Tool | When needed | Purpose |
|------|-------------|---------|
| `git` | `worktree` profiles | Worktree creation, repo root detection, remote fetch |
| `docker` or `podman` | `environment: container` profiles | Build image, create volume, run container |
| `zellij` | `launch: zellij` profiles | Multi-pane session |

### Host (additional — required by zellij layout panes)

When `launch: zellij` is used, the layout spawns helper panes that shell out to the following tools. Install the ones for the panes you actually use.

**`git-diff-picker` pane** — interactive diff viewer
- `fzf` — fuzzy picker (listen mode)
- `delta` — side-by-side diff renderer
- `lsof` — free-port detection for the fzf listen server
- `curl` — posts reload commands to the fzf server

**`pr-status` pane** — current branch's PR status
- `gh` (GitHub CLI) — fetches PR info and checks
- `jq` — parses PR JSON

**`plans-watcher` pane** — live Markdown preview of `plans/`
- `fswatch` **or** `entr` — file-change watcher (either works; fswatch preferred)
- `glow` *(optional)* — Markdown renderer; falls back to `cat` if missing

### Container (bundled — for reference)

The default container image includes only the minimal base. All development tools are installed via mise at runtime.

- Base: Debian bookworm-slim
- `git`, `curl`, `wget`, `ca-certificates`, `openssh-client`, `sudo`
- [mise](https://mise.jdx.dev/) — dev tool version manager
- Additional tools: installed from project's `mise.toml` (see [mise integration](#mise-integration))

### Optional

- `gpg` / `gpg-agent` — only if you sign git commits
- SSH keys / `ssh-agent` — only if you push/pull over SSH
