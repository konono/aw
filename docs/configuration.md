# 設定リファレンス

`aw` が `.agent-workspace.yml` 形式の設定をどのように読み込み、マージし、バリデーションするかを説明します。

## 概要

`aw` は設定ファイルがなくても組み込みのスタータープロファイルで起動できます。設定ファイルがある場合は、組み込み設定とマージして、指定されたプロファイル名の有効なプロファイルを生成します。

## ファイルの場所

`aw` は以下の場所から設定を読み込みます:

1. バイナリに埋め込まれた組み込みスターター設定
2. `~/.config/aw/config.yml`
3. `<git ルート>/.agent-workspace.yml`

`git rev-parse --show-toplevel` が失敗した場合、カレントディレクトリの `.agent-workspace.yml` にフォールバックします。

組み込みスターター設定を `~/.config/aw/config.yml` に書き出してカスタマイズしたい場合は `aw init` を実行してください。

## 解決と優先順位

優先順位は2つの軸で決まります:

1. **ソース優先順位**
   組み込みスターター設定 < グローバル設定 < プロジェクト設定
2. **同じファイル内**
   トップレベルの共有デフォルト < `profiles.<name>`

つまり、後に読まれたファイルが優先され、同じファイル内では `profiles.<name>` の明示的なフィールドがトップレベルの値より優先されます。

### 設定ポイント

設定は以下の場所から来ます:

- `internal/profile/embed/config.yml` に埋め込まれたスターター設定
- `~/.config/aw/config.yml`
- `.agent-workspace.yml`
- 上記いずれかのファイルのトップレベル共有デフォルト
- `profiles.<name>` のプロファイルごとのオーバーライド

### マージモデル

`aw` は設定を以下の手順で解決します:

1. 組み込みスターター設定を読み込む
2. `~/.config/aw/config.yml` があればオーバーレイ
3. `.agent-workspace.yml` があればオーバーレイ
4. 最終的なトップレベル共有デフォルトを全プロファイルに適用
5. 結果の有効なプロファイルをバリデーション

### フィールドごとのマージルール

- `env` はキーごとにマージ（後の値が優先）
- `worktree` はフィールドごとにマージ
- `zellij` はフィールドごとにマージ
- `mounts` は指定時に全体を置換
- `mount_gh` は明示的な三値動作:
  - 省略: 継承
  - `true`: 有効化
  - `false`: 無効化
- `mount_ssh` は `mount_gh` と同じ三値動作
- `ssh_agent_forwarding` は `mount_gh` と同じ三値動作
- `os` と `dockerfile` は最終的なプロファイルレベルで排他的。片方が継承され、もう片方が後から指定された場合、継承された側はクリアされる

## YAML の構造

ネストした `defaults:` ブロックはありません。共有デフォルトはトップレベルにフラットに置きます:

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

`profiles.<name>` はトップレベルと同じフィールド名を使います。違いはセマンティクスです:

- トップレベルのフィールドは共有デフォルト
- `profiles.<name>` のフィールドはプロファイル固有のオーバーライド

## 最小限の例

```yaml
profiles:
  my-profile:
    environment: container
    launch: claude
```

## フルの例

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
    auth:
      on_launch:
        check: warn
      codex:
        login_mode: device
        credentials_store: file
        seed_from_host: if_missing

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

  claude-vertex:
    launch: claude
    env:
      CLAUDE_CODE_USE_VERTEX: "1"
    mounts:
      - source: "~/.config/gcloud"
        target: "/home/agent/.config/gcloud"

  playwright:
    launch: claude
    dockerfile: playwright-docker/Dockerfile
    mounts:
      - source: "~/.config/gcloud"
        target: "/home/agent/.config/gcloud"
        mode: ro
