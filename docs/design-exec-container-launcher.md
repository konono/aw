# Design: Replace container launcher wrapper with syscall.Exec

## Background

PR #64 fixed a bug where the `aw` wrapper forwarded SIGTERM/SIGHUP to `podman run`, causing containers (and processes inside them, e.g. Claude Code) to die with exit status 143.

However, review identified structural issues with the wrapper approach itself:

1. **PR #64's scope is narrow** --- it only prevents signal forwarding from `aw` to `podman run`. Since both processes share the same process group, terminal-close SIGHUP reaches `podman run` directly, bypassing `aw` entirely.
2. **SIGTERM swallowing side effect** --- `kill <aw-pid>` no longer terminates `aw`, which violates Unix conventions.
3. **The wrapper may be unnecessary** --- host mode already uses `syscall.Exec` to replace `aw` with the launched process. Container mode could do the same.

## Current structure

### Host mode (already uses exec)

```
shell -> aw -> syscall.Exec -> claude/bash
```

`aw` prepares the environment, then replaces itself. No signal forwarding needed.

Relevant code:
- `internal/launcher/shell.go:43` --- `syscall.Exec(shellPath, ...)`
- `internal/launcher/tool.go:53` --- `syscall.Exec(binPath, ...)`

### Container mode (wrapper stays alive)

```
shell -> aw -> exec.Command("podman run ...") -> cmd.Wait()
```

