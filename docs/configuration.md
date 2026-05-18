# Configuration Guide

This document describes how `aw` loads, merges, and validates `.agent-workspace.yml` style configuration.

## Overview

`aw` can launch built-in starter profiles with no config files at all. When config files are present, it merges them with the built-in starter config and then materializes an effective profile for the requested profile name.

## File locations

`aw` reads configuration from these locations:

1. Built-in starter config embedded in the binary
2. `~/.config/aw/config.yml`
3. `<git-root>/.agent-workspace.yml`

If `git rev-parse --show-toplevel` fails, `aw` falls back to `.agent-workspace.yml` in the current directory.

Run `aw init` if you want to write the current built-in starter config to `~/.config/aw/config.yml` and customize it there.

## Resolution And Precedence

There are two precedence axes:

1. **Source precedence**
   Built-in starter config < global config < project config
2. **Within a single file**
   Top-level shared defaults < `profiles.<name>`

That means a later file wins over an earlier one, and within the same file an explicit field on `profiles.<name>` wins over the top-level value for that field.

### Setting points

Configuration can come from the following places:

- The embedded starter config in `internal/profile/embed/config.yml`
- `~/.config/aw/config.yml`
- `.agent-workspace.yml`
- Top-level shared defaults in any of those files
- Per-profile overrides under `profiles.<name>`

### Merge model

At a high level, `aw` resolves configuration like this:

1. Read the built-in starter config
2. Overlay `~/.config/aw/config.yml` if present
3. Overlay `.agent-workspace.yml` if present
4. Apply the final top-level shared defaults to every profile
5. Validate the resulting effective profiles

### Field-specific merge rules

- `env` is merged key by key, with later values winning
- `worktree` is merged field by field
- `zellij` is merged field by field
- `mounts` is replaced as a whole when specified
- `mount_ssh` uses explicit tri-state behavior:
  - omitted: inherit
  - `true`: enable
  - `false`: disable
- `ssh_agent_forwarding` uses the same tri-state behavior as `mount_ssh`
- `os` and `dockerfile` are mutually exclusive at the final profile level; if one is inherited and the other is specified later, the later one clears the inherited counterpart

## YAML shape

There is no nested `defaults:` block. Shared defaults stay flat at the top level:

```yaml
default: claude

environment: container
container_runtime: podman
mount_ssh: false

profiles:
  claude:
    launch: claude

  shell:
    launch: shell
```

`profiles.<name>` uses the same field names as the top level. The difference is semantic:

- top-level fields are shared defaults
- `profiles.<name>` fields are profile-specific overrides

## Minimal example

```yaml
profiles:
  my-profile:
    environment: container
    launch: claude
```

## Full example

```yaml
default: worktree-zellij

environment: container
container_runtime: podman
mount_ssh: false
ssh_agent_forwarding: true
env:
  CLAUDE_CODE_USE_VERTEX: "1"

profiles:
  claude:
    launch: claude

  codex:
    launch: codex

  opencode:
    launch: opencode

  host-shell:
    environment: host
    launch: shell

  worktree-zellij:
    worktree: {}
    launch: zellij
    zellij:
      layout: default
      tool: codex

  ubi10-shell:
    launch: shell
    os: ubi10

  playwright:
    launch: claude
    dockerfile: playwright-docker/Dockerfile
    mounts:
      - source: "~/.config/gcloud"
        target: "/home/agent/.config/gcloud"
        readonly: true
```

## Top-level keys

### `default`

The profile name used when you run `aw` without arguments. If omitted, `aw` lists available profiles instead of launching one.

### Shared defaults

Any profile field can also appear at the top level. These top-level fields become shared defaults for every profile in the merged config.

Common top-level defaults include:

- `environment`
- `container_runtime`
- `env`
- `mount_ssh`
- `ssh_agent_forwarding`
- `mounts`
- `os`
- `dockerfile`
- `worktree`
- `zellij`

### `profiles`

A required map of named profiles. Each key is a profile name, and each value is a partial or complete profile definition.

## Profile fields

### `environment` (required)

Controls where the main process runs.

- `host` - run directly on the host
- `container` - run inside the aw container image

### `launch` (required)

Controls what `aw` launches.

- `shell`
- `claude`
- `codex`
- `opencode`
- `zellij`

### `worktree` (optional)

If present, `aw` creates a git worktree before launch.

`worktree: {}` enables worktree mode with defaults.

Supported fields:

- `base` - default `origin/main`
- `dir` - directory to host worktrees
- `on-create` - shell command run after creating the worktree
- `on-end` - shell command run after the launched process exits

Available environment variables for hooks:

