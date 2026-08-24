# 設定リファレンス

`aw` が `.aw.yml` 形式の設定をどのように読み込み、マージし、バリデーションするかを説明します。

## 概要

`aw` は設定ファイルがなくても組み込みのスタータープロファイルで起動できます。設定ファイルがある場合は、組み込み設定とマージして、指定されたプロファイル名の有効なプロファイルを生成します。

## ファイルの場所

`aw` は以下の場所から設定を読み込みます:

1. バイナリに埋め込まれた組み込みスターター設定
2. `~/.config/aw/config.yml`
3. `<git ルート>/.aw.yml`

`git rev-parse --show-toplevel` が失敗した場合、カレントディレクトリの `.aw.yml` にフォールバックします。

`~/.config/aw/` にはプロファイル設定ファイルを配置します:

| ファイル | 用途 |
|---------|------|
| `config.yml` | プロファイル設定 |

`aw init` を実行すると、設定ファイルが `~/.config/aw/` に生成されます。パッケージやツールの定義はワークスペースの `packages.txt` や `mise.toml` で管理します。

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
- `.aw.yml`
- 上記いずれかのファイルのトップレベル共有デフォルト
- `profiles.<name>` のプロファイルごとのオーバーライド

### マージモデル

`aw` は設定を以下の手順で解決します:

1. 組み込みスターター設定を読み込む
2. `~/.config/aw/config.yml` があればオーバーレイ
3. `.aw.yml` があればオーバーレイ
4. 最終的なトップレベル共有デフォルトを全プロファイルに適用
5. 結果の有効なプロファイルをバリデーション

### フィールドごとのマージルール

- `env` はキーごとにマージ（後の値が優先）
- `worktree` はフィールドごとにマージ
- `packages` は指定時に全体を置換（ワークスペースの `packages.txt` とマージされる）
- `mounts` は指定時に全体を置換
- `mount_gh` は明示的な三値動作:
  - 省略: 継承
  - `true`: 有効化
  - `false`: 無効化
- `mount_ssh` は `mount_gh` と同じ三値動作
- `ssh_agent_forwarding` は `mount_gh` と同じ三値動作
- `mount_container_sock` は `mount_gh` と同じ三値動作
- `gh_token` は `mount_gh` と同じ三値動作
- `skip_devbox_install` は `mount_gh` と同じ三値動作
- `skip_mise_install` は `mount_gh` と同じ三値動作
- `os` と `dockerfile` は排他的。`image` と `dockerfile` は共存可能（`aw run` 時は `image` を使い、`aw build` 時は `dockerfile` でビルドする）

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
default: claude

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

  worktree-claude:
    worktree: {}
    launch: claude

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

  airgap:
    launch: claude
    image: 'aw-container:a1b2c3d4e5f6'
    skip_devbox_install: true
    skip_mise_install: true
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
- `mount_container_sock`
- `gh_token`
- `packages`
- `package_manager`
- `mounts`
- `os`
- `dockerfile`
- `container_user`
- `worktree`

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

### `reaper`（任意）

コンテナ終了後の後処理（reaper）の動作を制御します。`environment: container` の場合のみ有効です。

サポートされるフィールド:

- `timeout` — reaper のタスク実行タイムアウト（秒）。デフォルト `60`、最大 `3600`
- `keep-container` — `true` の場合、コンテナ終了後も削除せず保持（デバッグ用）。デフォルト `false`
- `report-retention` — 保持するレポート件数。デフォルト `10`、最大 `100`

```yaml
profiles:
  debug-claude:
    environment: container
    launch: claude
    reaper:
      timeout: 120
      keep-container: true
      report-retention: 20
```

### `auth`（任意）

プロファイルに対する `aw auth ...` の動作と、起動時の認証状態チェック（任意）を制御します。

通常は `aw auth login claude|codex|opencode|cursor` がメインのエントリポイントです。このコマンドはデフォルトの Debian コンテナを使うため、ホストにツールがインストールされていなくても動作します。

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

### `image`（任意）

事前にビルド済みのコンテナイメージ名。`environment: container` の場合のみ有効。

`aw run` 時はこのイメージを直接使用します。イメージがローカルに存在しない場合はエラーになります。

`aw build` 時の動作は他のフィールドとの組み合わせで変わります:

- `image` + `dockerfile`: `dockerfile` でビルド（`image` は無視）
- `image` のみ + ワークスペースファイル（mise.toml 等）: `image` をベースに snapshot で増分ビルド（mise install → docker commit）
- `image` + `--from-template` / `--no-cache`: `image` を無視してテンプレートからフルビルド

