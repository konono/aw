# aw — fork

> **これは [hiragram/agent-workspace](https://github.com/hiragram/agent-workspace) のフォーク**であり、Podman サポート、カスタムマウント、mise 統合、各種改善を追加しています。

プロファイル設定可能なエージェントワークスペースを起動する CLI ツールです。Docker/Podman コンテナ、git worktree、zellij セッション、およびそれらの組み合わせに対応しています。

## フォークでの追加機能

| 機能 | 説明 |
|------|------|
| **Podman ネイティブサポート** | `container_runtime: podman` — ラッパースクリプト不要 |
| **カスタムマウント** | `mounts:` フィールドでホストの任意のディレクトリをコンテナにバインドマウント |
| **mise 統合** | プロジェクトの `mise.toml` でコンテナ内のツールインストールを制御 |
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

## 使い方

```bash
aw                      # デフォルトプロファイルを実行
aw <profile-name>       # 指定したプロファイルを実行
aw profiles             # 利用可能なプロファイル一覧を表示
aw default-dockerfile   # デフォルトの Dockerfile を出力
aw update               # セルフアップデート
aw --version            # バージョンを表示
aw --help               # ヘルプを表示
```

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

以下は全パラメーターを含む設定例です（`.agent-workspace.yml` と `~/.config/aw/config.yml` の両方で同じ書式が使えます）:

```yaml
# デフォルトで実行するプロファイル名
default: worktree-zellij

# --- トップレベルのデフォルト値（全プロファイルで共有） ---
# ここに書いたフィールドは全プロファイルのデフォルトになります。
# 各プロファイルでフィールドごとに上書きできます。

environment: container             # "host" または "container"（必須）
container_runtime: podman          # "docker"（デフォルト）または "podman"
dockerfile: docker/Dockerfile.custom  # カスタム Dockerfile のパス（任意）

env:                               # コンテナに渡す環境変数（任意）
  CLAUDE_CODE_USE_VERTEX: "1"
  CLOUD_ML_REGION: "us-east5"

mounts:                            # カスタムバインドマウント（任意、container のみ有効）
  - source: "~/.config/gcloud"     #   ホストパス（~ 展開に対応）
    target: "/home/claude/.config/gcloud"  #   コンテナパス
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

  host-shell:
    environment: host              # ホスト上で直接実行（コンテナなし）
    launch: shell
```

`.agent-workspace.yml` が見つからない場合、`aw` はビルトインのデフォルト設定を使用します。worktree を作成し、Docker ベースの Claude で zellij 開発環境を起動します。

### プロファイルオプション

- **`worktree`**（任意）: git worktree を作成します。
  - `base` — 新しい worktree のベース ref。デフォルトは `origin/main`。
  - `dir` — worktree を作成するディレクトリ。デフォルトは `<repoRoot>/worktrees`。`~` 展開に対応。
  - `on-create` / `on-end` — worktree 作成後 / 起動プロセス終了後に実行されるシェルフック。
- **`environment`**（必須）: `"host"` または `"container"` — メインプロセスの実行環境。
- **`launch`**（必須）: `"shell"`、`"claude"`、または `"zellij"` — 起動するもの。
- **`zellij`**（任意）: Zellij セッション設定。`launch: zellij` の場合のみ有効。
- **`container_runtime`**（任意）: `"docker"` または `"podman"`。デフォルトは `"docker"`。
- **`mounts`**（任意）: Docker/Podman コンテナ用のカスタムバインドマウント。`environment: container` の場合のみ有効。
  - `source` — ホストパス（`~` 展開に対応）
  - `target` — コンテナパス
  - `readonly` — 読み取り専用でマウント（デフォルト: false）
- **`env`**（任意）: コンテナに渡す環境変数。
- **`dockerfile`**（任意）: カスタム Dockerfile のパス。`environment: container` の場合のみ有効。Dockerfile が置かれたディレクトリ全体がビルドコンテキストになるため、`COPY` で同じディレクトリ内のファイルを参照できます。

### トップレベルのデフォルト値

プロファイルの各フィールドはトップレベルにも宣言できます。トップレベルの値は全プロファイルのデフォルトとなり、各プロファイルがフィールドごとに上書きします:

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

## カスタム Dockerfile

`dockerfile` フィールドでカスタム Dockerfile を指定すると、通常の `docker build` と同じ感覚でイメージをカスタマイズできます。Dockerfile が置かれた**ディレクトリ全体**がビルドコンテキストになるため、`COPY` で同じディレクトリ内のファイルを自由に参照できます。

### ディレクトリ構成例

```
docker/
├── Dockerfile.custom   ← dockerfile: で指定
├── entrypoint.sh       ← COPY entrypoint.sh で参照可能
└── scripts/
    └── setup.sh        ← COPY scripts/setup.sh で参照可能
```

### 設定例

```yaml
profiles:
  custom:
    environment: container
    launch: claude
    dockerfile: docker/Dockerfile.custom  # git ルートからの相対パス
```

### Dockerfile の例

```dockerfile
FROM debian:bookworm-slim

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
      git curl ca-certificates sudo && \
    rm -rf /var/lib/apt/lists/*

RUN useradd -m -s /bin/bash claude && \
    echo 'claude ALL=(ALL) NOPASSWD:ALL' >> /etc/sudoers

# 自前のスクリプトをコピー
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh
COPY scripts/setup.sh /usr/local/bin/setup.sh

WORKDIR /workspace
ENTRYPOINT ["/entrypoint.sh"]
CMD ["claude"]
```

デフォルトの Dockerfile と entrypoint.sh は `aw default-dockerfile` で確認できます。これをベースにカスタマイズするのが最も簡単です。

## mise 統合

コンテナイメージには [mise](https://mise.jdx.dev/) がプリインストールされており、最小限の OS ベースのみが含まれています — 言語はバンドルされていません。ツールはコンテナ起動時にプロジェクトの `mise.toml` に基づいてインストールされます。

### 仕組み

1. `docker build` 時: Debian slim + git + curl + mise のみインストール（高速で軽量なイメージ）
2. `docker run` 時（エントリポイント）:
   - ワークスペースに `mise.toml` または `.mise.toml` がある → `mise install` を実行
   - mise 設定がない → Node.js LTS のみインストール（Claude Code に必要）
3. インストールされたツールは永続ボリューム（`/home/claude/.local` の `claude-code-local`）にキャッシュされるため、2回目以降の起動は即座に完了

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
| `~/.gitconfig` | `/home/claude/.gitconfig` | バインドマウント（存在する場合） |

`user.name`、`user.email`、エイリアスなどがそのまま利用できます。設定不要です。

### SSH — そのまま動作

| ホスト | コンテナ | 方法 |
|--------|----------|------|
| `~/.ssh/` | `/home/claude/.ssh/` | 読み取り専用でマウント → エントリポイントで正しいパーミッションでコピー |

秘密鍵、`known_hosts`、`config` がすべて引き継がれます。SSH 経由の `git push` が即座に動作します。

### GitHub CLI — そのまま動作

| ホスト | コンテナ | 方法 |
|--------|----------|------|
| `~/.config/gh/` | `/home/claude/.config/gh/` | バインドマウント（存在する場合） |

`gh` コマンド（PR 作成、Issue 管理など）が既存の認証で動作します。

### Claude Code 設定 — 同期（コピー、マウントではない）

| ホスト | コンテナ | 方法 |
|--------|----------|------|
| `~/.claude/settings.json` | `/home/claude/.claude/settings.json` | `~/.agent-workspace/` にコピーしてからマウント |
| `~/.claude/CLAUDE.md` | `/home/claude/.claude/CLAUDE.md` | 同上 |
| `~/.claude/hooks/` | `/home/claude/.claude/hooks/` | 同上 |
| `~/.claude/plugins/` | `/home/claude/.claude/plugins/` | 同上 |
| `~/.claude/commands/` | `/home/claude/.claude/commands/` | 同上 |
| `~/.claude/agents/` | `/home/claude/.claude/agents/` | 同上 |

これらはホスト上の `~/.agent-workspace/` に**コピー**（直接マウントではなく）され、それがコンテナにマウントされます。Linux コンテナ内で動作しない macOS キーチェーンベースの認証情報との競合を避けるためです。

### Claude Code 認証 — コンテナごとに独立

OAuth トークンはホストからは同期**されません**。コンテナの Claude Code は初回実行時に独自の OAuth ログインを行います。認証情報は永続ボリューム（`claude-code-local`）に保存されるため、認証は一度だけで済みます。

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
| `~/.agent-workspace/` | コンテナ側の Claude 設定（`~/.claude/` からコピー） |
| `~/.agent-workspace.json` | オンボーディング状態 |
| ボリューム `claude-code-local` | Claude Code のインストール + mise ツールキャッシュ + OAuth 認証情報（実行間で永続化） |

## アンインストール

```bash
# バイナリの削除
rm ~/go/bin/aw

# データの削除
rm -rf ~/.agent-workspace ~/.agent-workspace.json

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