`aw` stays alive for the entire container lifetime. Signal handling in `internal/docker/client.go:Run()` catches SIGTERM/SIGHUP/SIGINT and (after PR #64) only forwards SIGINT.

## What blocks exec for container mode

`syscall.Exec` replaces the current process, so `runCleanups(ec)` in `internal/cmd/root.go:193` never runs. Two cleanup responsibilities would be lost:

### 1. SSH agent cleanup (macOS + Podman only)

**File**: `internal/sshagent/podman_darwin.go`

On macOS + Podman, `aw` starts an SSH tunnel (`ssh -f -N -R ...`) to forward `SSH_AUTH_SOCK` into the Podman VM. The cleanup function:
- Sends SIGTERM to the tunnel process
- Removes the remote socket (`/tmp/aw-ssh-agent.sock`) from the VM

**Scope**: Only affects `runtime.GOOS == "darwin" && containerRuntime == "podman"`. On Linux, or macOS + Docker, cleanup is a no-op (`sshagent.go:37`).

### 2. Worktree on-end hook

**File**: `internal/stage/worktree.go:RunOnEndHook()`

Runs a user-defined shell command on the host after the container exits. Only active when `worktree.on-end` is set in the profile.

**Precedent**: Host mode already warns that on-end hooks won't run (`root.go:168-171`).

## Design: cleanup watcher + exec

Both cleanup responsibilities can be handled by a **detached watcher process** that runs alongside `podman run`, eliminating the need for a wrapper entirely.

### Architecture

```
aw
├── fork → cleanup watcher (reads from pipe, waits for podman exit)
└── exec → podman run    (inherits pipe write end, closed on exit)
```

### How the pipe-based watcher works

1. `aw` creates a pipe
2. `aw` spawns a cleanup watcher as a child process, passing the pipe read end
3. `aw` clears `O_CLOEXEC` on the pipe write end so it survives exec
4. `aw` calls `syscall.Exec` to replace itself with `podman run`
5. `podman run` runs (inheriting the write end of the pipe without knowing about it)
6. When `podman run` exits, the write end is closed automatically by the OS
7. The watcher reads EOF from the pipe → runs cleanup → exits

### Implementation sketch

```go
func (c *ShellClient) ExecRun(config RunConfig, cleanups []func()) error {
    args := BuildRunArgs(config)
    binPath, err := exec.LookPath(c.dockerCmd())
    if err != nil {
        return err
    }

    if len(cleanups) > 0 {
        if err := spawnCleanupWatcher(cleanups); err != nil {
            return fmt.Errorf("cleanup watcher: %w", err)
        }
    }

    argv := append([]string{c.dockerCmd()}, args...)
    return syscall.Exec(binPath, argv, os.Environ())
}
```

The watcher itself is a hidden subcommand of `aw`:

```go
// aw --internal-cleanup-watcher
func runCleanupWatcher() {
    // fd 3 = pipe read end, passed via ExtraFiles
    pipe := os.NewFile(3, "pipe")

    // Block until podman exits (pipe write end closes → EOF)
    buf := make([]byte, 1)
    pipe.Read(buf)

    // Run all cleanup tasks
    runSSHAgentCleanup()
    runOnEndHook()
}
```

Spawning the watcher before exec:

```go
func spawnCleanupWatcher(cleanups []func()) error {
    r, w, err := os.Pipe()
    if err != nil {
        return err
    }

    cmd := exec.Command(os.Args[0], "--internal-cleanup-watcher")
    cmd.ExtraFiles = []*os.File{r}          // fd 3
    cmd.SysProcAttr = &syscall.SysProcAttr{
        Setpgid: true,                      // detach from foreground process group
    }
    if err := cmd.Start(); err != nil {
        r.Close()
        w.Close()
        return err
    }
    r.Close()

    // Clear O_CLOEXEC so the write end survives exec into podman
    _, _, errno := syscall.RawSyscall(
        syscall.SYS_FCNTL, w.Fd(), syscall.F_SETFD, 0,
    )
    if errno != 0 {
        return fmt.Errorf("fcntl F_SETFD: %w", errno)
    }

    return nil
}
```

### Watcher data passing

The watcher needs to know what cleanup to run. Two approaches:

**A. Self-contained subcommand**: The watcher re-reads the profile/config and determines cleanup tasks itself. Simple but requires the watcher to understand the full config.

**B. Serialized instructions**: `aw` writes cleanup instructions (SSH tunnel PID, on-end command, etc.) to a temp file or passes them as arguments/env vars to the watcher. More explicit and testable.

Recommended: **B** --- pass a JSON blob via env or temp file:

```go
type CleanupSpec struct {
    SSHTunnelPID   int    `json:"ssh_tunnel_pid,omitempty"`
    VMSocketPath   string `json:"vm_socket_path,omitempty"`
    OnEndCommand   string `json:"on_end_command,omitempty"`
    WorktreePath   string `json:"worktree_path,omitempty"`
}
```

### Why this is better than the phased approach

The previous design proposed 3 phases: (1) exec when no cleanup needed, (2) make SSH tunnel self-terminating, (3) handle on-end hooks. This required maintaining two code paths (wrapper + exec) and incremental migration.

The cleanup watcher approach eliminates all phases:

| Concern | Phased approach | Watcher approach |
|---------|----------------|-----------------|
| Code paths | Two (wrapper + exec), conditional | One (always exec) |
| SSH tunnel cleanup | Needs self-terminating redesign | Watcher kills tunnel on exit |
| On-end hook | Warning or separate process | Watcher runs it |
| Signal handling | Still needed in wrapper fallback | Not needed (no wrapper) |
| Complexity | Grows with edge cases | Constant |

### Edge cases

**Watcher is killed before podman exits**: Cleanup doesn't run. Same risk as the current wrapper being killed. Defense-in-depth: SSH keepalive timeout (`-o ServerAliveInterval=15 -o ServerAliveCountMax=3`) as safety net for tunnel cleanup.

**Podman exits immediately (startup failure)**: Pipe write end closes right away, watcher runs cleanup immediately. Works correctly.

**Multiple aw instances**: Each gets its own pipe + watcher. No interference.

**macOS specifics**: `SYS_FCNTL` with `F_SETFD` works on macOS (Darwin). `Setpgid` also works. No portability issues.

## Summary

| Item | Description |
|------|-------------|
| **What** | Replace `exec.Command + Wait` with `syscall.Exec`, using a pipe-based cleanup watcher for post-exit tasks |
| **Why** | Eliminates wrapper process, signal forwarding bugs, and SIGTERM swallowing |
| **Risk** | Low --- pipe + fork is a well-established Unix pattern |
| **Scope** | `internal/docker/client.go` (new `ExecRun`), new `--internal-cleanup-watcher` subcommand, launcher changes |
| **Cleanup coverage** | SSH tunnel (macOS+Podman) + on-end hooks --- same as current wrapper |

## Related

- PR #64 --- interim fix (stop SIGTERM/SIGHUP forwarding to podman run)