`aw build --apply` でビルド結果を `image` に書き戻せます。

### `dockerfile`（任意）

カスタム Dockerfile のパス。git ルートからの相対パス（絶対パスも可）。`environment: container` の場合のみ有効。`os` と排他的です。`image` と併用可能（`aw run` は `image` を使い、`aw build` は `dockerfile` でビルド）。

### `skip_devbox_install`（任意）

コンテナ起動時のプロジェクト devbox.json のインストールをスキップするかどうか。`environment: container` の場合のみ有効。

省略した場合、トップレベルのデフォルトから継承します。

### `skip_mise_install`（任意）

コンテナ起動時のプロジェクト mise.toml のインストール（および mise 未インストール時の curl フォールバック）をスキップするかどうか。`environment: container` の場合のみ有効。

省略した場合、トップレベルのデフォルトから継承します。

### `container_runtime`（任意）

使用するコンテナ CLI:

- `docker`
- `podman`

省略した場合、デフォルトは `docker` です。

### `container_user`（任意）

コンテナ内のユーザー名を指定します。ビルトイン OS テンプレート使用時は、このユーザー名で Dockerfile と entrypoint.sh がレンダリングされます。カスタム Dockerfile 使用時は、Dockerfile 内で作成したユーザー名と一致させてください。

**デフォルト: `"agent"`**

`container_user` を変更すると以下が連動して変わります:

- コンテナ内のホームディレクトリ（`/home/<container_user>`）
- マウント先のパス（`.gitconfig`、`.config/gh`、`.ssh-host`）
- ツール設定ディレクトリ（`.claude`、`.codex` など）
- スナップショットスクリプトの実行ユーザー

`environment: container` でのみ有効です。

### `package_manager`（任意）

コンテナ内の AI ツールのインストール方法を制御します。

- `apt`（デフォルト） — AI ツールをスタンドアロンの install script（curl ベース）でインストール。イメージサイズが軽量（約 400 MB）
- `devbox`（非推奨） — Nix single-user + devbox でインストール。イメージサイズが大きい（約 1.8 GB）

`environment: container` の場合のみ有効。

省略した場合、デフォルトは `apt` です。

### `packages`（任意）

コンテナイメージに追加でインストールする OS パッケージのリスト。`environment: container` の場合のみ有効。

```yaml
profiles:
  claude:
    environment: container
    launch: claude
    packages:
      - jq
      - tree
      - ripgrep
```

パッケージは 2 つの経路でインストールされます:

1. **ビルド時**（`os` テンプレート使用時）— `AW_EXTRA_PACKAGES` ビルド引数として Dockerfile に渡され、イメージレイヤーに組み込まれます。イメージハッシュにも含まれるため、パッケージ構成が変わるとイメージが再ビルドされます
2. **ランタイム**（カスタム `dockerfile` 使用時を含む全モード）— `AW_PACKAGES` 環境変数としてコンテナに渡され、`aw-init.sh` がコンテナ起動時にインストールします

パッケージ名は `[a-zA-Z0-9][a-zA-Z0-9.+_\-:]*` にマッチする必要があります。プロファイル YAML の不正な名前はバリデーションエラーになります。

#### ワークスペースパッケージファイル

ワークスペースルートに `packages.txt` を配置すると、プロファイルの `packages` とマージされます。`#` で始まる行はコメントとして無視され、不正な名前の行も無視されます。重複は自動的に除去されます。

```
# packages.txt（ワークスペースルート）
jq
tree
```

> **Note:** `~/.config/aw/packages.txt` によるグローバルパッケージ設定は廃止されました。ワークスペースの `packages.txt` またはプロファイルの `packages` フィールドを使用してください。

### `mount_gh`（任意）

ホストの `~/.config/gh` を読み取り専用でコンテナにマウントするかどうか。`gh` コマンド（PR 作成、Issue 管理など）をコンテナ内で使う場合に有効化します。

**デフォルト: `false`（無効）**。gh トークンは AI エージェントから読み取り可能であり、プロンプトインジェクション攻撃により外部に流出するリスクがあります。有効にする場合は、スコープを絞ったトークンの使用を推奨します。

省略した場合、トップレベルのデフォルトから継承します。組み込みスターター設定では `false` です。

### `gh_token`（任意）

