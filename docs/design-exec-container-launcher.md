# 設計: コンテナランチャーの wrapper 廃止と syscall.Exec 化

## 背景

PR #64 で、`aw` wrapper が SIGTERM/SIGHUP を `podman run` に転送し、コンテナ内のプロセス（Claude Code 等）が exit status 143 で死亡する問題を修正した。

しかしレビューで wrapper 方式そのものに構造的な問題が指摘された:

1. **PR #64 の射程が狭い** --- `aw` から `podman run` へのシグナル転送は止めたが、両者は同じ process group にいるため、端末クローズ時の SIGHUP は `podman run` に直接届く。`aw` の転送有無とは無関係。
2. **SIGTERM 吸収の副作用** --- `kill <aw-pid>` で `aw` を止められなくなる。Unix の慣習に反する。
3. **wrapper が不要な可能性** --- host モードでは既に `syscall.Exec` でプロセスを置き換えている。container モードでも同じことができればシグナル問題は根本解消する。

## 現在の構造

### Host モード（既に exec 化済み）

```
shell -> aw -> syscall.Exec -> claude/bash
```

`aw` は環境準備後にプロセスを置き換える。シグナル転送の問題は存在しない。

関連コード:
- `internal/launcher/shell.go:43` --- `syscall.Exec(shellPath, ...)`
- `internal/launcher/tool.go:53` --- `syscall.Exec(binPath, ...)`

### Container モード（wrapper が残る）

```
shell -> aw -> exec.Command("podman run ...") -> cmd.Wait()
```

`aw` はコンテナ実行中ずっとプロセスとして残る。`internal/docker/client.go:Run()` でシグナルハンドリングを行い、PR #64 適用後は SIGINT のみ転送する。

## exec 化の阻害要因

`syscall.Exec` は現在のプロセスを置き換えるため、`internal/cmd/root.go:193` の `runCleanups(ec)` が実行されなくなる。失われる後処理は 2 つ:

### 1. SSH agent cleanup（macOS + Podman 限定）

**ファイル**: `internal/sshagent/podman_darwin.go`

macOS + Podman 環境では、`SSH_AUTH_SOCK` を Podman VM に転送するために SSH tunnel（`ssh -f -N -R ...`）を起動する。cleanup 関数では:
- tunnel プロセスに SIGTERM を送信
- VM 内の remote socket（`/tmp/aw-ssh-agent.sock`）を削除

**影響範囲**: `runtime.GOOS == "darwin" && containerRuntime == "podman"` の場合のみ。Linux や macOS + Docker では cleanup は no-op（`sshagent.go:37`）。

### 2. Worktree on-end hook

**ファイル**: `internal/stage/worktree.go:RunOnEndHook()`

コンテナ終了後にホスト側でユーザー定義のシェルコマンドを実行する。`worktree.on-end` がプロファイルに設定されている場合のみ有効。

**前例**: host モードでは既に on-end hook が実行されない旨の警告を出している（`root.go:168-171`）。

## 設計: cleanup watcher + exec

上記 2 つの後処理を**分離した watcher プロセス**に任せることで、wrapper を完全に廃止できる。

### アーキテクチャ

```
aw
├── fork → cleanup watcher（パイプの read 側を持ち、podman 終了を待機）
└── exec → podman run   （パイプの write 側を継承、終了時に自動で閉じる）
```

### パイプベース watcher の動作

1. `aw` がパイプを作成
2. `aw` が cleanup watcher を子プロセスとして起動し、パイプの read 側を渡す
3. `aw` がパイプ write 側の `O_CLOEXEC` を解除（exec 後も継承させるため）
4. `aw` が `syscall.Exec` で自身を `podman run` に置き換える
5. `podman run` が実行される（パイプの write 側を知らずに継承している）
6. `podman run` が終了 → OS が write 側を自動で閉じる
7. watcher がパイプから EOF を検知 → cleanup 実行 → 終了

### 実装スケッチ

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

watcher は `aw` の隠しサブコマンドとして実装する:

```go
// aw --internal-cleanup-watcher
func runCleanupWatcher() {
    // fd 3 = ExtraFiles で渡されたパイプの read 側
    pipe := os.NewFile(3, "pipe")

    // podman 終了まで block（write 側が閉じると EOF）
    buf := make([]byte, 1)
    pipe.Read(buf)

    // 後処理を実行
    runSSHAgentCleanup()
    runOnEndHook()
}
```

