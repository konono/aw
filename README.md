# aw — fork

> **これは [hiragram/agent-workspace](https://github.com/hiragram/agent-workspace) のフォーク**であり、Podman サポート、カスタムマウント、mise 統合、各種改善を追加しています。

プロファイル設定可能なエージェントワークスペースを起動する CLI ツールです。Docker/Podman コンテナ、git worktree、zellij セッション、およびそれらの組み合わせに対応しています。

## フォークでの追加機能

| 機能 | 説明 |
|------|------|
| **Podman ネイティブサポート** | `container_runtime: podman` — ラッパースクリプト不要 |
| **カスタムマウント** | `mounts:` フィールドでホストの任意のディレクトリをコンテナにバインドマウント |
| **mise 統合** | プロジェクトの `mise.toml` でコンテナ内のツールインストールを制御 |
| **マルチOS テンプレート** | `os:` フィールドで Debian 12, UBI 9, UBI 10, Ubuntu 26.04 を選択可能 |
| **最小ベースイメージ** | Dockerfile を簡素化 — OS の必須パッケージ + mise のみ。言語はバンドルしない |
| **`volume create` Podman 修正** | Podman の非冪等な `volume create`（exit 125）を `--ignore` フラグで対処 |
| **Zellij タブ再利用** | zellij 内で実行した場合、セッションをネストせず新しいタブを開く |
| **`--help` フラグ** | `aw --help` / `aw -h` で使い方を表示 |
| **macOS テスト修正** | worktree フックテストでの `/var` → `/private/var` シンボリックリンク問題を修正 |

## インストール

### ソースから

```bash
go install github.com/konono/aw@latest
```

`aw` は `init` なしでも組み込みのスターター設定でそのまま使えます。設定を編集したくなったら `aw init` で `~/.config/aw/config.yml` に同じ内容を書き出せます。

```bash
aw        # 組み込みデフォルト（default: claude）で起動
aw init   # 組み込みスターター設定を ~/.config/aw/config.yml に書き出す
```

## 使い方

```bash
aw                      # デフォルトプロファイルを実行
aw <profile-name>       # 指定したプロファイルを実行
aw auth login claude    # Claude を認証（host に claude がなくても container で実行可能）
aw auth login codex     # Codex を認証
aw auth login opencode  # OpenCode を認証
aw auth status claude   # 認証状態確認
aw login claude         # `aw auth login claude` の短縮
aw profiles             # 利用可能なプロファイル一覧を表示
aw init                 # 組み込みスターター設定をグローバル設定として書き出す
aw default-dockerfile   # デフォルトの Dockerfile を出力
aw update               # セルフアップデート
aw --version            # バージョンを表示
aw --help               # ヘルプを表示
```

## 認証

`aw auth` は、**host 側に `claude` / `codex` / `opencode` が入っていなくても、そのアプリの認証を通すためのコマンド**です。

- `aw auth login claude|codex|opencode` は、既定では Debian container 側でその CLI の login を実行します。
- そのため、host 側に対象 CLI が未インストールでも認証できます。
- `aw auth login --profile <name>` を使うと、特定 profile の env / mounts / runtime を使って認証を実行できます。
- `auth` は token や API key を YAML に保存する場所ではありません。

### ツール別の早見表

- `codex`
  `aw auth login codex` は既定で `codex login --device-auth` を実行します。container や Podman machine では browser callback より安定するためです。`auth.codex.login_mode` で `browser` / `device` / `api-key` / `access-token` を選べます。container では `cli_auth_credentials_store = "file"` を既定にし、`auth.json` はホストから毎回上書きせず、必要時だけ seed します。

- `claude`
  `aw auth login claude` は `claude auth login` を実行します。通常の Claude CLI 認証状態は `~/.agent-workspace/claude` 経由で共有されるため、いったん container で認証すると、他の Claude container profile でもその状態を再利用できます。`CLAUDE_CODE_USE_VERTEX=1` や Bedrock / Foundry / `ANTHROPIC_API_KEY` / `CLAUDE_CODE_OAUTH_TOKEN` のような外部認証を使う場合は、`aw auth` ではなく `env` / `mounts` で管理してください。

- `opencode`
  `aw auth login opencode` は `opencode auth login` を実行します。`auth.opencode.provider` / `auth.opencode.method` を書くと対話プロンプトを減らせます。`aw auth status opencode` は `opencode auth list` を流します。

### `auth.on_launch.check` とは

これは `aw auth login ...` の設定ではなく、**通常の `aw <profile>` 起動直前に認証状態を status check する設定**です。

- `none`
  何もしません。

- `warn`
  未認証っぽければ警告だけ出して、そのまま起動します。

- `require`
  未認証っぽければ起動を止めます。

重要なのは、`auth.on_launch.check` は **status check だけ**であり、ログインを自動実行しないことです。CLI 管理の browser/device login に対して使う想定で、Vertex / Bedrock / API key のような external auth profile には付けないでください。

## 設定

> **[詳細な設定ガイド](docs/configuration.md)** — 全オプション、バリデーションルール、使用例のリファレンス。

`aw` は以下の順序で設定を読み込み、マージします:

```
ビルトインデフォルト → ~/.config/aw/config.yml（グローバル） → .agent-workspace.yml（プロジェクト）
```

| ファイル | 用途 |
|----------|------|
| `~/.config/aw/config.yml` | 全プロジェクト共通の設定（`container_runtime`、`env`、`mounts` など） |
| `.agent-workspace.yml` | プロジェクト固有の設定（git ルートまたはカレントディレクトリ） |

後に読まれた設定が優先されます。`.agent-workspace.yml` がないディレクトリでも、グローバル設定のプロファイルを使えます。

### 設定可能ポイントと優先順位

- 設定元は 5 つあります: ビルトインスターター設定、`~/.config/aw/config.yml`、`.agent-workspace.yml`、各ファイルのトップレベル shared defaults、各 `profiles.<name>`
- ソース優先順位は `ビルトイン < グローバル < プロジェクト` です
- 同じファイル内では `トップレベル shared defaults < profiles.<name>` です
- フィールドごとの merge ルールは一律ではなく、`env` は map merge、`worktree` / `zellij` は deep merge、`mounts` は置換、`mount_ssh` は明示 true/false override です
- `os` と `dockerfile` は最終的なプロファイル単位で排他的です。継承された値があっても、後から指定した側が反対側を置き換えます

`go install` 直後は、このビルトイン設定だけで `aw` を起動できます。グローバル設定ファイルが欲しい場合だけ `aw init` を実行してください。

以下は全パラメーターを含む設定例です（`.agent-workspace.yml` と `~/.config/aw/config.yml` の両方で同じ書式が使えます）:

```yaml
# デフォルトで実行するプロファイル名
default: worktree-zellij

# --- トップレベルのデフォルト値（全プロファイルで共有） ---
# ここに書いたフィールドは全プロファイルのデフォルトになります。
# 各プロファイルでフィールドごとに上書きできます。

environment: container             # "host" または "container"（必須）
container_runtime: podman          # "docker"（デフォルト）または "podman"
dockerfile: docker/Dockerfile.custom  # カスタム Dockerfile のパス（任意、各 profile で上書き可）

env:                               # コンテナに渡す環境変数（任意）
  CLAUDE_CODE_USE_VERTEX: "1"
  CLOUD_ML_REGION: "us-east5"

mount_ssh: true                    # ~/.ssh を読み取り専用でマウント（必要な場合のみ、デフォルト: false）
ssh_agent_forwarding: true                      # SSH Agent 転送で git push/clone を有効化（鍵ファイル不要、デフォルト: false）

mounts:                            # カスタムバインドマウント（任意、container のみ有効）
  - source: "~/.config/gcloud"     #   ホストパス（~ 展開に対応）
    target: "/home/agent/.config/gcloud"  #   コンテナパス
    readonly: false                #   読み取り専用でマウント（デフォルト: false）

profiles:
  claude:
    environment: container         # "host" または "container"（必須）
    launch: claude                 # "shell"、"claude"、または "zellij"（必須）
    container_runtime: podman      # "docker" または "podman"（任意）
    dockerfile: Dockerfile.dev     # カスタム Dockerfile のパス（任意）
    env:                           # 環境変数（任意、トップレベルとマージ）
      MY_VAR: "value"
    mounts:                        # カスタムマウント（任意、トップレベルを上書き）
      - source: "~/data"
        target: "/data"
        readonly: true

  codex:
    environment: container
    launch: codex
    auth:                          # 認証の振る舞い（任意）
      on_launch:
        check: warn                # 通常の `aw codex` 起動前に status check。warn は警告だけで続行
      codex:
        login_mode: device         # browser | device | api-key | access-token
        credentials_store: file    # file | keyring | auto
        seed_from_host: if_missing # if_missing | always | never
        persist_auth: stage        # 現在は stage 永続化のみ対応
        login_args:
          - --device-auth

  claude-vertex:
    environment: container
    launch: claude
    env:
      CLAUDE_CODE_USE_VERTEX: "1"
      CLOUD_ML_REGION: "us-east5"
      ANTHROPIC_VERTEX_PROJECT_ID: "my-gcp-project"
      GCP_PROJECT_ID: "my-gcp-project"
    mounts:
      - source: "~/.config/gcloud"
        target: "/home/agent/.config/gcloud"
        readonly: true
    # Vertex のような external auth profile では auth は書かない

  worktree-claude:
    worktree:                      # git worktree 設定（任意）
      base: origin/main            #   ベース ref（デフォルト: origin/main）
      dir: ~/worktrees/project     #   worktree 作成先（デフォルト: <repoRoot>/worktrees、~ 展開対応）
      on-create: "./scripts/setup.sh"  #   worktree 作成後に実行するシェルコマンド（任意）
      on-end: "./scripts/cleanup.sh"   #   起動プロセス終了後に実行するシェルコマンド（任意）
    environment: container
    launch: claude

  worktree-zellij:
    worktree: {}                   # 空オブジェクト = デフォルト設定で worktree を有効化
    environment: container
    launch: zellij                 # zellij セッションを起動
    zellij:                        # Zellij セッション設定（任意、launch: zellij の場合のみ有効）
      layout: default              #   レイアウト名（"default" またはカスタムパス）

  rhel10-shell:
    environment: container
    launch: shell
    os: ubi10                      # RHEL 10 (UBI 10) ベースのコンテナ

  host-shell:
    environment: host              # ホスト上で直接実行（コンテナなし）
    launch: shell
```

`.agent-workspace.yml` が見つからない場合、`aw` はビルトインのスターター設定を使用します。デフォルトでは `claude` プロファイル（Debian 12 + Podman）で起動し、`shell` / `codex` / `opencode` と `ubi9-shell` / `ubi10-shell` / `ubuntu2604-shell` もそのまま利用できます。必要なら `aw init` で同じ内容を `~/.config/aw/config.yml` に書き出して編集できます。

### プロファイルオプション

- **`worktree`**（任意）: git worktree を作成します。
  - `base` — 新しい worktree のベース ref。デフォルトは `origin/main`。
  - `dir` — worktree を作成するディレクトリ。デフォルトは `<repoRoot>/worktrees`。`~` 展開に対応。
  - `on-create` / `on-end` — worktree 作成後 / 起動プロセス終了後に実行されるシェルフック。
- **`environment`**（必須）: `"host"` または `"container"` — メインプロセスの実行環境。
- **`launch`**（必須）: `"shell"`、`"claude"`、`"codex"`、`"opencode"`、または `"zellij"` — 起動するもの。
- **`zellij`**（任意）: Zellij セッション設定。`launch: zellij` の場合のみ有効。`tool` には `"claude"`、`"codex"`、`"opencode"` を指定できます。
- **`auth`**（任意）: `aw auth ...` の既定動作と、必要なら通常起動前の認証チェックを指定します。秘密情報はここに書きません。
  - `on_launch.check` — 通常の `aw <profile>` 起動前に status check を行うかどうか。`none` / `warn` / `require`。
  - `codex.login_mode` — `browser` / `device` / `api-key` / `access-token`
  - `codex.credentials_store` — `file` / `keyring` / `auto`
  - `codex.seed_from_host` — `if_missing` / `always` / `never`
  - `claude.login_mode` — `browser` / `console` / `email` / `sso`
  - `opencode.provider` / `opencode.method` — `opencode auth login` の既定値
  - Vertex / Bedrock / API key など env/mounts ベースの外部認証では、`auth` を省略するのが基本です。
- **`container_runtime`**（任意）: `"docker"` または `"podman"`。デフォルトは `"docker"`。
- **`mount_ssh`**（任意）: ホストの `~/.ssh` を読み取り専用でコンテナへ持ち込む明示 opt-in。デフォルトは `false`。トップレベルに置けば全プロファイルの既定値になり、各プロファイルで `true` / `false` を個別に上書きできます。
- **`ssh_agent_forwarding`**（任意）: ホストの SSH Agent ソケットをコンテナに転送し、Git SSH 操作（push/clone/fetch）を有効化。`mount_ssh` と異なり、鍵ファイルはコンテナにコピーされません。ホスト側で SSH Agent が起動している必要があります（`ssh-add -l` で確認）。`mount_ssh: true` の場合は無視されます（`mount_ssh` が上位互換）。デフォルトは `false`。
- **`mounts`**（任意）: Docker/Podman コンテナ用のカスタムバインドマウント。`environment: container` の場合のみ有効。
  - `source` — ホストパス（`~` 展開に対応）
  - `target` — コンテナパス
  - `readonly` — 読み取り専用でマウント（デフォルト: false）
- **`env`**（任意）: コンテナに渡す環境変数。
- **`os`**（任意）: コンテナイメージのベース OS。`"debian12"`（デフォルト）、`"ubi9"`、`"ubi10"`、`"ubuntu2604"`。`environment: container` の場合のみ有効。`dockerfile` と排他的。
- **`dockerfile`**（任意）: カスタム Dockerfile のパス。`environment: container` の場合のみ有効。`os` と排他的。Dockerfile が置かれたディレクトリ全体がビルドコンテキストになるため、`COPY` で同じディレクトリ内のファイルを参照できます。

### トップレベルのデフォルト値

プロファイルの各フィールドはトップレベルにも宣言できます。トップレベルの値は全プロファイルのデフォルトとなり、各プロファイルがフィールドごとに上書きします:

```yaml
default: worktree-zellij

container_runtime: podman
environment: container
mount_ssh: true

mounts:
  - source: "~/.config/gcloud"
    target: "/home/agent/.config/gcloud"

profiles:
  shell:
    launch: shell
  claude:
    launch: claude
```

`os` と `dockerfile` は最終的なプロファイル単位で排他的です。トップレベルで片方を設定していても、各プロファイルで反対側を指定すると継承値を置き換えます。

## カスタム Dockerfile

`dockerfile` フィールドでカスタム Dockerfile を指定すると、通常の `docker build` と同じ感覚でイメージをカスタマイズできます。Dockerfile が置かれた**ディレクトリ全体**がビルドコンテキストになるため、`COPY` で同じディレクトリ内のファイルを自由に参照できます。

### 例: Playwright 対応コンテナ

このリポジトリには `playwright-docker/` ディレクトリに実例があります:

```
playwright-docker/
├── Dockerfile       ← dockerfile: で指定
├── entrypoint.sh    ← COPY entrypoint.sh で参照可能
└── mise.toml        ← コンテナ内で使うツール定義
```

#### 設定例

```yaml
profiles:
  playwright:
    environment: container
    launch: claude
    dockerfile: playwright-docker/Dockerfile  # git ルートからの相対パス
```

#### Dockerfile (`playwright-docker/Dockerfile`)

```dockerfile
FROM debian:bookworm-slim

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
      git curl ca-certificates wget openssh-client sudo \
      # Playwright browser dependencies
      libglib2.0-0 libnspr4 libnss3 libatk1.0-0 libatk-bridge2.0-0 \
      libdbus-1-3 libx11-6 libxcomposite1 libxdamage1 libxext6 \
      libxfixes3 libxrandr2 libgbm1 libxcb1 libpango-1.0-0 \
      libcairo2 libasound2 libcups2 libdrm2 libxshmfence1 \
      libxkbcommon0 libatspi2.0-0 && \
    rm -rf /var/lib/apt/lists/*

RUN useradd -m -s /bin/bash agent && \
    echo 'agent ALL=(ALL) NOPASSWD:ALL' >> /etc/sudoers

RUN su -s /bin/bash agent -c 'curl https://mise.jdx.dev/install.sh | sh'

ENV PATH="/home/agent/.local/bin:/home/agent/.local/share/mise/shims:${PATH}"

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

WORKDIR /workspace
ENTRYPOINT ["/entrypoint.sh"]
CMD ["claude"]
```

entrypoint.sh で Playwright ブラウザの自動インストールも行われます。詳細は `playwright-docker/entrypoint.sh` を参照してください。

デフォルトの Dockerfile と entrypoint.sh は `aw default-dockerfile` で確認できます。これをベースにカスタマイズするのが最も簡単です。

## mise 統合

コンテナイメージには [mise](https://mise.jdx.dev/) がプリインストールされており、最小限の OS ベースのみが含まれています — 言語はバンドルされていません。ツールはコンテナ起動時にプロジェクトの `mise.toml` に基づいてインストールされます。

### 仕組み

1. `docker build` 時: Debian slim + git + curl + mise のみインストール（高速で軽量なイメージ）
2. `docker run` 時（エントリポイント）:
   - ワークスペースに `mise.toml` または `.mise.toml` がある → `mise install` を実行
   - mise 設定がない → Node.js LTS のみインストール（Claude Code に必要）
3. インストールされたツールは永続ボリューム（`/home/agent/.local` の `claude-code-local`）にキャッシュされるため、2回目以降の起動は即座に完了

### mise.toml の例

このリポジトリに `mise.toml.example` が含まれています。上流の元の Dockerfile のツールセットを再現します:

```toml
[tools]
node = "22"
python = "3"
go = "1.23"
gh = "latest"
```

プロジェクトに `mise.toml` としてコピーし、不要なツールを削除してください:

```bash
# Python プロジェクト — node（Claude Code 用）と python のみ必要
cat > mise.toml << 'EOF'
[tools]
node = "22"
python = "3.14"
EOF
```

## 動作の詳細（Docker/Podman モード）

`environment: container` での初回実行時:

1. 最小コンテナイメージをビルド（Debian slim + git + curl + mise）
2. mise でプロジェクトのツールをインストール（永続ボリュームにキャッシュ）
3. 永続ボリュームに Claude Code をインストール
4. OAuth でのログインを促す（ブラウザベース）

2回目以降は、既存の認証・ツール・設定を使用して即座に起動します。

### Zellij タブ再利用

`launch: zellij` を使用中に既に zellij セッション内にいる場合（`$ZELLIJ` が設定済み）、`aw` はネストしたセッションを作成する代わりに、レイアウト付きの**新しいタブ**を開きます。

## コンテナに同期されるもの

`environment: container` を使用する場合、aw はホストの設定を自動的に処理するため、コンテナ内で手動設定する必要はありません。具体的には以下の通りです:

### Git — そのまま動作

| ホスト | コンテナ | 方法 |
|--------|----------|------|
| `~/.gitconfig` | `/home/agent/.gitconfig` | バインドマウント（存在する場合） |

`user.name`、`user.email`、エイリアスなどがそのまま利用できます。設定不要です。

### SSH — 2つの方式

Git SSH 操作（push/clone/fetch）には `ssh_agent_forwarding` と `mount_ssh` の2つの方式があります:

| 方式 | 設定 | 仕組み | 鍵ファイルの露出 |
|------|------|--------|------------------|
| **SSH Agent 転送** | `ssh_agent_forwarding: true` | ホストの SSH Agent ソケット（`SSH_AUTH_SOCK`）をコンテナに転送 | なし（推奨） |
| **SSH ディレクトリマウント** | `mount_ssh: true` | `~/.ssh/` 全体を読み取り専用でマウント → エントリポイントでコピー | あり（鍵ファイルがコンテナ内に存在） |

`ssh_agent_forwarding: true` は鍵ファイルをコンテナに入れずに Git SSH 認証を通す軽量な方式です。ホスト側で SSH Agent が起動し、鍵が登録されている必要があります（`ssh-add -l` で確認）。`mount_ssh: true` と同時に設定した場合は `mount_ssh` が優先されます。

`mount_ssh: true` はサーバーへの SSH ログインなど、Git 以外の SSH 操作も必要な場合に使います。

**プラットフォームごとの動作（`ssh_agent_forwarding`）:**

| 環境 | 仕組み |
|------|--------|
| Linux (Docker/Podman) | `$SSH_AUTH_SOCK` を直接 bind mount |
| macOS + Docker Desktop | Docker Desktop のファイル共有経由で `$SSH_AUTH_SOCK` を bind mount |
| macOS + Podman | `ssh -R` で Podman VM にソケット転送 + SELinux モジュール自動インストール（下記参照） |

#### macOS + Podman での SSH Agent 転送の仕組み

macOS + Podman では、SSH Agent ソケットが Podman VM から直接アクセスできないため、`aw` が以下の処理を自動的に行います:

1. **SSH トンネル確立**: `ssh -R` で macOS の `$SSH_AUTH_SOCK` を Podman VM 内の `/tmp/aw-ssh-agent.sock` に転送
2. **SELinux モジュールインストール** (初回のみ): コンテナ (`container_t`) が VM 内のソケットに接続するためのカスタムポリシー `aw_agent_sock` を VM にインストール
3. **ソケットマウント**: VM 内のソケットをコンテナに bind mount し、`SSH_AUTH_SOCK` 環境変数を設定
4. **クリーンアップ**: コンテナ終了時に SSH トンネルプロセスを停止し、VM 内のソケットを削除

**Podman VM に設定されるもの:**

| 対象 | 内容 | 永続性 |
|------|------|--------|
| `/tmp/aw-ssh-agent.sock` | SSH Agent 転送ソケット | コンテナ終了時に削除 |
| SELinux module `aw_agent_sock` | `container_t` → `unconfined_t` ソケット接続許可 | 永続（VM 再起動後も有効） |

SELinux モジュールの内容:
```
allow container_t unconfined_t:unix_stream_socket connectto;
allow container_t user_tmp_t:sock_file { read write getattr };
```

### GitHub CLI — そのまま動作

| ホスト | コンテナ | 方法 |
|--------|----------|------|
| `~/.config/gh/` | `/home/agent/.config/gh/` | バインドマウント（存在する場合） |

`gh` コマンド（PR 作成、Issue 管理など）が既存の認証で動作します。

### Claude Code 設定 — 同期して stage をマウント

| ホスト | コンテナ | 方法 |
|--------|----------|------|
| `~/.claude/settings.json` | `/home/agent/.claude/settings.json` | `~/.agent-workspace/claude/` に同期してからマウント |
| `~/.claude/CLAUDE.md` | `/home/agent/.claude/CLAUDE.md` | 同上 |
| `~/.claude/hooks/` | `/home/agent/.claude/hooks/` | 同上 |
| `~/.claude/plugins/` | `/home/agent/.claude/plugins/` | 同上 |
| `~/.claude/commands/` | `/home/agent/.claude/commands/` | 同上 |
| `~/.claude/agents/` | `/home/agent/.claude/agents/` | 同上 |

これらはホスト上の `~/.agent-workspace/claude/` に**同期**され、その stage がコンテナの `/home/agent/.claude/` にマウントされます。ホストの `~/.claude/` 本体を直接マウントしないのは、container 用の patch を入れつつ、Linux コンテナ内で動作しない macOS キーチェーン系の状態と切り離すためです。

container 内で Claude Code が設定や補助ファイルを書き換えた場合、その変更は `~/.agent-workspace/claude/` に保存されます。ホストの `~/.claude/` へ自動で書き戻しはしません。

`settings.json` の `statusLine.command` や hooks などで Claude 設定配下のファイルを参照する場合は、host の絶対パス（例: `/Users/name/.claude/...`）ではなく、`~/.claude/...` または相対パスを使ってください。container 内の正式な設定パスは `/home/agent/.claude/...` です。

### Claude Code 認証 — コンテナごとに独立

OAuth トークンはホストからは同期**されません**。コンテナの Claude Code は初回実行時に独自の OAuth ログインを行います。認証情報は永続ボリューム（`claude-code-local`）に保存されるため、認証は一度だけで済みます。

### Codex 設定と認証 — stage に永続化

Codex は `~/.codex/` の一部を `~/.agent-workspace/codex/` にコピーしてからコンテナへマウントします。

- `config.toml` は毎回同期しつつ、container 側では `cli_auth_credentials_store = "file"` を既定にします。
- `auth.json` はホストから**毎回上書きしません**。既定では stage 側に無ければ seed し、以降は container で更新されたものを保持します。
- そのため、container 内で `codex login` した結果や token refresh の結果は、次回 `aw codex` でも使い続けられます。
- `auth.codex.seed_from_host` で `if_missing` / `always` / `never` を切り替えられます。

### カスタムマウント — 手動設定

`.agent-workspace.yml` の `mounts:` フィールドを使用して追加ディレクトリをマウントします（例: `~/.config/gcloud`）。[プロファイルオプション](#プロファイルオプション)を参照してください。

### 同期されないもの

| 項目 | 回避策 |
|------|--------|
| GPG 鍵（`~/.gnupg`） | 署名付きコミットが必要な場合は `mounts:` で追加 |
| macOS キーチェーン | N/A — コンテナは独自の OAuth フローを使用 |

### プロジェクトレベルの設定

プロジェクトの `.claude/settings.json` と `CLAUDE.md` は、ワークスペースディレクトリがコンテナにマウントされるため自動的に利用可能です。

## データ保存先

| パス | 用途 |
|------|------|
| `~/.agent-workspace/claude/` | Claude 設定の stage（`~/.claude/` から同期。container の書き込みもここに保存） |
| `~/.agent-workspace/codex/` | Codex 設定と認証状態の stage |
| `~/.agent-workspace/opencode/` | OpenCode 設定とデータの stage |
| `~/.agent-workspace/claude/.claude.json` | Claude のオンボーディング状態 |
| ボリューム `claude-code-local` | Claude Code のインストール + mise ツールキャッシュ + OAuth 認証情報（実行間で永続化） |

## アンインストール

```bash
# バイナリの削除
rm ~/go/bin/aw

# データの削除
rm -rf ~/.agent-workspace

# Docker の場合
docker rmi claude-code-docker
docker volume rm claude-code-local

# Podman の場合
podman rmi claude-code-docker
podman volume rm claude-code-local
```

## 開発

```bash
# テスト実行
go test ./...

# ビルド
go build -o aw .

# ローカルインストール
go install .

# リント
golangci-lint run
```

## 必要なツール

### ホスト（必須）

| ツール | 必要な場面 | 用途 |
|--------|-----------|------|
| `git` | `worktree` プロファイル | worktree 作成、リポジトリルート検出、リモート fetch |
| `docker` または `podman` | `environment: container` プロファイル | イメージビルド、ボリューム作成、コンテナ実行 |
| `zellij` | `launch: zellij` プロファイル | マルチペインセッション |

### ホスト（追加 — zellij レイアウトペインに必要）

`launch: zellij` を使用する場合、レイアウトは以下のツールを使用するヘルパーペインを起動します。実際に使用するペインに必要なものをインストールしてください。

**`git-diff-picker` ペイン** — インタラクティブ diff ビューア
- `fzf` — ファジーピッカー（リッスンモード）
- `delta` — サイドバイサイド diff レンダラー
- `lsof` — fzf リッスンサーバー用の空きポート検出
- `curl` — fzf サーバーにリロードコマンドを送信

**`pr-status` ペイン** — 現在のブランチの PR ステータス
- `gh`（GitHub CLI）— PR 情報とチェックを取得
- `jq` — PR の JSON をパース

**`plans-watcher` ペイン** — `plans/` のライブ Markdown プレビュー
- `fswatch` **または** `entr` — ファイル変更ウォッチャー（どちらでも可。fswatch 推奨）
- `glow` *（任意）* — Markdown レンダラー。ない場合は `cat` にフォールバック

### コンテナ（同梱 — 参考情報）

デフォルトのコンテナイメージには最小限のベースのみ含まれます。すべての開発ツールは mise によりランタイムでインストールされます。

- ベース: Debian bookworm-slim
- `git`、`curl`、`wget`、`ca-certificates`、`openssh-client`、`sudo`
- [mise](https://mise.jdx.dev/) — 開発ツールバージョンマネージャー
- 追加ツール: プロジェクトの `mise.toml` からインストール（[mise 統合](#mise-統合)を参照）

### オプション

- `gpg` / `gpg-agent` — git コミットに署名する場合のみ
- SSH 鍵 / `ssh-agent` — SSH 経由で push/pull する場合のみ
