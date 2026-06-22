# 認証ガイド

`aw auth` は、**ホスト側にツールがインストールされていなくても、コンテナ内で認証を実行するためのコマンド**です。

## 基本コマンド

```bash
aw auth login claude       # Claude を認証
aw auth login codex        # Codex を認証
aw auth login opencode     # OpenCode を認証
aw auth login cursor       # Cursor CLI を認証
aw auth status claude      # 認証状態を確認
aw auth logout claude      # ログアウト
aw login claude            # aw auth login claude の短縮形
```

`--profile <name>` を付けると、特定プロファイルの env / mounts / runtime を使って認証を実行できます。

> **注意:** `auth` は token や API key を YAML に保存する場所ではありません。

## ツール別の詳細

### Claude

`aw auth login claude` は `claude auth login` を実行します。Claude CLI の認証状態は `~/.agent-workspace/claude/` 経由で共有されるため、いったんコンテナで認証すると、他の Claude コンテナプロファイルでもその状態を再利用できます。

**外部認証（Vertex / Bedrock / API key）を使う場合:**

`CLAUDE_CODE_USE_VERTEX=1` や `ANTHROPIC_API_KEY` のような外部認証を使う場合は、`aw auth` ではなく `env` / `mounts` で管理してください。

### Codex

`aw auth login codex` は既定で `codex login --device-auth` を実行します。コンテナや Podman machine ではブラウザコールバックより安定するためです。

設定可能なオプション:

| オプション | 値 | 説明 |
|-----------|-----|------|
| `auth.codex.login_mode` | `browser` / `device` / `api-key` / `access-token` | ログイン方式 |
| `auth.codex.credentials_store` | `file` / `keyring` / `auto` | 認証情報の保存先 |
| `auth.codex.seed_from_host` | `if_missing` / `always` / `never` | ホストから認証情報をコピーするタイミング |

コンテナでは `cli_auth_credentials_store = "file"` を既定にし、`auth.json` はホストから毎回上書きせず、必要時だけ seed します。

### OpenCode

`aw auth login opencode` は `opencode auth login` を実行します。`auth.opencode.provider` / `auth.opencode.method` を設定すると対話プロンプトを減らせます。`aw auth status opencode` は `opencode auth list` を実行します。

### Cursor

`aw auth login cursor` は `agent login` を実行します。OAuth トークンは macOS では Keychain から、Linux では `~/.config/cursor/auth.json` からコンテナ staging に seed されます。API キーで使う場合は `env: CURSOR_API_KEY` を設定し、`auth` を省略できます。`auth.on_launch.check` は Cursor では未対応です。

## `auth.on_launch.check`

これは `aw auth login` の設定ではなく、**通常の `aw <profile>` 起動直前に認証状態をチェックする設定**です。

| 値 | 動作 |
|----|------|
| `none` | 何もしない |
| `warn` | 未認証なら警告を出して続行 |
| `require` | 未認証なら起動を停止 |

`auth.on_launch.check` は **status check のみ**であり、ログインを自動実行しません。CLI 管理の browser/device login に対して使う想定で、Vertex / Bedrock / API key のような外部認証プロファイルには付けないでください。