ホストの `gh auth token` から GitHub トークンを取得し、コンテナに `GITHUB_TOKEN` 環境変数として渡します。コンテナ内で `gh` CLI や git HTTPS 操作（clone、push、fetch）が動作するようになります。

**デフォルト: `false`（無効）**。`mount_gh` と排他的です（両方を同時に指定するとバリデーションエラー）。

`environment: container` の場合のみ有効。

省略した場合、トップレベルのデフォルトから継承します。

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

### `mount_container_sock`（任意）

コンテナランタイム（Docker/Podman）のソケットをコンテナにマウントし、docker-compose 等によるコンテナ操作を有効にするかどうか（DooD: Docker outside of Docker 方式）。

コンテナ内に `DOCKER_HOST` と `CONTAINER_HOST`（podman-remote 用）が `unix:///run/container.sock` に自動設定されます。docker-compose / docker CLI はユーザーが mise.toml や devbox.json、カスタム Dockerfile で別途インストールしてください。

**デフォルト: `false`（無効）**。

省略した場合、トップレベルのデフォルトから継承します。

**プラットフォームごとの動作:**
- Linux + Docker: `/var/run/docker.sock` を直接マウント
- macOS + Docker Desktop: Docker Desktop のファイル共有経由で動作
- Linux + Podman: `podman info` でソケットパスを自動検出（rootless 対応）
- macOS + Podman: Podman VM 内の `/run/podman/podman.sock` をマウント

**⚠ セキュリティ:** AI エージェントがホスト（または Podman VM）のコンテナランタイムにフルアクセスできるようになります。有効化時に Warning ログが出力されます。

### `mounts`（任意）

コンテナプロファイル用の追加バインドマウント。

各マウントでサポートされるフィールド:

- `source` — ホストパス（`~` 展開に対応）
- `target` — コンテナパス
- `mode` — `ro`（デフォルト）または `rw`
- `options` — Docker/Podman のマウントオプション（例: `"z"`, `"Z,nocopy"`, `"cached"`）

### `build`（任意）

`aw build` コマンド用の設定。`environment: container` の場合のみ有効。

> **Note:** 旧 `export:` フィールドも後方互換のため読み込まれますが、`build:` の使用を推奨します。

サポートされるフィールド:

- `include` — ホストのディレクトリをイメージ内にコピーするリスト。各エントリは `src`（ホストパス）と `dst`（コンテナ内の絶対パス）を持つ
- `env` — イメージに焼き込む環境変数のマップ

```yaml
profiles:
  airgap:
    environment: container
    launch: claude
    build:
      include:
        - src: ./certs
          dst: /usr/local/share/ca-certificates
      env:
        HTTP_PROXY: http://proxy.corp:8080
```

CLI フラグ（`--include`, `--env`）でも指定でき、設定ファイルの値とマージされます。

### `build_env`（任意）

`docker build` / `podman build` に `--build-arg` として渡されるキー・バリューのマップ。ビルド時のみ有効で、イメージの `ENV` には焼き込まれません。`environment: container` の場合のみ有効。

```yaml
profiles:
  claude:
    environment: container
    launch: claude
    build_env:
      HTTP_PROXY: "http://proxy.corp:8080"
      HTTPS_PROXY: "http://proxy.corp:8080"
```

CLI フラグ `--build-arg KEY=VAL` でも指定でき、設定ファイルの値とマージされます（同一キーは CLI が優先）。

```bash
aw build claude --build-arg GITHUB_TOKEN=$GITHUB_TOKEN --from-template
```

