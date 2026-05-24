# コンテナ同期ガイド

`environment: container` を使用する場合、`aw` はホストの設定を自動的にコンテナへ同期します。

## Git — そのまま動作

| ホスト | コンテナ | 方法 |
|--------|----------|------|
| `~/.gitconfig` | `/home/agent/.gitconfig` | バインドマウント（存在する場合） |

`user.name`、`user.email`、エイリアスなどがそのまま利用できます。

## GitHub CLI — オプトイン（デフォルト無効）

| ホスト | コンテナ | 方法 | 条件 |
|--------|----------|------|------|
| `~/.config/gh/` | `/home/agent/.config/gh/` | 読み取り専用バインドマウント | `mount_gh: true` の場合のみ |

`mount_gh: true` を設定すると、`gh` コマンド（PR 作成、Issue 管理など）が既存の認証で動作します。

**デフォルトでは無効です。** gh トークンは AI エージェントから読み取り可能であり、プロンプトインジェクション攻撃により外部に流出するリスクがあるためです。有効にする場合は、スコープを絞ったトークンの使用を推奨します。

```yaml
# トップレベルで全プロファイルに適用
mount_gh: true

# または特定プロファイルのみ
profiles:
  claude:
    mount_gh: true
```

## SSH — 2つの方式

Git SSH 操作（push/clone/fetch）には2つの方式があります:

| 方式 | 設定 | 仕組み | 鍵ファイルの露出 |
|------|------|--------|------------------|
| **SSH Agent 転送** | `ssh_agent_forwarding: true` | ホストの SSH Agent ソケットをコンテナに転送 | なし（推奨） |
| **SSH ディレクトリマウント** | `mount_ssh: true` | `~/.ssh/` 全体を読み取り専用でマウント | あり |

`ssh_agent_forwarding` は鍵ファイルをコンテナに入れずに Git SSH 認証を通す軽量な方式です。ホスト側で SSH Agent が起動し、鍵が登録されている必要があります（`ssh-add -l` で確認）。

`mount_ssh` はサーバーへの SSH ログインなど、Git 以外の SSH 操作も必要な場合に使います。両方設定した場合は `mount_ssh` が優先されます。

### プラットフォームごとの動作（`ssh_agent_forwarding`）

| 環境 | 仕組み |
|------|--------|
| Linux (Docker/Podman) | `$SSH_AUTH_SOCK` を直接 bind mount |
| macOS + Docker Desktop | Docker Desktop のファイル共有経由で `$SSH_AUTH_SOCK` を bind mount |
| macOS + Podman | `ssh -R` で Podman VM にソケット転送 + SELinux モジュール自動インストール |

#### macOS + Podman での SSH Agent 転送

macOS + Podman では SSH Agent ソケットが Podman VM から直接アクセスできないため、`aw` が以下を自動的に行います:

1. **SSH トンネル確立**: `ssh -R` で macOS の `$SSH_AUTH_SOCK` を Podman VM 内の `/tmp/aw-ssh-agent.sock` に転送
2. **SELinux モジュールインストール** (初回のみ): カスタムポリシー `aw_agent_sock` を VM にインストール
3. **ソケットマウント**: VM 内のソケットをコンテナに bind mount し、`SSH_AUTH_SOCK` を設定
4. **クリーンアップ**: コンテナ終了時に SSH トンネルを停止しソケットを削除

SELinux モジュールの内容:
```
allow container_t unconfined_t:unix_stream_socket connectto;
allow container_t user_tmp_t:sock_file { read write getattr };
```

## Claude Code 設定 — 同期して stage をマウント

| ホスト | コンテナ |
|--------|----------|
| `~/.claude/settings.json` | `/home/agent/.claude/settings.json` |
| `~/.claude/CLAUDE.md` | `/home/agent/.claude/CLAUDE.md` |
| `~/.claude/hooks/` | `/home/agent/.claude/hooks/` |
| `~/.claude/plugins/` | `/home/agent/.claude/plugins/` |
| `~/.claude/commands/` | `/home/agent/.claude/commands/` |
| `~/.claude/agents/` | `/home/agent/.claude/agents/` |

これらはホスト上の `~/.agent-workspace/claude/` に**同期**され、その stage がコンテナの `/home/agent/.claude/` にマウントされます。ホストの `~/.claude/` 本体を直接マウントしないのは、container 用の patch を入れつつ、Linux コンテナ内で動作しない macOS キーチェーン系の状態と切り離すためです。

コンテナ内で Claude Code が設定を書き換えた場合、変更は `~/.agent-workspace/claude/` に保存されます。ホストの `~/.claude/` へ自動で書き戻しはしません。

### Claude Code 認証 — コンテナごとに独立

OAuth トークンはホストからは同期**されません**。コンテナの Claude Code は初回実行時に独自の OAuth ログインを行います。認証情報は永続ボリューム（`claude-code-local`）に保存されるため、認証は一度だけで済みます。

## Codex 設定と認証 — stage に永続化

Codex は `~/.codex/` の一部を `~/.agent-workspace/codex/` にコピーしてからコンテナへマウントします。

- `config.toml` は毎回同期しつつ、container 側では `cli_auth_credentials_store = "file"` を既定にします
- `auth.json` はホストから毎回上書きしません（既定では stage 側に無ければ seed し、以降は container で更新されたものを保持）
- `auth.codex.seed_from_host` で `if_missing` / `always` / `never` を切り替え可能

## データ保存先

| パス | 用途 |
|------|------|
| `~/.agent-workspace/claude/` | Claude 設定の stage |
| `~/.agent-workspace/codex/` | Codex 設定と認証状態の stage |
| `~/.agent-workspace/opencode/` | OpenCode 設定とデータの stage |
| ボリューム `claude-code-local` | Claude Code インストール + mise ツールキャッシュ + OAuth 認証情報 |

## 同期されないもの

| 項目 | 回避策 |
|------|--------|
| GPG 鍵（`~/.gnupg`） | 署名付きコミットが必要な場合は `mounts:` で追加 |
| macOS キーチェーン | N/A — コンテナは独自の OAuth フローを使用 |

プロジェクトの `.claude/settings.json` と `CLAUDE.md` は、ワークスペースディレクトリがコンテナにマウントされるため自動的に利用可能です。
