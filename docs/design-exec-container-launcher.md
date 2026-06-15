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

## Proposed phases

### Phase 1: exec when no cleanup is needed (low risk, high coverage)

Use `syscall.Exec` to replace `aw` with `podman/docker run` when ALL of:
- `ec.SSHAgentCleanup == nil` (not macOS + Podman, or SSH agent not configured)
- `ec.Profile.Worktree == nil || ec.Profile.Worktree.OnEnd == ""` (no on-end hook)

This covers the majority of usage. The wrapper/wait path remains as fallback.

```go
// In launchDockerShell / launchContainer:
if ec.SSHAgentCleanup == nil && (ec.Profile.Worktree == nil || ec.Profile.Worktree.OnEnd == "") {
    // exec: replace aw with podman/docker
    args := docker.BuildRunArgs(runConfig)
    bin, _ := exec.LookPath(runtime)
    return syscall.Exec(bin, append([]string{runtime}, args...), os.Environ())
}
// fallback: wrapper mode
return client.Run(ctx, runConfig)
```

### Phase 2: make SSH tunnel self-terminating

Eliminate the need for aw-side cleanup, enabling exec on macOS + Podman too.

| Approach | How it works | Tradeoff |
|----------|-------------|----------|
| **A. Entrypoint trap** | `entrypoint.sh` runs `trap` to delete VM socket on exit. SSH tunnel loses its forward target and exits. | Simplest. Relies on SSH behavior for tunnel cleanup. |
| **B. Pidfile** | `aw` writes tunnel PID to a known file. Next `aw` invocation detects and kills stale tunnels. | Works with exec. Cleanup is deferred to next run. |
| **C. SSH keepalive timeout** | Use `-o ServerAliveInterval=15 -o ServerAliveCountMax=3`. Tunnel exits ~45s after VM connectivity loss. | Lazy cleanup. Good as defense-in-depth alongside A or B. |
| **D. Parent death signal** | Start tunnel without `-f`, use `prctl(PR_SET_PDEATHSIG)` equivalent. | Not portable to macOS (no prctl). |

**Recommendation**: A (entrypoint trap) + C (keepalive as safety net). This makes the tunnel self-contained without any wrapper-side cleanup.

### Phase 3: on-end hook parity

Apply the same approach as host mode: warn that on-end hooks won't run with exec mode. Since host mode already accepts this limitation, container mode can do the same.

Alternatively, spawn a detached cleanup process before exec:

```go
// spawn a background process that waits for podman to exit, then runs on-end
cleanupCmd := exec.Command("sh", "-c", fmt.Sprintf("wait %d; %s", podmanPID, onEndCommand))
cleanupCmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
cleanupCmd.Start()
// then exec podman
```

This is more complex and probably not worth it for the current use case.

## Summary

| Phase | What | Risk | Effect |
|-------|------|------|--------|
| 1 | Exec when cleanup-free | Low | Eliminates signal problem for most users |
| 2 | Self-terminating SSH tunnel | Medium | Enables exec on macOS + Podman |
| 3 | On-end hook warning | Low | Full exec coverage |

## Related

- PR #64 --- local fix (stop SIGTERM/SIGHUP forwarding)