```

## トップレベルキー

### `default`

引数なしで `aw` を実行したときに使用するプロファイル名。省略した場合、`aw` は起動せずに利用可能なプロファイル一覧を表示します。

### 共有デフォルト

プロファイルのフィールドはトップレベルにも置けます。トップレベルのフィールドはマージ後の全プロファイルの共有デフォルトになります。

よく使われるトップレベルデフォルト:

- `environment`
- `container_runtime`
- `auth`
- `env`
- `mount_gh`
- `mount_ssh`
- `ssh_agent_forwarding`
- `mounts`
- `os`
- `dockerfile`
- `worktree`
- `zellij`

### `profiles`

名前付きプロファイルの必須マップ。各キーがプロファイル名、各値が部分的または完全なプロファイル定義です。

## プロファイルフィールド

### `environment`（必須）

メインプロセスの実行場所を制御します。

- `host` — ホスト上で直接実行
- `container` — aw コンテナイメージ内で実行

### `launch`（必須）

`aw` が起動するものを制御します。

- `shell`
- `claude`
- `codex`
- `opencode`
- `zellij`

### `worktree`（任意）

指定した場合、`aw` は起動前に git worktree を作成します。

`worktree: {}` でデフォルト設定の worktree モードを有効化します。

サポートされるフィールド:

- `base` — デフォルト `origin/main`
- `dir` — worktree を作成するディレクトリ
- `on-create` — worktree 作成後に実行するシェルコマンド
- `on-end` — 起動プロセス終了後に実行するシェルコマンド

フックで利用可能な環境変数:

- `AW_WORKTREE_PATH`
- `AW_WORKTREE_BRANCH`
- `AW_REPO_ROOT`
- `AW_PROFILE_NAME`
- `AW_ENVIRONMENT`

### `zellij`（任意）

`launch: zellij` の場合のみ有効です。

サポートされるフィールド:

- `layout` — デフォルト `default`
- `tool` — `claude`、`codex`、または `opencode` のいずれか

### `auth`（任意）

プロファイルに対する `aw auth ...` の動作と、起動時の認証状態チェック（任意）を制御します。

通常は `aw auth login claude|codex|opencode` がメインのエントリポイントです。このコマンドはデフォルトの Debian コンテナを使うため、ホストにツールがインストールされていなくても動作します。

Codex の場合、`aw auth login codex` はデフォルトで `codex login --device-auth` を実行します。コンテナや Podman machine 内ではブラウザコールバックが不安定なためです。明示的に localhost コールバックフローを使いたい場合のみ `auth.codex.login_mode: browser` を設定してください。

特定プロファイルの `env`、`mounts`、runtime 設定で認証を実行する必要がある場合のみ `aw auth ... --profile <name>` を使用してください。

`auth` はトークンや API key を保存しません。それらは各 CLI 独自の認証ストアまたは外部クラウド認証情報に保存されます。

`env`、`mounts`、プロバイダー固有の認証情報による外部認証を使うプロファイルでは、`auth` を完全に省略してください。

例:

```yaml
profiles:
  codex:
    environment: container
    launch: codex
    auth:
      on_launch:
        check: warn
      codex:
        login_mode: device
        credentials_store: file
        seed_from_host: if_missing
        persist_auth: stage
        login_args:
          - --device-auth