- `AW_WORKTREE_PATH`
- `AW_WORKTREE_BRANCH`
- `AW_REPO_ROOT`
- `AW_PROFILE_NAME`
- `AW_ENVIRONMENT`

### `zellij` (optional)

Only valid when `launch: zellij`.

Supported fields:

- `layout` - default `default`
- `tool` - one of `claude`, `codex`, or `opencode`

### `env` (optional)

Additional environment variables passed into the launched environment. Top-level and per-profile `env` values are merged.

### `os` (optional)

Built-in container OS template. Valid values:

- `debian12`
- `ubi9`
- `ubi10`
- `ubuntu2604`

Only valid with `environment: container`. Mutually exclusive with `dockerfile`.

### `dockerfile` (optional)

Path to a custom Dockerfile, relative to the git root unless absolute. Only valid with `environment: container`. Mutually exclusive with `os`.

### `container_runtime` (optional)

Container CLI to use:

- `docker`
- `podman`

If omitted, the effective runtime defaults to `docker`.

### `mount_ssh` (optional)

Whether to mount host `~/.ssh` into the container as read-only input. The container entrypoint copies it to `/home/agent/.ssh` with fixed permissions. Provides full SSH access (server login, key-based authentication, etc.).

If omitted, the field inherits from top-level defaults. The built-in starter config sets this to `false`.

### `ssh_agent_forwarding` (optional)

Whether to forward the host's SSH agent into the container for Git SSH operations (push, clone, fetch). Unlike `mount_ssh`, this does not copy SSH key files into the container — only the SSH agent socket (`SSH_AUTH_SOCK`) is forwarded.

Requires the host to have an SSH agent running with keys loaded (`ssh-add -l` to verify).

If `mount_ssh: true` is also set, `ssh_agent_forwarding` is ignored because `mount_ssh` already provides full SSH access.

If omitted, the field inherits from top-level defaults. The built-in starter config sets this to `false`.

**Platform notes:**
- Linux (Docker/Podman): works by mounting `$SSH_AUTH_SOCK` directly
- macOS + Docker Desktop: works via Docker Desktop file sharing
- macOS + Podman: `aw` automatically establishes an SSH tunnel into the Podman VM and installs a minimal SELinux policy module (`aw_agent_sock`) on first use. See the README for details on what is set up inside the VM.

### `mounts` (optional)

Additional bind mounts for container profiles.

Each mount supports:

- `source`
- `target`
- `readonly`

## Built-in starter config

When no config files exist, `aw` behaves as if the built-in starter config were loaded. The starter config currently provides:

- `claude`
- `shell`
- `codex`
- `opencode`
- `ubi9-shell`
- `ubi10-shell`
- `ubuntu2604-shell`

The built-in default profile is `claude`.

You can materialize the current starter config into `~/.config/aw/config.yml` with:

```bash
aw init
```

## Validation rules

`aw` validates effective profiles on each run.

Current rules include:

1. At least one profile must exist
2. `default`, if set, must point to an existing profile
3. `environment` is required and must be `host` or `container`
4. `launch` is required and must be `shell`, `claude`, `codex`, `opencode`, or `zellij`
5. `zellij` is only valid with `launch: zellij`
6. `zellij.tool` must be `claude`, `codex`, or `opencode`
7. `os` must be one of the supported built-in templates
8. `os` is only valid with `environment: container`
9. `dockerfile` is only valid with `environment: container`
10. `os` and `dockerfile` are mutually exclusive
11. `container_runtime` must be `docker` or `podman`
12. `mounts` are only valid with `environment: container`
13. Every mount must include both `source` and `target`
14. `ssh_agent_forwarding` is only valid with `environment: container`

## Host settings synced into containers

When using `environment: container`, `aw` automatically handles common host-side settings:

- `~/.gitconfig` -> `/home/agent/.gitconfig`
- `~/.config/gh` -> `/home/agent/.config/gh`
- `~/.claude/settings.json` -> `/home/agent/.claude/settings.json`
- `~/.claude/CLAUDE.md` -> `/home/agent/.claude/CLAUDE.md`
- `~/.claude/hooks` -> `/home/agent/.claude/hooks`
- `~/.claude/plugins` -> `/home/agent/.claude/plugins`
- `~/.claude/commands` -> `/home/agent/.claude/commands`
- `~/.claude/agents` -> `/home/agent/.claude/agents`

`~/.ssh` is not mounted by default. Set `mount_ssh: true` for full SSH access, or `ssh_agent_forwarding: true` to forward only the SSH agent for Git operations.

## Tips

- Use `aw profiles` to see available profiles and where they were loaded from
- Use `aw init` to create a starting global config only when you want to customize it
- Keep profile names short and descriptive
- Commit `.agent-workspace.yml` when the team should share the same profiles