> **Note:** `build_env` および `--build-arg` のキーに `AW_` プレフィックスは使用できません（内部ビルド引数と衝突するため）。

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
4. `launch` は必須で `shell`、`claude`、`codex`、`opencode` のいずれかであること
5. `os` はサポートされている組み込みテンプレートのいずれかであること
6. `os` は `environment: container` の場合のみ有効
7. `dockerfile` は `environment: container` の場合のみ有効
8. `image` は `environment: container` の場合のみ有効
9. `os` と `dockerfile` は排他的。`image` と `dockerfile` は共存可能
10. `container_runtime` は `docker` または `podman` であること
11. `package_manager` は `apt` または `devbox` であること
12. `package_manager` は `environment: container` の場合のみ有効
13. `skip_devbox_install` は `environment: container` の場合のみ有効
14. `skip_mise_install` は `environment: container` の場合のみ有効
15. `mounts` は `environment: container` の場合のみ有効
16. すべてのマウントに `source` と `target` の両方が必要
17. `container_user` は `environment: container` の場合のみ有効
18. `ssh_agent_forwarding` は `environment: container` の場合のみ有効
19. `gh_token` は `environment: container` の場合のみ有効
20. `mount_gh` と `gh_token` は排他的
21. `mount_container_sock` は `environment: container` の場合のみ有効
22. `auth.on_launch.check` が設定されている場合、`none`、`warn`、`require` のいずれかであること
23. `auth.codex.login_mode` が設定されている場合、`browser`、`device`、`api-key`、`access-token` のいずれかであること
24. `auth.codex.credentials_store` が設定されている場合、`file`、`keyring`、`auto` のいずれかであること
25. `auth.codex.seed_from_host` が設定されている場合、`if_missing`、`always`、`never` のいずれかであること
26. `auth.codex.persist_auth` が設定されている場合、現在は `stage` であること
27. `auth.claude.login_mode` が設定されている場合、`browser`、`console`、`email`、`sso` のいずれかであること
28. `reaper` は `environment: container` の場合のみ有効
29. `reaper.timeout` は 0〜3600 の範囲であること
30. `reaper.report-retention` は 0〜100 の範囲であること
31. `packages` は `environment: container` の場合のみ有効
32. `packages` の各パッケージ名は `[a-zA-Z0-9][a-zA-Z0-9.+_\-:]*` にマッチすること

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

## Kubernetes マニフェスト生成

`aw manifest` コマンドでプロファイルから K8s マニフェストを生成できます。ローカル Docker/Podman のフローには影響しません。

### kubernetes: セクション

プロファイルに `kubernetes:` を追加すると、`aw manifest` の生成内容をカスタマイズできます:

```yaml
profiles:
  k8s-claude:
    launch: claude
    environment: container
    gh_token: true
    kubernetes:
      mode: chat                    # "interactive" (default) / "chat"
      namespace: dev-agents         # default: "aw"
      registry: ghcr.io/myorg       # aw build --push 時のレジストリ
      resources:
        requests: { cpu: "1", memory: 2Gi }
        limits:   { cpu: "4", memory: 8Gi }
      node_selector:
        kubernetes.io/arch: amd64
      tolerations:
        - key: dedicated
          value: ai-agent
          effect: NoSchedule
      service_account: my-sa        # 既存 SA を使う場合（SA リソース生成をスキップ）
      image_pull_secrets: [ghcr-secret]
      pod_labels: { team: backend }
      pod_annotations: {}
      workspace_size: 10Gi          # 指定時のみ PVC 生成
      storage_class: gp3
      secrets:                      # 認証情報の注入
        env:
          - ANTHROPIC_API_KEY
          - CLAUDE_CODE_USE_VERTEX
        files:
          - source: ~/.config/gcloud/application_default_credentials.json
            mountPath: /home/agent/.config/gcloud/application_default_credentials.json
            env: GOOGLE_APPLICATION_CREDENTIALS
```

### 利用モード

| モード | 用途 | Pod の動作 |
|---|---|---|
| `interactive` | 開発者が `kubectl attach` で接続 | ツールコマンドを直接起動 (stdin/tty あり) |
| `chat` | Slack Bot が `kubectl exec` でプロンプト送信 | `sleep infinity` で待機、exec で実行 |

### secrets 設定

`kubernetes.secrets` を省略すると、ホスト環境から既知の認証情報 (Vertex AI, Bedrock, GCP ADC 等) を自動検出します。明示的に指定した場合は自動検出は無効になります。

- `secrets.env` — ホストの環境変数を K8s Secret (envFrom) に転送
- `secrets.files` — ホストのファイルを K8s Secret に埋め込み、Pod 内にマウント。`env` フィールドでマウントパスを環境変数として設定可能

### コマンド例

```bash
# マニフェスト生成 (stdout)
aw manifest k8s-claude --name alice

# ファイルに出力
aw manifest k8s-claude --name alice -o ./manifests/

# イメージビルド + push
aw build k8s-claude --push --registry ghcr.io/myorg

# クラスタに適用
aw manifest k8s-claude --name alice | kubectl apply -f -
```

## Tips

- `aw profiles` で利用可能なプロファイルとその読み込み元を確認できます
- `aw init` はカスタマイズしたい場合のみ実行してください
- プロファイル名は短く、わかりやすくしましょう
- チームで共有する場合は `.aw.yml` をコミットしてください