```

サポートされるフィールド:

- `on_launch.check` — `none`、`warn`、または `require`
  - `none`: チェックをスキップ
  - `warn`: 認証がなさそうなら警告するが起動は続行
  - `require`: 認証がなさそうなら起動を停止
  - これは status check のみであり、自動ログインはしません
- `codex.login_mode` — `browser`、`device`、`api-key`、または `access-token`
- `codex.credentials_store` — `file`、`keyring`、または `auto`
- `codex.seed_from_host` — `if_missing`、`always`、または `never`
- `codex.persist_auth` — 現在は `stage` のみ
- `codex.login_args` — `codex login` に追加する引数
- `claude.login_mode` — `browser`、`console`、`email`、または `sso`
- `claude.login_args` — `claude auth login` に追加する引数
- `opencode.provider` / `opencode.method` — `opencode auth login` に渡すデフォルト値
- `opencode.login_args` — `opencode auth login` に追加する引数

### `env`（任意）

起動環境に渡す追加の環境変数。トップレベルとプロファイルごとの `env` はマージされます。

### `os`（任意）

組み込みのコンテナ OS テンプレート。有効な値:

- `debian12`
- `ubi9`
- `ubi10`
- `ubuntu2604`

`environment: container` の場合のみ有効。`dockerfile` と排他的です。

### `dockerfile`（任意）

カスタム Dockerfile のパス。git ルートからの相対パス（絶対パスも可）。`environment: container` の場合のみ有効。`os` と排他的です。

### `container_runtime`（任意）

使用するコンテナ CLI:

- `docker`
- `podman`

省略した場合、デフォルトは `docker` です。

### `mount_gh`（任意）

ホストの `~/.config/gh` を読み取り専用でコンテナにマウントするかどうか。`gh` コマンド（PR 作成、Issue 管理など）をコンテナ内で使う場合に有効化します。

**デフォルト: `false`（無効）**。gh トークンは AI エージェントから読み取り可能であり、プロンプトインジェクション攻撃により外部に流出するリスクがあります。有効にする場合は、スコープを絞ったトークンの使用を推奨します。

省略した場合、トップレベルのデフォルトから継承します。組み込みスターター設定では `false` です。

### `mount_ssh`（任意）

ホストの `~/.ssh` を読み取り専用でコンテナにマウントするかどうか。コンテナのエントリポイントが `/home/agent/.ssh` にコピーしてパーミッションを修正します。フル SSH アクセス（サーバーログイン、鍵ベースの認証など）を提供します。

省略した場合、トップレベルのデフォルトから継承します。組み込みスターター設定では `false` です。

### `ssh_agent_forwarding`（任意）

ホストの SSH Agent をコンテナに転送し、Git SSH 操作（push、clone、fetch）を有効にするかどうか。`mount_ssh` と異なり、SSH 鍵ファイルはコンテナにコピーされません — SSH Agent ソケット（`SSH_AUTH_SOCK`）のみが転送されます。

ホスト側で SSH Agent が起動し、鍵が登録されている必要があります（`ssh-add -l` で確認）。

`mount_ssh: true` も設定されている場合、`ssh_agent_forwarding` は無視されます（`mount_ssh` が既にフル SSH アクセスを提供するため）。

省略した場合、トップレベルのデフォルトから継承します。組み込みスターター設定では `false` です。

**プラットフォームごとの動作:**
- Linux (Docker/Podman): `$SSH_AUTH_SOCK` を直接マウント
- macOS + Docker Desktop: Docker Desktop のファイル共有経由で動作
- macOS + Podman: `aw` が自動的に Podman VM への SSH トンネルを確立し、初回使用時に最小限の SELinux ポリシーモジュール（`aw_agent_sock`）をインストール。詳細は [コンテナ同期ガイド](container-sync.md) を参照

### `mounts`（任意）

コンテナプロファイル用の追加バインドマウント。

各マウントでサポートされるフィールド:

- `source` — ホストパス（`~` 展開に対応）
- `target` — コンテナパス
- `mode` — `ro`（デフォルト）または `rw`
- `options` — Docker/Podman のマウントオプション（例: `"z"`, `"Z,nocopy"`, `"cached"`）

## 組み込みスターター設定

設定ファイルがない場合、`aw` は組み込みスターター設定が読み込まれたかのように動作します。スターター設定は現在以下を提供します:

- `claude`
- `shell`
- `codex`
- `opencode`
- `ubi9-shell`
- `ubi10-shell`
- `ubuntu2604-shell`

組み込みのデフォルトプロファイルは `claude` です。

現在のスターター設定を `~/.config/aw/config.yml` に書き出すには:

```bash
aw init
```

## バリデーションルール

`aw` は実行のたびに有効なプロファイルをバリデーションします。

現在のルール:

1. 少なくとも1つのプロファイルが存在すること
2. `default` が設定されている場合、既存のプロファイルを指していること
3. `environment` は必須で `host` または `container` であること
4. `launch` は必須で `shell`、`claude`、`codex`、`opencode`、`zellij` のいずれかであること
5. `zellij` は `launch: zellij` の場合のみ有効
6. `zellij.tool` は `claude`、`codex`、`opencode` のいずれかであること
7. `os` はサポートされている組み込みテンプレートのいずれかであること
8. `os` は `environment: container` の場合のみ有効
9. `dockerfile` は `environment: container` の場合のみ有効
10. `os` と `dockerfile` は排他的
11. `container_runtime` は `docker` または `podman` であること
12. `mounts` は `environment: container` の場合のみ有効
13. すべてのマウントに `source` と `target` の両方が必要
14. `ssh_agent_forwarding` は `environment: container` の場合のみ有効
15. `auth.on_launch.check` が設定されている場合、`none`、`warn`、`require` のいずれかであること
16. `auth.codex.login_mode` が設定されている場合、`browser`、`device`、`api-key`、`access-token` のいずれかであること
17. `auth.codex.credentials_store` が設定されている場合、`file`、`keyring`、`auto` のいずれかであること
18. `auth.codex.seed_from_host` が設定されている場合、`if_missing`、`always`、`never` のいずれかであること
19. `auth.codex.persist_auth` が設定されている場合、現在は `stage` であること
20. `auth.claude.login_mode` が設定されている場合、`browser`、`console`、`email`、`sso` のいずれかであること

## コンテナに同期されるホスト設定

`environment: container` を使用する場合、`aw` は以下のホスト設定を自動的に処理します:

- `~/.gitconfig` → `/home/agent/.gitconfig`
- `~/.config/gh` → `/home/agent/.config/gh`（`mount_gh: true` の場合のみ。デフォルト無効）
- `~/.claude/settings.json` → `/home/agent/.claude/settings.json`
- `~/.claude/CLAUDE.md` → `/home/agent/.claude/CLAUDE.md`
- `~/.claude/hooks` → `/home/agent/.claude/hooks`
- `~/.claude/plugins` → `/home/agent/.claude/plugins`
- `~/.claude/commands` → `/home/agent/.claude/commands`
- `~/.claude/agents` → `/home/agent/.claude/agents`

`~/.ssh` はデフォルトではマウントされません。フル SSH アクセスには `mount_ssh: true`、Git 操作のみなら `ssh_agent_forwarding: true` を設定してください。

## Tips

- `aw profiles` で利用可能なプロファイルとその読み込み元を確認できます
- `aw init` はカスタマイズしたい場合のみ実行してください
- プロファイル名は短く、わかりやすくしましょう
- チームで共有する場合は `.agent-workspace.yml` をコミットしてください