watcher の起動処理:

```go
func spawnCleanupWatcher(cleanups []func()) error {
    r, w, err := os.Pipe()
    if err != nil {
        return err
    }

    cmd := exec.Command(os.Args[0], "--internal-cleanup-watcher")
    cmd.ExtraFiles = []*os.File{r}          // fd 3
    cmd.SysProcAttr = &syscall.SysProcAttr{
        Setpgid: true,                      // フォアグラウンドの process group から分離
    }
    if err := cmd.Start(); err != nil {
        r.Close()
        w.Close()
        return err
    }
    r.Close()

    // O_CLOEXEC を解除して exec 後も write 側が podman に継承されるようにする
    _, _, errno := syscall.RawSyscall(
        syscall.SYS_FCNTL, w.Fd(), syscall.F_SETFD, 0,
    )
    if errno != 0 {
        return fmt.Errorf("fcntl F_SETFD: %w", errno)
    }

    return nil
}
```

### watcher へのデータ受け渡し

watcher は何を cleanup すべきか知る必要がある。2 つのアプローチ:

**A. 自己完結型**: watcher がプロファイル/設定を再読み込みして cleanup 内容を自分で判断する。シンプルだが、watcher が設定全体を理解する必要がある。

**B. シリアライズ方式**: `aw` が cleanup の指示（SSH tunnel PID、on-end コマンド等）を一時ファイルまたは環境変数で watcher に渡す。明示的でテストしやすい。

推奨: **B** --- JSON で cleanup 仕様を渡す:

```go
type CleanupSpec struct {
    SSHTunnelPID   int    `json:"ssh_tunnel_pid,omitempty"`
    VMSocketPath   string `json:"vm_socket_path,omitempty"`
    OnEndCommand   string `json:"on_end_command,omitempty"`
    WorktreePath   string `json:"worktree_path,omitempty"`
}
```

### 旧案（段階的移行）との比較

旧案では 3 フェーズ（cleanup 不要時のみ exec → SSH tunnel 自律化 → on-end hook 対応）を提案していた。これは wrapper と exec の 2 つのコードパスを維持する必要があった。

cleanup watcher 方式はこれを一本化する:

| 観点 | 旧案（段階的） | watcher 方式 |
|------|--------------|-------------|
| コードパス | 2 つ（wrapper + exec）、条件分岐 | 1 つ（常に exec） |
| SSH tunnel cleanup | 自律終了の再設計が必要 | watcher が tunnel を kill |
| on-end hook | 警告のみ、または別プロセス | watcher が実行 |
| シグナルハンドリング | wrapper fallback では依然必要 | 不要（wrapper がないため） |
| 複雑性 | エッジケースが増えると増大 | 一定 |

### エッジケース

**podman 終了前に watcher が kill された場合**: cleanup が実行されない。現在の wrapper が kill される場合と同じリスク。多層防御として SSH keepalive タイムアウト（`-o ServerAliveInterval=15 -o ServerAliveCountMax=3`）を tunnel に設定し、安全策とする。

**podman が即座に終了した場合（起動失敗）**: パイプの write 側がすぐ閉じるため、watcher が即座に cleanup を実行する。正常に動作する。

**複数の aw インスタンス**: 各インスタンスが独自のパイプ + watcher を持つ。干渉しない。

**macOS 固有の問題**: `SYS_FCNTL` + `F_SETFD` は macOS（Darwin）で動作する。`Setpgid` も同様。移植性の問題はない。

## まとめ

| 項目 | 内容 |
|------|------|
| **何を** | `exec.Command + Wait` を `syscall.Exec` に置き換え、パイプベースの cleanup watcher で後処理を実行 |
| **なぜ** | wrapper プロセス、シグナル転送バグ、SIGTERM 吸収の問題を根本的に解消 |
| **リスク** | 低 --- パイプ + fork は確立された Unix パターン |
| **対象範囲** | `internal/docker/client.go`（新規 `ExecRun`）、`--internal-cleanup-watcher` サブコマンド新設、launcher 変更 |
| **cleanup カバレッジ** | SSH tunnel（macOS+Podman）+ on-end hook --- 現在の wrapper と同等 |

## 関連

- PR #64 --- 暫定修正（SIGTERM/SIGHUP の podman run への転送を停止）
